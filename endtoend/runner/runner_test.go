// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package runner

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"sync"
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
	logs, err := filepath.Glob(filepath.Join(reports, "multi", "output.log"))
	require.NoError(t, err)
	require.Len(t, logs, 1)
}

type concurrentNetworks struct {
	mutex       sync.Mutex
	active, max int
}

func (networks *concurrentNetworks) Start(_ context.Context, options devnet.StartOptions) (devnet.Environment, error) {
	networks.mutex.Lock()
	networks.active++
	if networks.active > networks.max {
		networks.max = networks.active
	}
	networks.mutex.Unlock()
	return testEnvironment(options.EnclaveName, options.Backend), nil
}

func (*concurrentNetworks) Inspect(_ context.Context, name string, backend devnet.Backend) (devnet.Environment, error) {
	return testEnvironment(name, backend), nil
}

func (networks *concurrentNetworks) Stop(context.Context, string) error {
	networks.mutex.Lock()
	networks.active--
	networks.mutex.Unlock()
	return nil
}

func TestRunBoundsParallelLanes(t *testing.T) {
	networks := new(concurrentNetworks)
	tests := New(Config{
		BaseName:     "qrl-tests",
		ReportDir:    t.TempDir(),
		Backend:      devnet.BackendKubernetes,
		StartTimeout: time.Minute,
		MaxParallel:  2,
	}, nil, nil)
	tests.networks = networks
	tests.runCommand = func(context.Context, commandSpec) error {
		time.Sleep(20 * time.Millisecond)
		return nil
	}

	var selected []lanes.Lane
	for _, name := range []string{"multi", "workloads", "lifecycle"} {
		lane, err := lanes.Named(name)
		require.NoError(t, err)
		selected = append(selected, lane)
	}
	require.NoError(t, tests.run(t.Context(), selected, provisionPerLane))
	require.Equal(t, 2, networks.max)
	require.Zero(t, networks.active)
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
	single, err := lanes.Named("single")
	require.NoError(t, err)
	require.Equal(t, []lanes.Tool{lanes.ToolGQRL, lanes.ToolClef}, requiredTools([]lanes.Lane{single, single}))
}

func TestRunPlanDescribesEachLane(t *testing.T) {
	reports := t.TempDir()
	single, err := lanes.Named("single")
	require.NoError(t, err)
	multi, err := lanes.Named("multi")
	require.NoError(t, err)
	selected := []lanes.Lane{single, multi}
	plan, err := newRunPlan(Config{BaseName: "qrl-tests", ReportDir: reports}, selected, provisionPerLane)
	require.NoError(t, err)
	require.Equal(t, []lanes.Tool{lanes.ToolGQRL, lanes.ToolClef}, plan.tools)
	require.Len(t, plan.lanes, 2)
	require.Equal(t, "qrl-tests-single", plan.lanes[0].enclaveName)
	require.Equal(t, filepath.Join(reports, "single", "environment.json"), plan.lanes[0].manifestPath)
	require.Contains(t, plan.lanes[0].arguments, "./endtoend/suites/execution/abi")
	require.True(t, plan.lanes[0].provision)
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
