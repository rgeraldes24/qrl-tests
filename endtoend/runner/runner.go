// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

// Package runner executes the registered end-to-end test lanes.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/lanes"
	"github.com/cyyber/qrl-tests/endtoend/internal/runenv"
	"github.com/cyyber/qrl-tests/endtoend/internal/sourcecheck"
)

const DefaultReportDir = "reports"

type Config struct {
	SourceDir    string
	BaseName     string
	ReportDir    string
	Backend      devnet.Backend
	Images       devnet.Images
	StartTimeout time.Duration
}

type networkManager interface {
	Start(context.Context, devnet.StartOptions) (devnet.Environment, error)
	Inspect(context.Context, string, devnet.Backend) (devnet.Environment, error)
	Stop(context.Context, string) error
}

type commandSpec struct {
	Path   string
	Args   []string
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

type runMode uint8

const (
	useExistingNetwork runMode = iota
	provisionNetwork
	provisionPerLane
)

func (mode runMode) provisions() bool {
	return mode != useExistingNetwork
}

func (mode runMode) suffixesEnclave() bool {
	return mode == provisionPerLane
}

type Runner struct {
	configuration Config
	networks      networkManager
	buildBinary   func(context.Context, string, string, string) error
	runCommand    func(context.Context, commandSpec) error
	stdout        io.Writer
	stderr        io.Writer
}

func New(configuration Config, stdout, stderr io.Writer) *Runner {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &Runner{
		configuration: configuration,
		networks:      devnet.NewManager(),
		buildBinary:   buildBinary,
		runCommand:    execute,
		stdout:        stdout,
		stderr:        stderr,
	}
}

func buildBinary(ctx context.Context, sourceDir, packagePath, output string) error {
	command := exec.CommandContext(ctx, "go", "build", "-o", output, packagePath)
	command.Dir = sourceDir
	if commandOutput, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("build %s: %w\n%s", packagePath, err, commandOutput)
	}
	return nil
}

func execute(ctx context.Context, specification commandSpec) error {
	command := exec.CommandContext(ctx, specification.Path, specification.Args...)
	command.Env = specification.Env
	command.Stdout = specification.Stdout
	command.Stderr = specification.Stderr
	return command.Run()
}

func (runner *Runner) List() error {
	for _, lane := range lanes.All() {
		if _, err := fmt.Fprintf(runner.stdout, "%-16s profile=%-16s timeout=%s\n", lane.Name, lane.Profile, lane.Timeout); err != nil {
			return err
		}
	}
	return nil
}

func (runner *Runner) Test(ctx context.Context, name string) error {
	lane, err := runner.supportedLane(name)
	if err != nil {
		return err
	}
	return runner.run(ctx, []lanes.Lane{lane}, useExistingNetwork)
}

func (runner *Runner) Run(ctx context.Context, name string) error {
	lane, err := runner.supportedLane(name)
	if err != nil {
		return err
	}
	return runner.run(ctx, []lanes.Lane{lane}, provisionNetwork)
}

func (runner *Runner) RunAll(ctx context.Context) error {
	selected := make([]lanes.Lane, 0, len(lanes.All()))
	for _, lane := range lanes.All() {
		lane, supported := lane.ForBackend(runner.configuration.Backend)
		if !supported {
			fmt.Fprintf(runner.stdout, "=== SKIP lane=%s: unsupported by %s backend ===\n", lane.Name, runner.configuration.Backend)
			continue
		}
		selected = append(selected, lane)
	}
	return runner.run(ctx, selected, provisionPerLane)
}

func (runner *Runner) supportedLane(name string) (lanes.Lane, error) {
	lane, err := lanes.Named(name)
	if err != nil {
		return lanes.Lane{}, err
	}
	lane, supported := lane.ForBackend(runner.configuration.Backend)
	if !supported {
		return lanes.Lane{}, fmt.Errorf("lane %s is unsupported by %s backend", lane.Name, runner.configuration.Backend)
	}
	return lane, nil
}

