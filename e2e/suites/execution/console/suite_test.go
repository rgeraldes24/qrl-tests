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
		"failure then success": {
			output:  "CONSOLE_E2E_FAIL api unexpected callback\nCONSOLE_E2E_PASS api",
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := parseSuiteResult("api", []byte(testCase.output))
			if testCase.wantErr && err == nil {
				t.Fatal("failed suite was accepted")
			}
			if !testCase.wantErr && err != nil {
				t.Fatal(err)
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
		if _, err := fs.Stat(consoleFixtures, "testdata/console/"+name+".js"); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestEventsFixtureEmitsTerminalMarkersBeforeFilterTeardown(t *testing.T) {
	source, err := fs.ReadFile(consoleFixtures, "testdata/console/events.js")
	if err != nil {
		t.Fatal(err)
	}

	script := string(source)
	const teardown = "watcher.stopWatching();"
	if got := strings.Count(script, teardown); got != 2 {
		t.Fatalf("got %d watcher teardowns, want 2", got)
	}

	failureMarker := strings.Index(script, `console.error("CONSOLE_E2E_FAIL events " + failure);`)
	firstTeardown := strings.Index(script, teardown)
	if failureMarker < 0 || failureMarker > firstTeardown {
		t.Fatal("failure marker must be emitted before watcher teardown")
	}

	successMarker := strings.LastIndex(script, "suite.finish();")
	lastTeardown := strings.LastIndex(script, teardown)
	if successMarker < 0 || successMarker > lastTeardown {
		t.Fatal("success marker must be emitted before watcher teardown")
	}
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
	copied           bool
	removed          bool
	removeContextErr error
}

func (engine *fakeConsoleEngine) copyFixtures(_ context.Context, containerID, jsPath string) error {
	engine.copied = true
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
	if err != nil {
		t.Fatal(err)
	}
	if containerID != "container-id" {
		t.Fatalf("got container ID %q", containerID)
	}
	wantCreate := []string{
		"docker", "create", "--pull=missing", "--interactive",
		"--network", "host",
		"registry.example/go-qrl@sha256:digest",
		"attach",
		"--datadir", "/tmp/qrl-tests-console",
		"--jspath", "/tmp/qrl-tests-js",
		"--preload", "harness.js,events.js",
		"ws://127.0.0.1:8546",
	}
	if len(calls) != 1 || !slices.Equal(calls[0], wantCreate) {
		t.Fatalf("got create call %q, want %q", calls, wantCreate)
	}
	if err := engine.copyFixtures(t.Context(), containerID, jsPath); err != nil {
		t.Fatal(err)
	}
	wantCopy := []string{"docker", "cp", jsPath, "container-id:/tmp/qrl-tests-js"}
	if len(calls) != 2 || !slices.Equal(calls[1], wantCopy) {
		t.Fatalf("got copy call %q, want %q", calls, wantCopy)
	}
	_ = engine.start(t.Context(), containerID, true)
	if startPath != "docker" || !slices.Equal(startArgs, []string{"start", "--attach", "--interactive", "container-id"}) {
		t.Fatalf("got start command %q %q", startPath, startArgs)
	}
	if err := engine.remove(t.Context(), containerID); err != nil {
		t.Fatal(err)
	}
	wantRemove := []string{"docker", "rm", "--force", "container-id"}
	if len(calls) != 3 || !slices.Equal(calls[2], wantRemove) {
		t.Fatalf("got remove call %q, want %q", calls, wantRemove)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"docker", "create", "--pull=missing",
		"--network", "host",
		"registry.example/go-qrl@sha256:digest",
		"attach",
		"--datadir", "/tmp/qrl-tests-console",
		"--jspath", "/tmp/qrl-tests-js",
		"--exec", "loadScript('harness.js');loadScript('api.js')",
		"http://127.0.0.1:8545",
	}
	if !slices.Equal(call, want) {
		t.Fatalf("got create call %q, want %q", call, want)
	}
}

func TestRunSuiteUsesExecutionImageContainer(t *testing.T) {
	jsPath := t.TempDir()
	var command *exec.Cmd
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		command = consoleHelperCommand(ctx, "ordinary-success")
		return command
	}}
	err := runSuiteWithEngine(
		t.Context(),
		"registry.example/go-qrl@sha256:digest",
		jsPath,
		"http://127.0.0.1:8545",
		"api",
		engine,
	)
	if err != nil {
		t.Fatal(err)
	}
	if engine.spec.image != "registry.example/go-qrl@sha256:digest" ||
		engine.spec.endpoint != "http://127.0.0.1:8545" ||
		engine.spec.scenario != "api" || engine.spec.interactive {
		t.Fatalf("unexpected container spec: %+v", engine.spec)
	}
	if !engine.copied || engine.copyID != engine.containerID || engine.copyPath != jsPath ||
		engine.startID != engine.containerID || engine.startInteractive || !engine.removed || engine.removeContextErr != nil {
		t.Fatalf("unexpected lifecycle state: %+v", engine)
	}
	if command.ProcessState == nil || !command.ProcessState.Success() {
		t.Fatalf("console process did not exit cleanly: %v", command.ProcessState)
	}
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
			if err == nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, detail := range testCase.wantDetails {
				if !strings.Contains(err.Error(), detail) {
					t.Fatalf("error %q does not contain %q", err, detail)
				}
			}
			if !engine.removed {
				t.Fatal("container was not removed")
			}
		})
	}
}

