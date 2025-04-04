// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build !windows

// Package bootstrap provides logic to self-bootstrap the installer.
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DataDog/datadog-agent/pkg/fleet/installer/paths"

	"github.com/DataDog/datadog-agent/pkg/fleet/installer/env"
	"github.com/DataDog/datadog-agent/pkg/fleet/installer/exec"
	"github.com/DataDog/datadog-agent/pkg/fleet/installer/oci"
)

func install(ctx context.Context, env *env.Env, url string, experiment bool) error {
	err := os.MkdirAll(paths.RootTmpDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	tmpDir, err := os.MkdirTemp(paths.RootTmpDir, "")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	cmd, err := downloadInstaller(ctx, env, url, tmpDir)
	if err != nil {
		return fmt.Errorf("failed to download installer: %w", err)
	}
	if experiment {
		return cmd.InstallExperiment(ctx, url)
	}
	return cmd.Install(ctx, url, nil)
}

// downloadInstaller downloads the installer package from the registry and returns an installer executor.
//
// This process is made to have the least assumption possible as it is long lived and should always work in the future.
// 1. Download the installer package from the registry.
// 2. Export the installer image as an OCI layout on the disk.
// 3. Extract the installer image layers on the disk.
// 4. Create an installer executor from the extract layer.
func downloadInstaller(ctx context.Context, env *env.Env, url string, tmpDir string) (*exec.InstallerExec, error) {
	// 1. Download the installer package from the registry.
	downloader := oci.NewDownloader(env, env.HTTPClient())
	downloadedPackage, err := downloader.Download(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to download installer package: %w", err)
	}
	if downloadedPackage.Name != InstallerPackage {
		return nil, fmt.Errorf("unexpected package name: %s, expected %s", downloadedPackage.Name, InstallerPackage)
	}

	// 2. Export the installer image as an OCI layout on the disk.
	layoutTmpDir, err := os.MkdirTemp(paths.RootTmpDir, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(layoutTmpDir)
	err = downloadedPackage.WriteOCILayout(layoutTmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to write OCI layout: %w", err)
	}

	// 3. Extract the installer image layers on the disk.
	err = downloadedPackage.ExtractLayers(oci.DatadogPackageLayerMediaType, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract layers: %w", err)
	}

	// 4. Create an installer executor from the extract layer.
	installerBinPath := filepath.Join(tmpDir, installerBinPath)
	return exec.NewInstallerExec(env, installerBinPath), nil
}

func getInstallerOCI(_ context.Context, env *env.Env) (string, error) {
	version := "latest"
	if env.DefaultPackagesVersionOverride[InstallerPackage] != "" {
		version = env.DefaultPackagesVersionOverride[InstallerPackage]
	}
	return oci.PackageURL(env, InstallerPackage, version), nil
}
