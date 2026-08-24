package console

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const watchedConsoleHelper = "QRL_TEST_WATCHED_CONSOLE_HELPER"
const watchedConsoleHelperMode = "QRL_TEST_WATCHED_CONSOLE_HELPER_MODE"

func TestParseSuiteResult(t *testing.T) {
	for name, testCase := range map[string]struct {
		output  string
		wantErr bool
	}{
		"success": {
			output: "CONSOLE_E2E_PASS api",
		},
		"failure": {
			output:  "CONSOLE_E2E_FAIL api",
			wantErr: true,
		},
		"success then failure": {
			output:  "CONSOLE_E2E_PASS api\nCONSOLE_E2E_FAIL api unexpected callback",
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := parseSuiteResult("api", []byte(testCase.output))
			if testCase.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSuiteFixtures(t *testing.T) {
	names := []string{"harness"}
	for _, scenario := range consoleScenarios {
		names = append(names, scenario.name)
	}
	for _, name := range names {
		_, err := fs.Stat(consoleFixtures, "testdata/console/"+name+".js")
		require.NoErrorf(t, err, "%s", name)
	}
}

func TestEventsFixtureEmitsTerminalMarkersBeforeFilterTeardown(t *testing.T) {
	source, err := fs.ReadFile(consoleFixtures, "testdata/console/events.js")
	require.NoError(t, err)

	script := string(source)
	const teardown = "watcher.stopWatching();"
	require.Equal(t, 2, strings.Count(script, teardown))

	failureMarker := strings.Index(script, `console.error("CONSOLE_E2E_FAIL events " + failure);`)
	firstTeardown := strings.Index(script, teardown)
	require.True(t, failureMarker >= 0 && failureMarker < firstTeardown,
		"failure marker must be emitted before watcher teardown")

	successMarker := strings.LastIndex(script, "suite.finish();")
	lastTeardown := strings.LastIndex(script, teardown)
	require.True(t, successMarker >= 0 && successMarker < lastTeardown,
		"success marker must be emitted before watcher teardown")
}

type fakeConsoleEngine struct {
	containerID      string
	spec             consoleContainerSpec
	copyID           string
	copyPath         string
	startID          string
	startInteractive bool
	command          func(context.Context) *exec.Cmd
	createErr        error
	copyErr          error
	removeErr        error
	removed          bool
	removeContextErr error
}

func (engine *fakeConsoleEngine) copyFixtures(_ context.Context, containerID, jsPath string) error {
	engine.copyID = containerID
	engine.copyPath = jsPath
	return engine.copyErr
}

func (engine *fakeConsoleEngine) create(_ context.Context, spec consoleContainerSpec) (string, error) {
	engine.spec = spec
	if engine.createErr != nil {
		return "", engine.createErr
	}
	if engine.containerID == "" {
		engine.containerID = "console-container"
	}
	return engine.containerID, nil
}

func (engine *fakeConsoleEngine) start(ctx context.Context, containerID string, interactive bool) *exec.Cmd {
	engine.startID = containerID
	engine.startInteractive = interactive
	return engine.command(ctx)
}

func (engine *fakeConsoleEngine) remove(ctx context.Context, containerID string) error {
	engine.removed = true
	engine.removeContextErr = ctx.Err()
	if containerID != engine.containerID {
		return fmt.Errorf("remove unexpected container %q", containerID)
	}
	return engine.removeErr
}

func TestDockerConsoleEngineBuildsContainerCommands(t *testing.T) {
	jsPath := t.TempDir()
	var calls [][]string
	var startPath string
	var startArgs []string
	engine := dockerConsoleEngine{
		output: func(_ context.Context, name string, arguments ...string) ([]byte, error) {
			calls = append(calls, append([]string{name}, arguments...))
			if arguments[0] == "create" {
				return []byte("container-id\n"), nil
			}
			return nil, nil
		},
		command: func(ctx context.Context, path string, arguments ...string) *exec.Cmd {
			startPath = path
			startArgs = slices.Clone(arguments)
			return consoleHelperCommand(ctx, "early-exit")
		},
	}

	containerID, err := engine.create(t.Context(), consoleContainerSpec{
		image:       "registry.example/go-qrl@sha256:digest",
		endpoint:    "ws://127.0.0.1:8546",
		scenario:    "events",
		interactive: true,
	})
	require.NoError(t, err)
	require.Equal(t, "container-id", containerID)
	wantCreate := []string{
		"docker", "create", "--pull=never", "--interactive",
		"--add-host", "host.docker.internal=host-gateway",
		"--entrypoint", "gqrl",
		"registry.example/go-qrl@sha256:digest",
		"attach",
		"--datadir", "/tmp/qrl-tests-console",
		"--jspath", "/tmp/qrl-tests-js",
		"--preload", "harness.js,events.js",
		"ws://host.docker.internal:8546",
	}
	require.Len(t, calls, 1)
	require.Equal(t, wantCreate, calls[0])
	require.NoError(t, engine.copyFixtures(t.Context(), containerID, jsPath))
	wantCopy := []string{"docker", "cp", jsPath, "container-id:/tmp/qrl-tests-js"}
	require.Len(t, calls, 2)
	require.Equal(t, wantCopy, calls[1])
	_ = engine.start(t.Context(), containerID, true)
	require.Equal(t, "docker", startPath)
	require.Equal(t, []string{"start", "--attach", "--interactive", "container-id"}, startArgs)
	require.NoError(t, engine.remove(t.Context(), containerID))
	wantRemove := []string{"docker", "rm", "--force", "container-id"}
	require.Len(t, calls, 3)
	require.Equal(t, wantRemove, calls[2])
}

func TestDockerConsoleEngineBuildsExecContainerCommand(t *testing.T) {
	var call []string
	engine := dockerConsoleEngine{output: func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		call = append([]string{name}, arguments...)
		return []byte("container-id\n"), nil
	}}

	_, err := engine.create(t.Context(), consoleContainerSpec{
		image:    "registry.example/go-qrl@sha256:digest",
		endpoint: "http://127.0.0.1:8545",
		scenario: "api",
	})
	require.NoError(t, err)
	want := []string{
		"docker", "create", "--pull=never",
		"--add-host", "host.docker.internal=host-gateway",
		"--entrypoint", "gqrl",
		"registry.example/go-qrl@sha256:digest",
		"attach",
		"--datadir", "/tmp/qrl-tests-console",
		"--jspath", "/tmp/qrl-tests-js",
		"--exec", "loadScript('harness.js');loadScript('api.js')",
		"http://host.docker.internal:8545",
	}
	require.Equal(t, want, call)
}

func TestConsoleContainerEndpoint(t *testing.T) {
	for name, testCase := range map[string]struct {
		endpoint string
		want     string
		wantErr  bool
	}{
		"IPv4 loopback": {
			endpoint: "http://127.23.45.67:8545",
			want:     "http://host.docker.internal:8545",
		},
		"localhost WebSocket": {
			endpoint: "ws://localhost:8546",
			want:     "ws://host.docker.internal:8546",
		},
		"IPv6 loopback": {
			endpoint: "ws://[::1]:8546",
			want:     "ws://host.docker.internal:8546",
		},
		"non-loopback": {
			endpoint: "https://rpc.example:443",
			want:     "https://rpc.example:443",
		},
		"missing host": {
			endpoint: "http:///rpc",
			wantErr:  true,
		},
		"invalid URL": {
			endpoint: "http://%zz",
			wantErr:  true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := consoleContainerEndpoint(testCase.endpoint)
			if testCase.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestRunSuiteUsesExecutionImageContainer(t *testing.T) {
	jsPath := t.TempDir()
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		return consoleHelperCommand(ctx, "ordinary-success")
	}}
	err := runSuiteWithEngine(
		t.Context(),
		"registry.example/go-qrl@sha256:digest",
		jsPath,
		"http://127.0.0.1:8545",
		"api",
		engine,
	)
	require.NoError(t, err)
	require.Equal(t, consoleContainerSpec{
		image:    "registry.example/go-qrl@sha256:digest",
		endpoint: "http://127.0.0.1:8545",
		scenario: "api",
	}, engine.spec)
	require.True(t, engine.copyID == engine.containerID && engine.copyPath == jsPath &&
		engine.startID == engine.containerID && !engine.startInteractive && engine.removed && engine.removeContextErr == nil,
		"unexpected lifecycle state: %+v", engine)
}

func TestRunSuiteCleansUpAfterCreate(t *testing.T) {
	for name, testCase := range map[string]struct {
		mode        string
		removeErr   error
		wantDetails []string
	}{
		"script failure":  {mode: "ordinary-failure", wantDetails: []string{"emitted a failure marker"}},
		"process failure": {mode: "ordinary-nonzero", wantDetails: []string{"exit status"}},
		"cleanup failure": {mode: "ordinary-success", removeErr: errors.New("remove failed"), wantDetails: []string{"remove failed"}},
		"script and cleanup failure": {
			mode:        "ordinary-failure",
			removeErr:   errors.New("remove failed"),
			wantDetails: []string{"emitted a failure marker", "remove failed"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			engine := &fakeConsoleEngine{
				command: func(ctx context.Context) *exec.Cmd {
					return consoleHelperCommand(ctx, testCase.mode)
				},
				removeErr: testCase.removeErr,
			}
			err := runSuiteWithEngine(t.Context(), "image", t.TempDir(), "http://127.0.0.1:8545", "api", engine)
			require.Error(t, err)
			for _, detail := range testCase.wantDetails {
				require.ErrorContains(t, err, detail)
			}
			require.True(t, engine.removed)
		})
	}
}

func TestRunSuiteDoesNotCleanUpAfterCreateFailure(t *testing.T) {
	engine := &fakeConsoleEngine{createErr: errors.New("create failed")}
	err := runSuiteWithEngine(t.Context(), "image", t.TempDir(), "http://127.0.0.1:8545", "api", engine)
	require.ErrorContains(t, err, "create failed")
	require.False(t, engine.removed)
}

func TestRunSuiteCleansUpAfterFixtureCopyFailure(t *testing.T) {
	engine := &fakeConsoleEngine{copyErr: errors.New("copy failed")}
	err := runSuiteWithEngine(t.Context(), "image", t.TempDir(), "http://127.0.0.1:8545", "api", engine)
	require.ErrorContains(t, err, "copy console suite api fixtures")
	require.True(t, engine.removed && engine.startID == "", "unexpected lifecycle state: %+v", engine)
}

func TestRunSuiteTerminatesOnTimeoutAndUsesFreshCleanupContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		return consoleHelperCommand(ctx, "timeout")
	}}
	err := runSuiteWithEngine(ctx, "image", t.TempDir(), "http://127.0.0.1:8545", "api", engine)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.True(t, engine.removed && engine.removeContextErr == nil,
		"cleanup did not use a fresh context: %+v", engine)
}

