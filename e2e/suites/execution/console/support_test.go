package console

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/cyyber/qrl-tests/e2e/internal/consolefixture"
	endtoendlive "github.com/cyyber/qrl-tests/e2e/internal/live"
)

const (
	resultPrefix  = "CONSOLE_E2E_PASS "
	failurePrefix = "CONSOLE_E2E_FAIL "

	watchedSuitePollInterval = 100 * time.Millisecond
	watchedSuiteExitTimeout  = 5 * time.Second
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

func runSuite(ctx context.Context, gqrlPath, jsPath, rpcURL, name string) error {
	expression := "loadScript('harness.js');loadScript('" + name + ".js')"
	command := exec.CommandContext(
		ctx,
		gqrlPath,
		"attach",
		"--jspath",
		jsPath,
		"--exec",
		expression,
		rpcURL,
	)
	output, err := command.CombinedOutput()
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

type commandContext func(context.Context, string, ...string) *exec.Cmd

func runWatchedSuite(ctx context.Context, gqrlPath, jsPath, webSocketURL, name string) error {
	return runWatchedSuiteWithCommand(
		ctx,
		gqrlPath,
		jsPath,
		webSocketURL,
		name,
		exec.CommandContext,
	)
}

func runWatchedSuiteWithCommand(
	ctx context.Context,
	gqrlPath string,
	jsPath string,
	webSocketURL string,
	name string,
	newCommand commandContext,
) (result error) {
	dataDir, err := os.MkdirTemp("", "qrl-tests-console-"+name+"-")
	if err != nil {
		return fmt.Errorf("create console data directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(dataDir); err != nil {
			result = errors.Join(result, fmt.Errorf("remove console data directory: %w", err))
		}
	}()

	command := newCommand(
		ctx,
		gqrlPath,
		"attach",
		"--datadir",
		dataDir,
		"--jspath",
		jsPath,
		"--preload",
		"harness.js,"+name+".js",
		webSocketURL,
	)
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
