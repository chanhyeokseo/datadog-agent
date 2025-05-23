// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build docker

package tailerfactory

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	compConfig "github.com/DataDog/datadog-agent/comp/core/config"
	log "github.com/DataDog/datadog-agent/comp/core/log/def"
	logmock "github.com/DataDog/datadog-agent/comp/core/log/mock"
	workloadmeta "github.com/DataDog/datadog-agent/comp/core/workloadmeta/def"
	workloadmetafxmock "github.com/DataDog/datadog-agent/comp/core/workloadmeta/fx-mock"
	workloadmetamock "github.com/DataDog/datadog-agent/comp/core/workloadmeta/mock"
	"github.com/DataDog/datadog-agent/comp/logs/agent/config"
	configmock "github.com/DataDog/datadog-agent/pkg/config/mock"
	"github.com/DataDog/datadog-agent/pkg/logs/internal/util/containersorpods"
	"github.com/DataDog/datadog-agent/pkg/logs/pipeline"
	"github.com/DataDog/datadog-agent/pkg/logs/sources"
	dockerutilPkg "github.com/DataDog/datadog-agent/pkg/util/docker"
	"github.com/DataDog/datadog-agent/pkg/util/fxutil"
	"github.com/DataDog/datadog-agent/pkg/util/option"
	"github.com/DataDog/datadog-agent/pkg/util/pointer"
)

var platformDockerLogsBasePath string

func fileTestSetup(t *testing.T) {
	dockerutilPkg.EnableTestingMode()
	tmp := t.TempDir()
	var oldPodLogsBasePath, oldDockerLogsBasePathNix, oldDockerLogsBasePathWin, oldPodmanLogsBasePath string
	oldPodLogsBasePath, podLogsBasePath = podLogsBasePath, filepath.Join(tmp, "pods")
	oldDockerLogsBasePathNix, dockerLogsBasePathNix = dockerLogsBasePathNix, filepath.Join(tmp, "docker-nix")
	oldDockerLogsBasePathWin, dockerLogsBasePathWin = dockerLogsBasePathWin, filepath.Join(tmp, "docker-win")
	oldPodmanLogsBasePath, podmanRootfullLogsBasePath = podmanRootfullLogsBasePath, filepath.Join(tmp, "containers")

	switch runtime.GOOS {
	case "windows":
		platformDockerLogsBasePath = dockerLogsBasePathWin
	default: // linux, darwin
		platformDockerLogsBasePath = dockerLogsBasePathNix
	}

	t.Cleanup(func() {
		podLogsBasePath = oldPodLogsBasePath
		dockerLogsBasePathNix = oldDockerLogsBasePathNix
		dockerLogsBasePathWin = oldDockerLogsBasePathWin
		podmanRootfullLogsBasePath = oldPodmanLogsBasePath
	})
}

func makeTestPod() (*workloadmeta.KubernetesPod, *workloadmeta.Container) {
	podID := "poduuid"
	containerID := "abc"
	pod := &workloadmeta.KubernetesPod{
		EntityID: workloadmeta.EntityID{
			ID:   podID,
			Kind: workloadmeta.KindKubernetesPod,
		},
		EntityMeta: workloadmeta.EntityMeta{
			Name:      "podname",
			Namespace: "podns",
		},
		Containers: []workloadmeta.OrchestratorContainer{
			{
				ID:   containerID,
				Name: "cname",
				Image: workloadmeta.ContainerImage{
					Name: "iname",
				},
			},
		},
	}

	container := &workloadmeta.Container{
		EntityID: workloadmeta.EntityID{
			Kind: workloadmeta.KindContainer,
			ID:   containerID,
		},
		Owner: &workloadmeta.EntityID{
			Kind: workloadmeta.KindKubernetesPod,
			ID:   podID,
		},
	}

	return pod, container
}

