// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package runner

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/lanes"
	"github.com/cyyber/qrl-tests/endtoend/internal/runenv"
	"github.com/stretchr/testify/require"
)

type recordingNetworks struct {
	started devnet.StartOptions
	stopped string
	stopErr error
}

func (networks *recordingNetworks) Start(_ context.Context, options devnet.StartOptions) (devnet.Environment, error) {
	networks.started = options
	return testEnvironment(options.EnclaveName, options.Backend), nil
}

func (networks *recordingNetworks) Inspect(_ context.Context, name string, backend devnet.Backend) (devnet.Environment, error) {
	return testEnvironment(name, backend), nil
}

func (networks *recordingNetworks) Stop(_ context.Context, name string) error {
	networks.stopped = name
	return networks.stopErr
}

func TestRunBuildsCommandAndCleansUp(t *testing.T) {
	reports := t.TempDir()
	networks := new(recordingNetworks)
	var command commandSpec
	var output bytes.Buffer
	tests := New(Config{
		BaseName:     "qrl-tests",
		ReportDir:    reports,
		Backend:      devnet.BackendDocker,
		StartTimeout: time.Minute,
	}, &output, &output)
	tests.networks = networks
	tests.runCommand = func(_ context.Context, specification commandSpec) error {
		command = specification
		return nil
	}

	require.NoError(t, tests.Run(t.Context(), "multi"))
	require.Equal(t, "qrl-tests", networks.started.EnclaveName)
	require.Equal(t, devnet.ProfileMulti, networks.started.Profile)
	require.Equal(t, "qrl-tests", networks.stopped)
	require.Equal(t, "go", command.Path)
	require.Contains(t, command.Args, "./endtoend/suites/crosslayer/network")

	manifestPath := filepath.Join(reports, "multi", "environment.json")
	manifest, err := runenv.Read(manifestPath)
	require.NoError(t, err)
	require.Equal(t, "multi", manifest.Lane)
	require.Equal(t, devnet.ProfileMulti, manifest.Profile)
	require.Contains(t, command.Env, runenv.PathEnv+"="+manifestPath)
}

func TestRunReturnsCleanupFailure(t *testing.T) {
	networks := &recordingNetworks{stopErr: errors.New("stop failed")}
	tests := New(Config{
		BaseName:     "qrl-tests",
		ReportDir:    t.TempDir(),
		Backend:      devnet.BackendDocker,
		StartTimeout: time.Minute,
	}, nil, nil)
	tests.networks = networks
	tests.runCommand = func(context.Context, commandSpec) error { return nil }

	err := tests.Run(t.Context(), "multi")
	require.ErrorContains(t, err, "stop failed")
}

func TestRequiredToolsAreUnique(t *testing.T) {
	require.Equal(t, []lanes.Tool{lanes.ToolGQRL, lanes.ToolClef}, requiredTools([]lanes.Lane{
		{Tools: []lanes.Tool{lanes.ToolGQRL, lanes.ToolClef}},
		{Tools: []lanes.Tool{lanes.ToolGQRL}},
	}))
}

func testEnvironment(name string, backend devnet.Backend) devnet.Environment {
	return devnet.Environment{
		EnclaveName: name,
		Backend:     backend,
		Participants: []devnet.Participant{{
			Index: 1,
			Execution: devnet.ExecutionService{
				RPCURL: "http://127.0.0.1:8545",
			},
			Consensus: devnet.ConsensusService{URL: "http://127.0.0.1:3500"},
		}},
	}
}
