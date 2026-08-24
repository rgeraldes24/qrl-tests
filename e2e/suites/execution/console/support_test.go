package console

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cyyber/qrl-tests/e2e/internal/consolefixture"
	endtoendlive "github.com/cyyber/qrl-tests/e2e/internal/live"
)

const (
	resultPrefix  = "CONSOLE_E2E_PASS "
	failurePrefix = "CONSOLE_E2E_FAIL "

	consoleContainerJSPath         = "/tmp/qrl-tests-js"
	consoleContainerDataDir        = "/tmp/qrl-tests-console"
	consoleContainerHost           = "host.docker.internal"
	consoleContainerCleanupTimeout = 30 * time.Second
	watchedSuitePollInterval       = 100 * time.Millisecond
	watchedSuiteExitTimeout        = 5 * time.Second
)

type consoleScenario struct {
	name        string
	description string
	webSocket   bool
}

var consoleScenarios = []consoleScenario{
	{
		name:        "api",
		description: "validates console and RPC APIs against the live network",
	},
	{
		name:        "contract",
		description: "deploys a contract and validates VM64 ABI, receipts, events, and filters",
	},
	{
		name:        "topics",
		description: "formats and decodes indexed VM64 scalar topics",
	},
	{
		name:        "constructor",
		description: "deploys a contract through the embedded web3 contract factory",
		webSocket:   true,
	},
	{
		name:        "events",
		description: "formats and submits a contract transaction and watches indexed events over WebSocket",
		webSocket:   true,
	},
}

//go:embed testdata/console/*.js
var consoleFixtures embed.FS

type outputCommand func(context.Context, string, ...string) ([]byte, error)

type commandContext func(context.Context, string, ...string) *exec.Cmd

type consoleContainerSpec struct {
	image       string
	endpoint    string
	scenario    string
	interactive bool
}

type consoleContainerEngine interface {
	create(context.Context, consoleContainerSpec) (string, error)
	copyFixtures(context.Context, string, string) error
	start(context.Context, string, bool) *exec.Cmd
	remove(context.Context, string) error
}

type dockerConsoleEngine struct {
	output  outputCommand
	command commandContext
}

func newDockerConsoleEngine() dockerConsoleEngine {
	return dockerConsoleEngine{
		output:  executeOutput,
		command: exec.CommandContext,
	}
}

func executeOutput(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, arguments...)
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		if detail := strings.TrimSpace(string(exitError.Stderr)); detail != "" {
			return output, fmt.Errorf("%w: %s", err, detail)
		}
	}
	return output, err
}

func consoleContainerEndpoint(endpoint string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse console endpoint: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("parse console endpoint: URL must include a scheme and host")
	}

	host := strings.TrimSuffix(parsed.Hostname(), ".")
	if host == "" {
		return "", fmt.Errorf("parse console endpoint: URL must include a host")
	}
	address := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (address == nil || !address.IsLoopback()) {
		return endpoint, nil
	}

	port := parsed.Port()
	parsed.Host = consoleContainerHost
	if port != "" {
		parsed.Host = net.JoinHostPort(consoleContainerHost, port)
	}
	return parsed.String(), nil
}

func (engine dockerConsoleEngine) create(ctx context.Context, spec consoleContainerSpec) (string, error) {
	endpoint, err := consoleContainerEndpoint(spec.endpoint)
	if err != nil {
		return "", fmt.Errorf("create console suite %s container: %w", spec.scenario, err)
	}

	arguments := []string{"create", "--pull=missing"}
	if spec.interactive {
		arguments = append(arguments, "--interactive")
	}
	arguments = append(
		arguments,
		"--add-host", consoleContainerHost+"=host-gateway",
		"--entrypoint", "gqrl",
		spec.image,
		"attach",
		"--datadir", consoleContainerDataDir,
		"--jspath", consoleContainerJSPath,
	)
	if spec.interactive {
		arguments = append(arguments, "--preload", "harness.js,"+spec.scenario+".js")
	} else {
		expression := "loadScript('harness.js');loadScript('" + spec.scenario + ".js')"
		arguments = append(arguments, "--exec", expression)
	}
	arguments = append(arguments, endpoint)

	output, err := engine.output(ctx, "docker", arguments...)
	if err != nil {
		return "", fmt.Errorf("create console suite %s container: %w", spec.scenario, err)
	}
	containerID := strings.TrimSpace(string(output))
	if containerID == "" {
		return "", fmt.Errorf("create console suite %s container: docker returned no container ID", spec.scenario)
	}
	return containerID, nil
}