func TestMakeFileSource_docker_success(t *testing.T) {
	fileTestSetup(t)

	p := filepath.Join(platformDockerLogsBasePath, filepath.FromSlash("containers/abc/abc-json.log"))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o777))
	require.NoError(t, os.WriteFile(p, []byte("{}"), 0o666))

	tf := &factory{
		pipelineProvider: pipeline.NewMockProvider(),
		cop:              containersorpods.NewDecidedChooser(containersorpods.LogContainers),
	}
	source := sources.NewLogSource("test", &config.LogsConfig{
		Type:                        "docker",
		Identifier:                  "abc",
		Source:                      "src",
		Service:                     "svc",
		Tags:                        []string{"tag!"},
		AutoMultiLine:               pointer.Ptr(true),
		AutoMultiLineSampleSize:     123,
		AutoMultiLineMatchThreshold: 0.123,
	})
	child, err := tf.makeFileSource(source)
	require.NoError(t, err)
	require.Equal(t, source.Name, child.Name)
	require.Equal(t, "file", child.Config.Type)
	require.Equal(t, source.Config.Identifier, child.Config.Identifier)
	require.Equal(t, p, child.Config.Path)
	require.Equal(t, source.Config.Source, child.Config.Source)
	require.Equal(t, source.Config.Service, child.Config.Service)
	require.Equal(t, source.Config.Tags, child.Config.Tags)
	require.Equal(t, sources.DockerSourceType, child.GetSourceType())
	require.Equal(t, *source.Config.AutoMultiLine, true)
	require.Equal(t, source.Config.AutoMultiLineSampleSize, 123)
	require.Equal(t, source.Config.AutoMultiLineMatchThreshold, 0.123)
}

func TestMakeFileSource_podman_success(t *testing.T) {
	fileTestSetup(t)
	mockConfig := configmock.New(t)
	mockConfig.SetWithoutSource("logs_config.use_podman_logs", true)

	// On Windows, podman runs within a Linux virtual machine, so the Agent would believe it runs in a Linux environment with all the paths being nix-like.
	// The real path on the system is abstracted by the Windows Subsystem for Linux layer, so this unit test is skipped.
	// Ref: https://github.com/containers/podman/blob/main/docs/tutorials/podman-for-windows.md
	if runtime.GOOS == "windows" {
		t.Skip("Skip on Windows due to WSL file path abstraction")
	}

	p := filepath.Join(podmanRootfullLogsBasePath, filepath.FromSlash("storage/overlay-containers/abc/userdata/ctr.log"))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o777))
	require.NoError(t, os.WriteFile(p, []byte("{}"), 0o666))

	tf := &factory{
		pipelineProvider: pipeline.NewMockProvider(),
		cop:              containersorpods.NewDecidedChooser(containersorpods.LogContainers),
	}
	source := sources.NewLogSource("test", &config.LogsConfig{
		Type:                        "podman",
		Identifier:                  "abc",
		Source:                      "src",
		Service:                     "svc",
		Tags:                        []string{"tag!"},
		AutoMultiLine:               pointer.Ptr(true),
		AutoMultiLineSampleSize:     321,
		AutoMultiLineMatchThreshold: 0.321,
	})
	child, err := tf.makeFileSource(source)
	require.NoError(t, err)
	require.Equal(t, source.Name, child.Name)
	require.Equal(t, "file", child.Config.Type)
	require.Equal(t, source.Config.Identifier, child.Config.Identifier)
	require.Equal(t, p, child.Config.Path)
	require.Equal(t, source.Config.Source, child.Config.Source)
	require.Equal(t, source.Config.Service, child.Config.Service)
	require.Equal(t, source.Config.Tags, child.Config.Tags)
	require.Equal(t, sources.DockerSourceType, child.GetSourceType())
	require.Equal(t, *source.Config.AutoMultiLine, true)
	require.Equal(t, source.Config.AutoMultiLineSampleSize, 321)
	require.Equal(t, source.Config.AutoMultiLineMatchThreshold, 0.321)
}

