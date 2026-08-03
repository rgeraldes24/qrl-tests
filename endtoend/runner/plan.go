// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package runner

import (
	"fmt"
	"path/filepath"

	"github.com/cyyber/qrl-tests/endtoend/internal/lanes"
)

type laneRun struct {
	lane         lanes.Lane
	enclaveName  string
	reportDir    string
	manifestPath string
	arguments    []string
	provision    bool
}

type runPlan struct {
	reportRoot string
	tools      []lanes.Tool
	lanes      []laneRun
}

func newRunPlan(configuration Config, selected []lanes.Lane, mode runMode) (runPlan, error) {
	reportRoot, err := filepath.Abs(configuration.ReportDir)
	if err != nil {
		return runPlan{}, fmt.Errorf("resolve report directory: %w", err)
	}
	plan := runPlan{
		reportRoot: reportRoot,
		tools:      requiredTools(selected),
		lanes:      make([]laneRun, len(selected)),
	}
	for index, lane := range selected {
		enclaveName := configuration.BaseName
		if mode.suffixesEnclave() {
			enclaveName += "-" + lane.Name
		}
		reportDir := filepath.Join(reportRoot, lane.Name)
		plan.lanes[index] = laneRun{
			lane:         lane,
			enclaveName:  enclaveName,
			reportDir:    reportDir,
			manifestPath: filepath.Join(reportDir, "environment.json"),
			arguments:    ginkgoArguments(lane, reportDir),
			provision:    mode.provisions(),
		}
	}
	return plan, nil
}
