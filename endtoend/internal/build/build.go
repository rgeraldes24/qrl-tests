// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

// Package build provides helpers for building repository binaries used by
// end-to-end suites.
package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

const sourceDirectoryEnv = "GO_QRL_SOURCE_DIR"

// Binary builds packagePath from the configured go-qrl source checkout.
func Binary(ctx context.Context, packagePath, output string) error {
	root := os.Getenv(sourceDirectoryEnv)
	if root == "" {
		return errors.New(sourceDirectoryEnv + " is not set")
	}
	command := exec.CommandContext(ctx, "go", "build", "-o", output, packagePath)
	command.Dir = root
	if commandOutput, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("build %s: %w\n%s", packagePath, err, commandOutput)
	}
	return nil
}
