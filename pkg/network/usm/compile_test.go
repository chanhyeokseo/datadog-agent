// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build linux_bpf

package usm

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/DataDog/datadog-agent/pkg/ebpf/ebpftest"
	usmconfig "github.com/DataDog/datadog-agent/pkg/network/usm/config"
	"github.com/DataDog/datadog-agent/pkg/network/usm/utils"
	"github.com/DataDog/datadog-agent/pkg/util/kernel"
)

func TestHttpCompile(t *testing.T) {
	ebpftest.TestBuildMode(t, ebpftest.RuntimeCompiled, "", func(t *testing.T) {
		currKernelVersion, err := kernel.HostVersion()
		require.NoError(t, err)
		if currKernelVersion < usmconfig.MinimumKernelVersion {
			t.Skip("USM Runtime compilation not supported on this kernel version")
		}
		cfg := utils.NewUSMEmptyConfig()
		cfg.BPFDebug = true
		out, err := getRuntimeCompiledUSM(cfg)
		require.NoError(t, err)
		_ = out.Close()
	})
}
