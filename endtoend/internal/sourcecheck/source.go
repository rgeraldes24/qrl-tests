// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

// Package sourcecheck keeps the go-qrl library used by the test binary aligned
// with the checkout used to build helper binaries and local images.
package sourcecheck

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
)

const goQRLModule = "github.com/theQRL/go-qrl"

func GoQRL(ctx context.Context, sourceDir string) error {
	if sourceDir == "" {
		return nil
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return errors.New("read qrl-tests build information")
	}
	for _, dependency := range info.Deps {
		if dependency.Path != goQRLModule {
			continue
		}
		module := dependency
		if dependency.Replace != nil {
			module = dependency.Replace
		}
		return verifyModule(ctx, sourceDir, module)
	}
	return fmt.Errorf("qrl-tests build has no %s dependency", goQRLModule)
}

func verifyModule(ctx context.Context, sourceDir string, module *debug.Module) error {
	source, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}
	if module.Version == "" || module.Version == "(devel)" {
		dependency, err := filepath.Abs(module.Path)
		if err != nil {
			return err
		}
		if samePath(source, dependency) {
			return nil
		}
		return fmt.Errorf("go-qrl source mismatch: tests use %s, helpers use %s", dependency, source)
	}

	head, err := gitOutput(ctx, source, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("read go-qrl source revision: %w", err)
	}
	if revision := pseudoVersionRevision(module.Version); revision != "" && strings.HasPrefix(head, revision) {
		return nil
	}
	tag, tagErr := gitOutput(ctx, source, "describe", "--tags", "--exact-match")
	if tagErr == nil && tag == module.Version {
		return nil
	}
	return fmt.Errorf("go-qrl source mismatch: tests use %s, helpers use %s", module.Version, head)
}

func samePath(left, right string) bool {
	leftResolved, leftErr := filepath.EvalSymlinks(left)
	rightResolved, rightErr := filepath.EvalSymlinks(right)
	return leftErr == nil && rightErr == nil && leftResolved == rightResolved
}

func pseudoVersionRevision(version string) string {
	parts := strings.Split(version, "-")
	if len(parts) < 3 {
		return ""
	}
	return parts[len(parts)-1]
}

func gitOutput(ctx context.Context, directory string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", directory}, arguments...)...)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(output)), nil
}