func (engine dockerConsoleEngine) copyFixtures(ctx context.Context, containerID, jsPath string) error {
	jsPath, err := filepath.Abs(jsPath)
	if err != nil {
		return fmt.Errorf("resolve console fixture directory: %w", err)
	}
	if _, err := engine.output(ctx, "docker", "cp", jsPath, containerID+":"+consoleContainerJSPath); err != nil {
		return fmt.Errorf("docker cp: %w", err)
	}
	return nil
}

func (engine dockerConsoleEngine) start(ctx context.Context, containerID string, interactive bool) *exec.Cmd {
	arguments := []string{"start", "--attach"}
	if interactive {
		arguments = append(arguments, "--interactive")
	}
	return engine.command(ctx, "docker", append(arguments, containerID)...)
}

func (engine dockerConsoleEngine) remove(ctx context.Context, containerID string) error {
	if _, err := engine.output(ctx, "docker", "rm", "--force", containerID); err != nil {
		return fmt.Errorf("docker rm: %w", err)
	}
	return nil
}

func removeConsoleContainer(engine consoleContainerEngine, containerID, scenario string) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), consoleContainerCleanupTimeout)
	defer cancel()
	if err := engine.remove(cleanupCtx, containerID); err != nil {
		return fmt.Errorf("remove console suite %s container: %w", scenario, err)
	}
	return nil
}

func runSuite(ctx context.Context, image, jsPath, rpcURL, name string) error {
	return runSuiteWithEngine(ctx, image, jsPath, rpcURL, name, newDockerConsoleEngine())
}

func runSuiteWithEngine(
	ctx context.Context,
	image string,
	jsPath string,
	rpcURL string,
	name string,
	engine consoleContainerEngine,
) (result error) {
	containerID, err := engine.create(ctx, consoleContainerSpec{
		image:    image,
		endpoint: rpcURL,
		scenario: name,
	})
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, removeConsoleContainer(engine, containerID, name))
	}()
	if err := engine.copyFixtures(ctx, containerID, jsPath); err != nil {
		return fmt.Errorf("copy console suite %s fixtures: %w", name, err)
	}

	output, err := engine.start(ctx, containerID, false).CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("console suite %s: %w\n%s", name, ctx.Err(), output)
	}
	if err != nil {
		return fmt.Errorf("run console suite %s: %w\n%s", name, err, output)
	}
	if err := parseSuiteResult(name, output); err != nil {
		return fmt.Errorf("%w\n%s", err, output)
	}
	return nil
}

type synchronizedBuffer struct {
	lock sync.Mutex
	data bytes.Buffer
}

func (buffer *synchronizedBuffer) Write(data []byte) (int, error) {
	buffer.lock.Lock()
	defer buffer.lock.Unlock()
	return buffer.data.Write(data)
}

func (buffer *synchronizedBuffer) Bytes() []byte {
	buffer.lock.Lock()
	defer buffer.lock.Unlock()
	return bytes.Clone(buffer.data.Bytes())
}

func runWatchedSuite(ctx context.Context, image, jsPath, webSocketURL, name string) error {
	return runWatchedSuiteWithEngine(
		ctx,
		image,
		jsPath,
		webSocketURL,
		name,
		newDockerConsoleEngine(),
	)
}

func runWatchedSuiteWithEngine(
	ctx context.Context,
	image string,
	jsPath string,
	webSocketURL string,
	name string,
	engine consoleContainerEngine,
) (result error) {
	containerID, err := engine.create(ctx, consoleContainerSpec{
		image:       image,
		endpoint:    webSocketURL,
		scenario:    name,
		interactive: true,
	})
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, removeConsoleContainer(engine, containerID, name))
	}()
	if err := engine.copyFixtures(ctx, containerID, jsPath); err != nil {
		return fmt.Errorf("copy console suite %s fixtures: %w", name, err)
	}

	command := engine.start(ctx, containerID, true)
	return runWatchedCommand(ctx, command, name)
}