func (runner *Runner) run(ctx context.Context, selected []lanes.Lane, mode runMode) error {
	plan, err := newRunPlan(runner.configuration, selected, mode)
	if err != nil {
		return err
	}
	if err := sourcecheck.GoQRL(ctx, runner.configuration.SourceDir); err != nil {
		return err
	}
	if err := os.MkdirAll(plan.reportRoot, 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	tools, err := runner.buildTools(ctx, plan.reportRoot, plan.tools)
	if err != nil {
		return err
	}

	var result error
	for _, lane := range plan.lanes {
		if err := runner.runLane(ctx, lane, tools); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func (runner *Runner) runLane(
	ctx context.Context,
	planned laneRun,
	tools runenv.Tools,
) (result error) {
	lane := planned.lane
	if err := os.MkdirAll(planned.reportDir, 0o755); err != nil {
		return fmt.Errorf("lane %s: create report directory: %w", lane.Name, err)
	}

	var environment devnet.Environment
	var err error
	if planned.provision {
		startCtx, cancelStart := context.WithTimeout(ctx, runner.configuration.StartTimeout)
		environment, err = runner.networks.Start(startCtx, devnet.StartOptions{
			EnclaveName: planned.enclaveName,
			Backend:     runner.configuration.Backend,
			Images:      runner.configuration.Images,
			Profile:     lane.Profile,
		})
		cancelStart()
		if err != nil {
			return fmt.Errorf("lane %s: start network: %w", lane.Name, err)
		}
		defer func() {
			stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := runner.networks.Stop(stopCtx, planned.enclaveName); err != nil {
				result = errors.Join(result, fmt.Errorf("lane %s: stop network: %w", lane.Name, err))
			}
		}()
	} else {
		environment, err = runner.networks.Inspect(ctx, planned.enclaveName, runner.configuration.Backend)
		if err != nil {
			return fmt.Errorf("lane %s: inspect network: %w", lane.Name, err)
		}
	}

	if err := runenv.Write(planned.manifestPath, runenv.Manifest{
		Lane:        lane.Name,
		Profile:     lane.Profile,
		Environment: environment,
		Tools:       tools,
	}); err != nil {
		return fmt.Errorf("lane %s: %w", lane.Name, err)
	}

	laneCtx, cancelLane := context.WithTimeout(ctx, lane.Timeout+5*time.Minute)
	defer cancelLane()
	fmt.Fprintf(runner.stdout, "=== RUN lane=%s profile=%s ===\n", lane.Name, lane.Profile)
	if err := runner.runCommand(laneCtx, commandSpec{
		Path:   "go",
		Args:   planned.arguments,
		Env:    append(os.Environ(), runenv.PathEnv+"="+planned.manifestPath),
		Stdout: runner.stdout,
		Stderr: runner.stderr,
	}); err != nil {
		return fmt.Errorf("lane %s: %w", lane.Name, err)
	}
	return nil
}

func ginkgoArguments(lane lanes.Lane, reportDir string) []string {
	arguments := []string{
		"tool", "ginkgo",
		"--tags=e2e",
		"--procs=1",
		"--keep-going",
		"--require-suite",
		"--fail-on-empty",
		"--fail-on-pending",
		"--timeout=" + lane.Timeout.String(),
		"--output-dir=" + reportDir,
		"--junit-report=junit.xml",
		"--json-report=report.json",
	}
	if lane.LabelFilter != "" {
		arguments = append(arguments, "--label-filter="+lane.LabelFilter)
	}
	arguments = append(arguments, lane.Packages()...)
	return append(arguments, "--", "-test.run=^TestE2E$")
}

func requiredTools(selected []lanes.Lane) []lanes.Tool {
	var required []lanes.Tool
	for _, lane := range selected {
		for _, tool := range lane.Tools {
			if !slices.Contains(required, tool) {
				required = append(required, tool)
			}
		}
	}
	return required
}

func (runner *Runner) buildTools(ctx context.Context, reportRoot string, tools []lanes.Tool) (runenv.Tools, error) {
	var result runenv.Tools
	if len(tools) == 0 {
		return result, nil
	}
	if runner.configuration.SourceDir == "" {
		return result, errors.New("GO_QRL_SOURCE_DIR must point to a go-qrl checkout")
	}
	binDir := filepath.Join(reportRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return result, fmt.Errorf("create tool directory: %w", err)
	}
	for _, tool := range tools {
		output := filepath.Join(binDir, string(tool))
		switch tool {
		case lanes.ToolGQRL:
			if err := runner.buildBinary(ctx, runner.configuration.SourceDir, "./cmd/gqrl", output); err != nil {
				return result, err
			}
			result.GQRL = output
		case lanes.ToolClef:
			if err := runner.buildBinary(ctx, runner.configuration.SourceDir, "./cmd/clef", output); err != nil {
				return result, err
			}
			result.Clef = output
		default:
			return result, fmt.Errorf("unknown lane tool %q", tool)
		}
	}
	return result, nil
}
