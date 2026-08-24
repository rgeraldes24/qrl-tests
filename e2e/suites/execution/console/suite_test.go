package console

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	dockerclient "github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

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

func TestEventsFixtureMarkerOrder(t *testing.T) {
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
	process          func(context.Context) consoleContainerProcess
	createErr        error
	copyErr          error
	startErr         error
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

func (engine *fakeConsoleEngine) start(
	ctx context.Context,
	containerID string,
	interactive bool,
) (consoleContainerProcess, error) {
	engine.startID = containerID
	engine.startInteractive = interactive
	if engine.startErr != nil {
		return nil, engine.startErr
	}
	return engine.process(ctx), nil
}

func (engine *fakeConsoleEngine) remove(ctx context.Context, containerID string) error {
	engine.removed = true
	engine.removeContextErr = ctx.Err()
	if containerID != engine.containerID {
		return fmt.Errorf("remove unexpected container %q", containerID)
	}
	return engine.removeErr
}

type fakeDockerClient struct {
	calls         []string
	createOptions dockerclient.ContainerCreateOptions
	copyOptions   dockerclient.CopyToContainerOptions
	copyContent   []byte
	clientConn    net.Conn
	serverConn    net.Conn
}

func (client *fakeDockerClient) ContainerCreate(
	_ context.Context,
	options dockerclient.ContainerCreateOptions,
) (dockerclient.ContainerCreateResult, error) {
	client.calls = append(client.calls, "create")
	client.createOptions = options
	return dockerclient.ContainerCreateResult{ID: "container-id"}, nil
}

func (client *fakeDockerClient) CopyToContainer(
	_ context.Context,
	_ string,
	options dockerclient.CopyToContainerOptions,
) (dockerclient.CopyToContainerResult, error) {
	client.calls = append(client.calls, "copy")
	client.copyOptions = options
	content, err := io.ReadAll(options.Content)
	client.copyContent = content
	return dockerclient.CopyToContainerResult{}, err
}

func (client *fakeDockerClient) ContainerAttach(
	_ context.Context,
	_ string,
	_ dockerclient.ContainerAttachOptions,
) (dockerclient.ContainerAttachResult, error) {
	client.calls = append(client.calls, "attach")
	client.clientConn, client.serverConn = net.Pipe()
	return dockerclient.ContainerAttachResult{
		HijackedResponse: dockerclient.NewHijackedResponse(client.clientConn, "application/vnd.docker.multiplexed-stream"),
	}, nil
}

func (client *fakeDockerClient) ContainerStart(
	_ context.Context,
	_ string,
	_ dockerclient.ContainerStartOptions,
) (dockerclient.ContainerStartResult, error) {
	client.calls = append(client.calls, "start")
	return dockerclient.ContainerStartResult{}, nil
}

func (client *fakeDockerClient) ContainerWait(
	_ context.Context,
	_ string,
	options dockerclient.ContainerWaitOptions,
) dockerclient.ContainerWaitResult {
	client.calls = append(client.calls, "wait:"+string(options.Condition))
	result := make(chan containertypes.WaitResponse, 1)
	result <- containertypes.WaitResponse{}
	return dockerclient.ContainerWaitResult{Result: result, Error: make(chan error)}
}

func (client *fakeDockerClient) ContainerRemove(
	_ context.Context,
	_ string,
	options dockerclient.ContainerRemoveOptions,
) (dockerclient.ContainerRemoveResult, error) {
	client.calls = append(client.calls, fmt.Sprintf("remove:%t", options.Force))
	return dockerclient.ContainerRemoveResult{}, nil
}

func TestDockerConsoleEngine(t *testing.T) {
	jsPath := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(jsPath, "harness.js"), []byte("fixture"), 0o600))
	client := &fakeDockerClient{}
	t.Cleanup(func() {
		if client.serverConn != nil {
			_ = client.serverConn.Close()
		}
	})
	engine := dockerConsoleEngine{client: client}

	containerID, err := engine.create(t.Context(), consoleContainerSpec{
		image:       "registry.example/go-qrl@sha256:digest",
		endpoint:    "ws://127.0.0.1:8546",
		scenario:    "events",
		interactive: true,
	})
	require.NoError(t, err)
	require.Equal(t, "container-id", containerID)
	require.Equal(t, "registry.example/go-qrl@sha256:digest", client.createOptions.Config.Image)
	require.Equal(t, []string{"gqrl"}, client.createOptions.Config.Entrypoint)
	require.Equal(t, []string{
		"attach",
		"--datadir", "/tmp/qrl-tests-console",
		"--jspath", "/tmp/qrl-tests-js",
		"--preload", "harness.js,events.js",
		"ws://host.docker.internal:8546",
	}, client.createOptions.Config.Cmd)
	require.True(t, client.createOptions.Config.AttachStdin)
	require.True(t, client.createOptions.Config.AttachStdout)
	require.True(t, client.createOptions.Config.AttachStderr)
	require.True(t, client.createOptions.Config.OpenStdin)
	require.True(t, client.createOptions.Config.StdinOnce)
	require.Equal(t, []string{"host.docker.internal:host-gateway"}, client.createOptions.HostConfig.ExtraHosts)

	require.NoError(t, engine.copyFixtures(t.Context(), containerID, jsPath))
	require.Equal(t, "/tmp", client.copyOptions.DestinationPath)
	archive := tar.NewReader(bytes.NewReader(client.copyContent))
	contents := make(map[string]string)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		content, err := io.ReadAll(archive)
		require.NoError(t, err)
		contents[header.Name] = string(content)
	}
	require.Equal(t, "fixture", contents["qrl-tests-js/harness.js"])

	process, err := engine.start(t.Context(), containerID, true)
	require.NoError(t, err)
	require.NoError(t, process.wait())
	process.close()
	require.NoError(t, engine.remove(t.Context(), containerID))
	require.Equal(t, []string{"create", "copy", "attach", "wait:next-exit", "start", "remove:true"}, client.calls)
}