func TestRunWatchedSuiteUsesExecutionImageContainer(t *testing.T) {
	var command *exec.Cmd
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		command = consoleHelperCommand(ctx, "success")
		return command
	}}
	err := runWatchedSuiteWithEngine(
		t.Context(),
		"registry.example/go-qrl@sha256:digest",
		t.TempDir(),
		"ws://127.0.0.1:8546",
		"events",
		engine,
	)
	require.NoError(t, err)
	require.True(t, engine.spec.interactive && engine.copyID == engine.containerID && engine.startID == engine.containerID &&
		engine.startInteractive && engine.removed, "unexpected lifecycle state: %+v", engine)
	require.NotNil(t, command.ProcessState)
	require.True(t, command.ProcessState.Success())
}

func TestRunWatchedSuiteCleansUpAfterStartFailure(t *testing.T) {
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing-docker"))
	}}
	err := runWatchedSuiteWithEngine(t.Context(), "image", t.TempDir(), "ws://127.0.0.1:8546", "events", engine)
	require.ErrorContains(t, err, "start console suite events")
	require.True(t, engine.removed)
}

func TestRunWatchedSuiteRejectsScriptFailure(t *testing.T) {
	var command *exec.Cmd
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		command = consoleHelperCommand(ctx, "failure")
		return command
	}}
	err := runWatchedSuiteWithEngine(t.Context(), "image", t.TempDir(), "ws://127.0.0.1:8546", "events", engine)
	require.ErrorContains(t, err, "emitted a failure marker")
	require.ErrorContains(t, err, "helper failure")
	require.NotNil(t, command.ProcessState)
	require.True(t, engine.removed)
}

