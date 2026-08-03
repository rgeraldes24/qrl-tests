// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

// Package build provides helpers for building repository binaries used by
// end-to-end suites.
package build

import (
	"context"
	"fmt"
	"os/exec"
)

// Binary builds packagePath from a go-qrl source checkout.
func Binary(ctx context.Context, sourceDir, packagePath, output string) error {
	command := exec.CommandContext(ctx, "go", "build", "-o", output, packagePath)
	command.Dir = sourceDir
	if commandOutput, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("build %s: %w\n%s", packagePath, err, commandOutput)
	}
	return nil
}