func runWatchedCommand(ctx context.Context, command *exec.Cmd, name string) error {
	stdin, err := command.StdinPipe()
	if err != nil {
		return fmt.Errorf("open console suite %s stdin: %w", name, err)
	}
	defer stdin.Close()

	var output synchronizedBuffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		return fmt.Errorf("start console suite %s: %w", name, err)
	}

	processDone := make(chan error, 1)
	go func() {
		processDone <- command.Wait()
	}()

	ticker := time.NewTicker(watchedSuitePollInterval)
	defer ticker.Stop()
	for {
		result := output.Bytes()
		successes, failed := suiteMarkers(name, result)
		if failed || bytes.Contains(result, []byte("GoError:")) || successes > 0 {
			processErr := stopWatchedCommand(command, stdin, processDone, true)
			result = output.Bytes()
			if ctx.Err() != nil {
				return fmt.Errorf("console suite %s: %w\n%s", name, ctx.Err(), result)
			}
			return finishWatchedSuite(name, result, processErr)
		}
		select {
		case <-ctx.Done():
			stopErr := stopWatchedCommand(command, stdin, processDone, false)
			result = output.Bytes()
			return errors.Join(
				fmt.Errorf("console suite %s: %w\n%s", name, ctx.Err(), result),
				stopErr,
			)
		case processErr := <-processDone:
			result = output.Bytes()
			if ctx.Err() != nil {
				return fmt.Errorf("console suite %s: %w\n%s", name, ctx.Err(), result)
			}
			return finishWatchedSuite(name, result, processErr)
		case <-ticker.C:
		}
	}
}

func stopWatchedCommand(
	command *exec.Cmd,
	stdin io.WriteCloser,
	processDone <-chan error,
	graceful bool,
) error {
	if graceful {
		_, _ = io.WriteString(stdin, "exit\n")
	}
	_ = stdin.Close()

	var killErr error
	if !graceful && command.Process != nil {
		killErr = command.Process.Kill()
		if errors.Is(killErr, os.ErrProcessDone) {
			killErr = nil
		}
	}

	timer := time.NewTimer(watchedSuiteExitTimeout)
	defer timer.Stop()
	select {
	case processErr := <-processDone:
		if graceful {
			return processErr
		}
		return killErr
	case <-timer.C:
		if command.Process != nil {
			err := command.Process.Kill()
			if !errors.Is(err, os.ErrProcessDone) {
				killErr = errors.Join(killErr, err)
			}
		}
		processErr := <-processDone
		return errors.Join(
			fmt.Errorf("console process did not exit within %s", watchedSuiteExitTimeout),
			killErr,
			processErr,
		)
	}
}

func finishWatchedSuite(name string, output []byte, processErr error) error {
	if bytes.Contains(output, []byte("GoError:")) {
		return fmt.Errorf("console suite %s failed with GoError\n%s", name, output)
	}
	if err := parseSuiteResult(name, output); err != nil {
		return fmt.Errorf("%w\n%s", err, output)
	}
	if processErr != nil {
		return fmt.Errorf("console suite %s process failed: %w\n%s", name, processErr, output)
	}
	return nil
}

func parseSuiteResult(name string, output []byte) error {
	matches, failed := suiteMarkers(name, output)
	if failed {
		return fmt.Errorf("console suite %s emitted a failure marker", name)
	}
	if matches != 1 {
		return fmt.Errorf("console suite %s emitted %d success markers", name, matches)
	}
	return nil
}

func suiteMarkers(name string, output []byte) (successes int, failed bool) {
	successMarker := []byte(resultPrefix + name)
	failureMarker := []byte(failurePrefix + name)
	failureDetailPrefix := append(bytes.Clone(failureMarker), ' ')
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if bytes.Equal(line, failureMarker) ||
			bytes.HasPrefix(line, failureDetailPrefix) {
			failed = true
		}
		if bytes.Equal(line, successMarker) {
			successes++
		}
	}
	return successes, failed
}

func prepareWorkspace(ctx context.Context, destination string, session *endtoendlive.Node) error {
	consoleScripts, err := fs.Sub(consoleFixtures, "testdata/console")
	if err != nil {
		return fmt.Errorf("open console fixtures: %w", err)
	}
	if err := os.CopyFS(destination, consoleScripts); err != nil {
		return fmt.Errorf("copy console fixtures: %w", err)
	}

	bytecode, err := consolefixture.Bytecode()
	if err != nil {
		return fmt.Errorf("decode console contract bytecode: %w", err)
	}

	params, err := deploymentParameters(ctx, session, consolefixture.ABI, bytecode)
	if err != nil {
		return err
	}
	script := append([]byte("var PARAMS = "), params...)
	script = append(script, ';', '\n')
	if err := os.WriteFile(filepath.Join(destination, ".params.js"), script, 0o600); err != nil {
		return fmt.Errorf("write console parameters: %w", err)
	}
	return nil
}