func TestRunWatchedSuiteRejectsEarlyExit(t *testing.T) {
	var command *exec.Cmd
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		command = consoleHelperCommand(ctx, "early-exit")
		return command
	}}
	err := runWatchedSuiteWithEngine(t.Context(), "image", t.TempDir(), "ws://127.0.0.1:8546", "events", engine)
	require.ErrorContains(t, err, "emitted 0 success markers")
	require.NotNil(t, command.ProcessState)
	require.True(t, command.ProcessState.Success() && engine.removed)
}

func TestRunWatchedSuiteTerminatesOnTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	var command *exec.Cmd
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		command = consoleHelperCommand(ctx, "timeout")
		return command
	}}
	err := runWatchedSuiteWithEngine(ctx, "image", t.TempDir(), "ws://127.0.0.1:8546", "events", engine)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NotNil(t, command.ProcessState)
	require.True(t, engine.removed && engine.removeContextErr == nil)
}

func consoleHelperCommand(ctx context.Context, mode string) *exec.Cmd {
	command := exec.CommandContext(
		ctx,
		os.Args[0],
		"-test.run=^TestWatchedConsoleHelperProcess$",
	)
	command.Env = append(
		os.Environ(),
		watchedConsoleHelper+"=1",
		watchedConsoleHelperMode+"="+mode,
	)
	return command
}

func TestWatchedConsoleHelperProcess(t *testing.T) {
	if os.Getenv(watchedConsoleHelper) != "1" {
		return
	}

	switch os.Getenv(watchedConsoleHelperMode) {
	case "ordinary-success":
		fmt.Fprintln(os.Stdout, resultPrefix+"api")
	case "ordinary-failure":
		fmt.Fprintln(os.Stderr, failurePrefix+"api helper failure")
	case "ordinary-nonzero":
		fmt.Fprintln(os.Stderr, "helper process failed")
		os.Exit(2)
	case "success":
		fmt.Fprintln(os.Stdout, resultPrefix+"events")
		waitForConsoleExit(t)
	case "failure":
		fmt.Fprintln(os.Stdout, resultPrefix+"events")
		fmt.Fprintln(os.Stderr, failurePrefix+"events helper failure")
		waitForConsoleExit(t)
	case "early-exit":
		fmt.Fprintln(os.Stdout, "console exited before the script completed")
	case "timeout":
		for {
			time.Sleep(time.Second)
		}
	default:
		t.Fatalf("unknown helper mode %q", os.Getenv(watchedConsoleHelperMode))
	}
}

func waitForConsoleExit(t *testing.T) {
	t.Helper()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "exit" {
			return
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
