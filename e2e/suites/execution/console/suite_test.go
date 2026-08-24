package console

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
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

func TestRunWatchedSuiteUsesGQRLProcess(t *testing.T) {
	jsPath := t.TempDir()
	var (
		gotPath string
		gotArgs []string
		command *exec.Cmd
	)
	newCommand := func(ctx context.Context, path string, arguments ...string) *exec.Cmd {
		gotPath = path
		gotArgs = slices.Clone(arguments)
		command = watchedConsoleCommand(ctx, "success")
		return command
	}

	err := runWatchedSuiteWithCommand(
		t.Context(),
		"/execution-image/gqrl",
		jsPath,
		"ws://execution.example:8546",
		"events",
		newCommand,
	)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/execution-image/gqrl" {
		t.Fatalf("got executable %q", gotPath)
	}
	if len(gotArgs) != 8 {
		t.Fatalf("got arguments %q", gotArgs)
	}
	dataDir := gotArgs[2]
	wantArgs := []string{
		"attach",
		"--datadir", dataDir,
		"--jspath", jsPath,
		"--preload", "harness.js,events.js",
		"ws://execution.example:8546",
	}
	if !slices.Equal(gotArgs, wantArgs) {
		t.Fatalf("got arguments %q, want %q", gotArgs, wantArgs)
	}
	if _, err := os.Stat(dataDir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("console data directory was not removed: %v", err)
	}
	if command.ProcessState == nil || !command.ProcessState.Success() {
		t.Fatalf("console process did not exit cleanly: %v", command.ProcessState)
	}
}

func TestRunWatchedSuiteRejectsScriptFailure(t *testing.T) {
	var command *exec.Cmd
	err := runWatchedSuiteWithCommand(
		t.Context(),
		"gqrl",
		t.TempDir(),
		"ws://execution.example:8546",
		"events",
		func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			command = watchedConsoleCommand(ctx, "failure")
			return command
		},
	)
	if err == nil {
		t.Fatal("watched suite accepted an explicit script failure")
	}
	if !strings.Contains(err.Error(), "emitted a failure marker") ||
		!strings.Contains(err.Error(), "helper failure") {
		t.Fatalf("unexpected error: %v", err)
	}
	if command.ProcessState == nil {
		t.Fatal("failed console process was not reaped")
	}
}

func TestRunWatchedSuiteRejectsEarlyExit(t *testing.T) {
	var command *exec.Cmd
	err := runWatchedSuiteWithCommand(
		t.Context(),
		"gqrl",
		t.TempDir(),
		"ws://execution.example:8546",
		"events",
		func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			command = watchedConsoleCommand(ctx, "early-exit")
			return command
		},
	)
	if err == nil || !strings.Contains(err.Error(), "emitted 0 success markers") {
		t.Fatalf("unexpected early-exit result: %v", err)
	}
	if command.ProcessState == nil || !command.ProcessState.Success() {
		t.Fatalf("early-exit process was not reaped: %v", command.ProcessState)
	}
}

func TestRunWatchedSuiteTerminatesOnTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	var command *exec.Cmd
	err := runWatchedSuiteWithCommand(
		ctx,
		"gqrl",
		t.TempDir(),
		"ws://execution.example:8546",
		"events",
		func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			command = watchedConsoleCommand(ctx, "timeout")
			return command
		},
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want context deadline", err)
	}
	if command.ProcessState == nil {
		t.Fatal("timed-out console process was not reaped")
	}
}

func watchedConsoleCommand(ctx context.Context, mode string) *exec.Cmd {
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