func TestMakeFileSource_podman_with_db_path_success(t *testing.T) {
	tmp := t.TempDir()
	customPath := filepath.Join(tmp, "/custom/path/containers/storage/db.sql")
	mockConfig := configmock.New(t)
	mockConfig.SetWithoutSource("logs_config.use_podman_logs", true)
	mockConfig.SetWithoutSource("podman_db_path", customPath)

	// On Windows, podman runs within a Linux virtual machine, so the Agent would believe it runs in a Linux environment with all the paths being nix-like.
	// The real path on the system is abstracted by the Windows Subsystem for Linux layer, so this unit test is skipped.
	// Ref: https://github.com/containers/podman/blob/main/docs/tutorials/podman-for-windows.md
	if runtime.GOOS == "windows" {
		t.Skip("Skip on Windows due to WSL file path abstraction")
	}

	p := filepath.Join(filepath.Join(tmp, "/custom/path/containers"), filepath.FromSlash("storage/overlay-containers/abc/userdata/ctr.log"))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o777))
	require.NoError(t, os.WriteFile(p, []byte("{}"), 0o666))

	tf := &factory{
		pipelineProvider: pipeline.NewMockProvider(),
		cop:              containersorpods.NewDecidedChooser(containersorpods.LogContainers),
	}
	source := sources.NewLogSource("test", &config.LogsConfig{
		Type:       "podman",
		Identifier: "abc",
		Source:     "src",
		Service:    "svc",
	})
	child, err := tf.makeFileSource(source)
	require.NoError(t, err)
	require.Equal(t, source.Name, child.Name)
	require.Equal(t, "file", child.Config.Type)
	require.Equal(t, source.Config.Identifier, child.Config.Identifier)
	require.Equal(t, p, child.Config.Path)
	require.Equal(t, source.Config.Source, child.Config.Source)
	require.Equal(t, source.Config.Service, child.Config.Service)
	require.Equal(t, sources.DockerSourceType, child.GetSourceType())
}

func TestMakeFileSource_docker_no_file(t *testing.T) {
	fileTestSetup(t)

	p := filepath.Join(platformDockerLogsBasePath, filepath.FromSlash("containers/abc/abc-json.log"))

	tf := &factory{
		pipelineProvider: pipeline.NewMockProvider(),
		cop:              containersorpods.NewDecidedChooser(containersorpods.LogContainers),
	}
	source := sources.NewLogSource("test", &config.LogsConfig{
		Type:       "docker",
		Identifier: "abc",
		Source:     "src",
		Service:    "svc",
	})
	child, err := tf.makeFileSource(source)
	require.Nil(t, child)
	require.Error(t, err)
	switch runtime.GOOS {
	case "windows":
		require.Contains(t, err.Error(), "The system cannot find the path specified")
	default: // linux, darwin
		require.Contains(t, err.Error(), p) // error is about the path
	}
}

func TestDockerOverride(t *testing.T) {
	tmp := t.TempDir()
	mockConfig := configmock.New(t)
	customPath := filepath.Join(tmp, "/custom/path")
	mockConfig.SetWithoutSource("logs_config.docker_path_override", customPath)

	p := filepath.Join(mockConfig.GetString("logs_config.docker_path_override"), filepath.FromSlash("containers/abc/abc-json.log"))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o777))
	require.NoError(t, os.WriteFile(p, []byte("{}"), 0o666))

	tf := &factory{
		pipelineProvider: pipeline.NewMockProvider(),
		cop:              containersorpods.NewDecidedChooser(containersorpods.LogContainers),
	}
	source := sources.NewLogSource("test", &config.LogsConfig{
		Type:       "docker",
		Identifier: "abc",
		Source:     "src",
		Service:    "svc",
	})

	tf.findDockerLogPath(source.Config.Identifier)

	child, err := tf.makeFileSource(source)

	require.NoError(t, err)
	require.Equal(t, "file", child.Config.Type)
	require.Equal(t, p, child.Config.Path)
}

