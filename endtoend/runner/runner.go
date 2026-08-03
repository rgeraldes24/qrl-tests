// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

// Package runner executes the registered end-to-end test lanes.
package runner

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/lanes"
)

const defaultReportDir = "reports"

type runnerConfig struct {
	sourceDir string
	baseName  string
	reportDir string
	backend   devnet.Backend
	images    devnet.Images
}

// Run executes an E2E runner command.
func Run(ctx context.Context, arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: e2e list | run <lane> | run-all")
	}
	switch arguments[0] {
	case "list":
		for _, lane := range lanes.All() {
			fmt.Printf("%-16s profile=%-16s timeout=%s\n", lane.Name, lane.Profile, lane.Timeout)
		}
		return nil
	case "run":
		if len(arguments) != 2 {
			return errors.New("usage: e2e run <lane>")
		}
		lane, err := lanes.Named(arguments[1])
		if err != nil {
			return err
		}
		configuration, err := loadConfig()
		if err != nil {
			return err
		}
		lane, supported := lane.ForBackend(configuration.backend)
		if !supported {
			return fmt.Errorf("lane %s is unsupported by %s backend", lane.Name, configuration.backend)
		}
		return runLane(ctx, lane, false, configuration)
	case "run-all":
		if len(arguments) != 1 {
			return errors.New("usage: e2e run-all")
		}
		configuration, err := loadConfig()
		if err != nil {
			return err
		}
		var result error
		for _, lane := range lanes.All() {
			lane, supported := lane.ForBackend(configuration.backend)
			if !supported {
				fmt.Printf("=== SKIP lane=%s: unsupported by %s backend ===\n", lane.Name, configuration.backend)
				continue
			}
			if err := runLane(ctx, lane, true, configuration); err != nil {
				result = errors.Join(result, err)
			}
		}
		return result
	default:
		return fmt.Errorf("unknown command %q", arguments[0])
	}
}

func loadConfig() (runnerConfig, error) {
	sourceDir := strings.TrimSpace(os.Getenv("GO_QRL_SOURCE_DIR"))
	if sourceDir == "" {
		return runnerConfig{}, errors.New("GO_QRL_SOURCE_DIR must point to a go-qrl checkout")
	}
	backend, err := devnet.ParseBackend(os.Getenv("DEVNET_BACKEND"))
	if err != nil {
		return runnerConfig{}, err
	}
	return runnerConfig{
		sourceDir: sourceDir,
		baseName:  cmp.Or(strings.TrimSpace(os.Getenv("DEVNET_ENCLAVE_NAME")), devnet.DefaultEnclaveName),
		reportDir: cmp.Or(strings.TrimSpace(os.Getenv("E2E_REPORT_DIR")), defaultReportDir),
		backend:   backend,
		images: devnet.Images{
			Execution: cmp.Or(strings.TrimSpace(os.Getenv("DEVNET_EXECUTION_IMAGE")), devnet.DefaultExecutionImage),
			Clef:      cmp.Or(strings.TrimSpace(os.Getenv("DEVNET_CLEF_IMAGE")), devnet.DefaultClefImage),
			Consensus: cmp.Or(strings.TrimSpace(os.Getenv("DEVNET_CONSENSUS_IMAGE")), devnet.DefaultConsensusImage),
			Validator: cmp.Or(strings.TrimSpace(os.Getenv("DEVNET_VALIDATOR_IMAGE")), devnet.DefaultValidatorImage),
			Genesis:   cmp.Or(strings.TrimSpace(os.Getenv("DEVNET_GENESIS_IMAGE")), devnet.DefaultGenesisImage),
		},
	}, nil
}

func runLane(ctx context.Context, lane lanes.Lane, suffixEnclave bool, configuration runnerConfig) error {
	enclaveName := configuration.baseName
	if suffixEnclave {
		enclaveName += "-" + lane.Name
	}

	manager := devnet.NewManager()
	startCtx, cancelStart := context.WithTimeout(ctx, devnet.DefaultStartTimeout)
	err := manager.Start(startCtx, devnet.StartOptions{
		EnclaveName: enclaveName,
		Backend:     configuration.backend,
		Images:      configuration.images,
		Profile:     lane.Profile,
	})
	cancelStart()
	if err != nil {
		return fmt.Errorf("lane %s: start network: %w", lane.Name, err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := manager.Stop(stopCtx, enclaveName); err != nil {
			fmt.Fprintf(os.Stderr, "lane %s: stop network: %v\n", lane.Name, err)
		}
	}()

	reportDir := filepath.Join(configuration.reportDir, lane.Name)
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return fmt.Errorf("lane %s: create report directory: %w", lane.Name, err)
	}
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
	arguments = append(arguments, lane.Packages...)
	arguments = append(arguments, "--", "-test.run=^TestE2E$")

	laneCtx, cancelLane := context.WithTimeout(ctx, lane.Timeout+5*time.Minute)
	defer cancelLane()
	command := exec.CommandContext(laneCtx, "go", arguments...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = append(os.Environ(),
		"DEVNET_ENCLAVE_NAME="+enclaveName,
		"DEVNET_BACKEND="+string(configuration.backend),
		"DEVNET_PROFILE="+string(lane.Profile),
		"GO_QRL_SOURCE_DIR="+configuration.sourceDir,
	)
	fmt.Printf("=== RUN lane=%s profile=%s ===\n", lane.Name, lane.Profile)
	if err := command.Run(); err != nil {
		return fmt.Errorf("lane %s: %w", lane.Name, err)
	}
	return nil
}
