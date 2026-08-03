// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package main

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
	"github.com/cyyber/qrl-tests/internal/lanes"
)

const (
	defaultExecutionImage = "local/go-qrl:devnet"
	defaultReportDir      = "reports"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string) error {
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
		return runLane(ctx, lane, false)
	case "run-all":
		if len(arguments) != 1 {
			return errors.New("usage: e2e run-all")
		}
		var result error
		for _, lane := range lanes.All() {
			if err := runLane(ctx, lane, true); err != nil {
				result = errors.Join(result, err)
			}
		}
		return result
	default:
		return fmt.Errorf("unknown command %q", arguments[0])
	}
}

func runLane(ctx context.Context, lane lanes.Lane, suffixEnclave bool) error {
	sourceDir := strings.TrimSpace(os.Getenv("GO_QRL_SOURCE_DIR"))
	if sourceDir == "" {
		return errors.New("GO_QRL_SOURCE_DIR must point to a go-qrl checkout")
	}
	baseName := cmp.Or(strings.TrimSpace(os.Getenv("DEVNET_ENCLAVE_NAME")), devnet.DefaultEnclaveName)
	enclaveName := baseName
	if suffixEnclave {
		enclaveName += "-" + lane.Name
	}
	executionImage := cmp.Or(strings.TrimSpace(os.Getenv("DEVNET_EXECUTION_IMAGE")), defaultExecutionImage)

	manager := devnet.NewManager()
	startCtx, cancelStart := context.WithTimeout(ctx, devnet.DefaultStartTimeout)
	err := manager.Start(startCtx, devnet.StartOptions{
		EnclaveName:    enclaveName,
		ExecutionImage: executionImage,
		Profile:        lane.Profile,
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

	reportDir := filepath.Join(cmp.Or(os.Getenv("E2E_REPORT_DIR"), defaultReportDir), lane.Name)
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
		"DEVNET_PROFILE="+string(lane.Profile),
		"GO_QRL_SOURCE_DIR="+sourceDir,
	)
	fmt.Printf("=== RUN lane=%s profile=%s ===\n", lane.Name, lane.Profile)
	if err := command.Run(); err != nil {
		return fmt.Errorf("lane %s: %w", lane.Name, err)
	}
	return nil
}