func TestRunSuiteDoesNotCleanUpAfterCreateFailure(t *testing.T) {
	engine := &fakeConsoleEngine{createErr: errors.New("create failed")}
	err := runSuiteWithEngine(t.Context(), "image", t.TempDir(), "http://127.0.0.1:8545", "api", engine)
	if err == nil || !strings.Contains(err.Error(), "create failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.removed {
		t.Fatal("nonexistent container was removed")
	}
}

func TestRunSuiteCleansUpAfterFixtureCopyFailure(t *testing.T) {
	engine := &fakeConsoleEngine{copyErr: errors.New("copy failed")}
	err := runSuiteWithEngine(t.Context(), "image", t.TempDir(), "http://127.0.0.1:8545", "api", engine)
	if err == nil || !strings.Contains(err.Error(), "copy console suite api fixtures") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !engine.removed || engine.startID != "" {
		t.Fatalf("unexpected lifecycle state: %+v", engine)
	}
}

func TestRunSuiteTerminatesOnTimeoutAndUsesFreshCleanupContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		return consoleHelperCommand(ctx, "timeout")
	}}
	err := runSuiteWithEngine(ctx, "image", t.TempDir(), "http://127.0.0.1:8545", "api", engine)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want context deadline", err)
	}
	if !engine.removed || engine.removeContextErr != nil {
		t.Fatalf("cleanup did not use a fresh context: %+v", engine)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	if !engine.spec.interactive || !engine.copied || engine.startID != engine.containerID || !engine.startInteractive || !engine.removed {
		t.Fatalf("unexpected lifecycle state: %+v", engine)
	}
	if command.ProcessState == nil || !command.ProcessState.Success() {
		t.Fatalf("console process did not exit cleanly: %v", command.ProcessState)
	}
}

func TestRunWatchedSuiteCleansUpAfterStartFailure(t *testing.T) {
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing-docker"))
	}}
	err := runWatchedSuiteWithEngine(t.Context(), "image", t.TempDir(), "ws://127.0.0.1:8546", "events", engine)
	if err == nil || !strings.Contains(err.Error(), "start console suite events") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !engine.removed {
		t.Fatal("container was not removed")
	}
}

func TestRunWatchedSuiteRejectsScriptFailure(t *testing.T) {
	var command *exec.Cmd
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		command = consoleHelperCommand(ctx, "failure")
		return command
	}}
	err := runWatchedSuiteWithEngine(t.Context(), "image", t.TempDir(), "ws://127.0.0.1:8546", "events", engine)
	if err == nil || !strings.Contains(err.Error(), "emitted a failure marker") ||
		!strings.Contains(err.Error(), "helper failure") {
		t.Fatalf("unexpected error: %v", err)
	}
	if command.ProcessState == nil || !engine.removed {
		t.Fatal("failed console process was not reaped and removed")
	}
}

func TestRunWatchedSuiteRejectsEarlyExit(t *testing.T) {
	var command *exec.Cmd
	engine := &fakeConsoleEngine{command: func(ctx context.Context) *exec.Cmd {
		command = consoleHelperCommand(ctx, "early-exit")
		return command
	}}
	err := runWatchedSuiteWithEngine(t.Context(), "image", t.TempDir(), "ws://127.0.0.1:8546", "events", engine)
	if err == nil || !strings.Contains(err.Error(), "emitted 0 success markers") {
		t.Fatalf("unexpected early-exit result: %v", err)
	}
	if command.ProcessState == nil || !command.ProcessState.Success() || !engine.removed {
		t.Fatalf("early-exit process was not reaped and removed: %v", command.ProcessState)
	}
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
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want context deadline", err)
	}
	if command.ProcessState == nil || !engine.removed || engine.removeContextErr != nil {
		t.Fatal("timed-out console process was not reaped and removed")
	}
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