func TestDockerConsoleEngineExecConfig(t *testing.T) {
	client := &fakeDockerClient{}
	engine := dockerConsoleEngine{client: client}

	_, err := engine.create(t.Context(), consoleContainerSpec{
		image:    "registry.example/go-qrl@sha256:digest",
		endpoint: "http://127.0.0.1:8545",
		scenario: "api",
	})
	require.NoError(t, err)
	require.Equal(t, []string{
		"attach",
		"--datadir", "/tmp/qrl-tests-console",
		"--jspath", "/tmp/qrl-tests-js",
		"--exec", "loadScript('harness.js');loadScript('api.js')",
		"http://host.docker.internal:8545",
	}, client.createOptions.Config.Cmd)
	require.False(t, client.createOptions.Config.AttachStdin)
	require.False(t, client.createOptions.Config.OpenStdin)
	require.False(t, client.createOptions.Config.StdinOnce)
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

type fakeConsoleProcess struct {
	ctx        context.Context
	cancel     context.CancelFunc
	output     string
	readErr    error
	done       chan error
	closed     chan struct{}
	closeOnce  sync.Once
	lock       sync.Mutex
	input      strings.Builder
	inputClose bool
}

func newFakeConsoleProcess(ctx context.Context, mode string) *fakeConsoleProcess {
	processCtx, cancel := context.WithCancel(ctx)
	process := &fakeConsoleProcess{
		ctx:    processCtx,
		cancel: cancel,
		done:   make(chan error, 1),
		closed: make(chan struct{}),
	}
	switch mode {
	case "ordinary-success":
		process.output = resultPrefix + "api\n"
		process.done <- nil
	case "ordinary-failure":
		process.output = failurePrefix + "api helper failure\n"
		process.done <- nil
	case "ordinary-nonzero":
		process.output = "helper process failed\n"
		process.done <- errors.New("exit status 2")
	case "ordinary-read-error":
		process.readErr = errors.New("attach read failed")
	case "success":
		process.output = resultPrefix + "events\n"
	case "failure":
		process.output = resultPrefix + "events\n" + failurePrefix + "events helper failure\n"
	case "early-exit":
		process.output = "console exited before the script completed\n"
		process.done <- nil
	case "timeout":
	default:
		panic("unknown fake console process mode " + mode)
	}
	return process
}

func (process *fakeConsoleProcess) readOutput(destination io.Writer) error {
	if _, err := io.WriteString(destination, process.output); err != nil {
		return err
	}
	if process.readErr != nil {
		return process.readErr
	}
	<-process.closed
	return nil
}

func (process *fakeConsoleProcess) writeInput(input string) error {
	process.lock.Lock()
	_, _ = process.input.WriteString(input)
	process.lock.Unlock()
	if strings.Contains(input, "exit\n") {
		select {
		case process.done <- nil:
		default:
		}
	}
	return nil
}

func (process *fakeConsoleProcess) closeInput() error {
	process.lock.Lock()
	defer process.lock.Unlock()
	process.inputClose = true
	return nil
}

func (process *fakeConsoleProcess) wait() error {
	select {
	case err := <-process.done:
		process.close()
		return err
	case <-process.ctx.Done():
		process.close()
		return process.ctx.Err()
	}
}

func (process *fakeConsoleProcess) close() {
	process.closeOnce.Do(func() {
		process.cancel()
		process.lock.Lock()
		process.inputClose = true
		process.lock.Unlock()
		close(process.closed)
	})
}

func (process *fakeConsoleProcess) receivedInput() string {
	process.lock.Lock()
	defer process.lock.Unlock()
	return process.input.String()
}

func (process *fakeConsoleProcess) inputClosed() bool {
	process.lock.Lock()
	defer process.lock.Unlock()
	return process.inputClose
}

func TestRunSuite(t *testing.T) {
	jsPath := t.TempDir()
	engine := &fakeConsoleEngine{process: func(ctx context.Context) consoleContainerProcess {
		return newFakeConsoleProcess(ctx, "ordinary-success")
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
				process: func(ctx context.Context) consoleContainerProcess {
					return newFakeConsoleProcess(ctx, testCase.mode)
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

func TestRunSuiteTimeoutCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	engine := &fakeConsoleEngine{process: func(ctx context.Context) consoleContainerProcess {
		return newFakeConsoleProcess(ctx, "timeout")
	}}
	err := runSuiteWithEngine(ctx, "image", t.TempDir(), "http://127.0.0.1:8545", "api", engine)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.True(t, engine.removed && engine.removeContextErr == nil,
		"cleanup did not use a fresh context: %+v", engine)
}

func TestRunSuiteStopsWhenOutputStreamFails(t *testing.T) {
	engine := &fakeConsoleEngine{process: func(ctx context.Context) consoleContainerProcess {
		return newFakeConsoleProcess(ctx, "ordinary-read-error")
	}}
	err := runSuiteWithEngine(t.Context(), "image", t.TempDir(), "http://127.0.0.1:8545", "api", engine)
	require.ErrorContains(t, err, "attach read failed")
	require.True(t, engine.removed)
}

func TestRunWatchedSuite(t *testing.T) {
	var process *fakeConsoleProcess
	engine := &fakeConsoleEngine{process: func(ctx context.Context) consoleContainerProcess {
		process = newFakeConsoleProcess(ctx, "success")
		return process
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
	require.Equal(t, "exit\n", process.receivedInput())
	require.True(t, process.inputClosed())
}

func TestRunWatchedSuiteCleansUpAfterStartFailure(t *testing.T) {
	engine := &fakeConsoleEngine{startErr: errors.New("start failed")}
	err := runWatchedSuiteWithEngine(t.Context(), "image", t.TempDir(), "ws://127.0.0.1:8546", "events", engine)
	require.ErrorContains(t, err, "start console suite events")
	require.True(t, engine.removed)
}

func TestRunWatchedSuiteRejectsScriptFailure(t *testing.T) {
	var process *fakeConsoleProcess
	engine := &fakeConsoleEngine{process: func(ctx context.Context) consoleContainerProcess {
		process = newFakeConsoleProcess(ctx, "failure")
		return process
	}}
	err := runWatchedSuiteWithEngine(t.Context(), "image", t.TempDir(), "ws://127.0.0.1:8546", "events", engine)
	require.ErrorContains(t, err, "emitted a failure marker")
	require.ErrorContains(t, err, "helper failure")
	require.Equal(t, "exit\n", process.receivedInput())
	require.True(t, engine.removed)
}

func TestRunWatchedSuiteRejectsEarlyExit(t *testing.T) {
	engine := &fakeConsoleEngine{process: func(ctx context.Context) consoleContainerProcess {
		return newFakeConsoleProcess(ctx, "early-exit")
	}}
	err := runWatchedSuiteWithEngine(t.Context(), "image", t.TempDir(), "ws://127.0.0.1:8546", "events", engine)
	require.ErrorContains(t, err, "emitted 0 success markers")
	require.True(t, engine.removed)
}

func TestRunWatchedSuiteTerminatesOnTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	var process *fakeConsoleProcess
	engine := &fakeConsoleEngine{process: func(ctx context.Context) consoleContainerProcess {
		process = newFakeConsoleProcess(ctx, "timeout")
		return process
	}}
	err := runWatchedSuiteWithEngine(ctx, "image", t.TempDir(), "ws://127.0.0.1:8546", "events", engine)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.True(t, process.inputClosed())
	require.True(t, engine.removed && engine.removeContextErr == nil)
}