func TestMakeK8sSource(t *testing.T) {
	fileTestSetup(t)

	dir := filepath.Join(podLogsBasePath, filepath.FromSlash("podns_podname_poduuid/cname"))
	require.NoError(t, os.MkdirAll(dir, 0o777))
	filename := filepath.Join(dir, "somefile.log")
	require.NoError(t, os.WriteFile(filename, []byte("{}"), 0o666))
	wildcard := filepath.Join(dir, "*.log")

	store := fxutil.Test[workloadmetamock.Mock](t, fx.Options(
		fx.Provide(func() log.Component { return logmock.New(t) }),
		compConfig.MockModule(),
		fx.Supply(context.Background()),
		workloadmetafxmock.MockModule(workloadmeta.NewParams()),
	))
	pod, container := makeTestPod()
	store.Set(pod)
	store.Set(container)

	tf := &factory{
		pipelineProvider:  pipeline.NewMockProvider(),
		cop:               containersorpods.NewDecidedChooser(containersorpods.LogPods),
		workloadmetaStore: option.New[workloadmeta.Component](store),
	}
	for _, sourceConfigType := range []string{"docker", "containerd"} {
		t.Run("source.Config.Type="+sourceConfigType, func(t *testing.T) {
			source := sources.NewLogSource("test", &config.LogsConfig{
				Type:                        sourceConfigType,
				Identifier:                  "abc",
				Source:                      "src",
				Service:                     "svc",
				Tags:                        []string{"tag!"},
				AutoMultiLine:               pointer.Ptr(true),
				AutoMultiLineSampleSize:     123,
				AutoMultiLineMatchThreshold: 0.123,
			})
			child, err := tf.makeK8sFileSource(source)
			require.NoError(t, err)
			require.Equal(t, "podns/podname/cname", child.Name)
			require.Equal(t, "file", child.Config.Type)
			require.Equal(t, "abc", child.Config.Identifier)
			require.Equal(t, wildcard, child.Config.Path)
			require.Equal(t, "src", child.Config.Source)
			require.Equal(t, "svc", child.Config.Service)
			require.Equal(t, []string{"tag!"}, []string(child.Config.Tags))
			require.Equal(t, *child.Config.AutoMultiLine, true)
			require.Equal(t, child.Config.AutoMultiLineSampleSize, 123)
			require.Equal(t, child.Config.AutoMultiLineMatchThreshold, 0.123)
			switch sourceConfigType {
			case "docker":
				require.Equal(t, sources.DockerSourceType, child.GetSourceType())
			case "containerd":
				require.Equal(t, sources.KubernetesSourceType, child.GetSourceType())
			}
		})
	}
}

func TestMakeK8sSource_pod_not_found(t *testing.T) {
	fileTestSetup(t)

	p := filepath.Join(platformDockerLogsBasePath, "containers/abc/abc-json.log")
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o777))
	require.NoError(t, os.WriteFile(p, []byte("{}"), 0o666))

	workloadmetaStore := fxutil.Test[option.Option[workloadmeta.Component]](t, fx.Options(
		fx.Provide(func() log.Component { return logmock.New(t) }),
		compConfig.MockModule(),
		fx.Supply(context.Background()),
		workloadmetafxmock.MockModule(workloadmeta.NewParams()),
	))

	tf := &factory{
		pipelineProvider:  pipeline.NewMockProvider(),
		cop:               containersorpods.NewDecidedChooser(containersorpods.LogPods),
		workloadmetaStore: workloadmetaStore,
	}
	source := sources.NewLogSource("test", &config.LogsConfig{
		Type:       "docker",
		Identifier: "abc",
	})
	child, err := tf.makeK8sFileSource(source)
	require.Nil(t, child)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot find pod for container")
}

func TestFindK8sLogPath(t *testing.T) {
	fileTestSetup(t)

	tests := []struct{ name, pathExists, expectedPattern string }{
		{"..v1.9", "poduuid/cname_1.log", "poduuid/cname_*.log"},
		{"v1.10..v1.13", "poduuid/cname/1.log", "poduuid/cname/*.log"},
		{"v1.14..", "podns_podname_poduuid/cname/1.log", "podns_podname_poduuid/cname/*.log"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pathExists := filepath.FromSlash(test.pathExists)
			expectedPattern := filepath.FromSlash(test.expectedPattern)
			p := filepath.Join(podLogsBasePath, pathExists)
			require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o777))
			require.NoError(t, os.WriteFile(p, []byte("xx"), 0o666))
			defer func() {
				require.NoError(t, os.RemoveAll(podLogsBasePath))
			}()
			pod, _ := makeTestPod()
			gotPattern := findK8sLogPath(pod, "cname")
			require.Equal(t, filepath.Join(podLogsBasePath, expectedPattern), gotPattern)
		})
	}
}
