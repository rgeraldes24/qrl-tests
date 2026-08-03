// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package sourcecheck

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyModule(t *testing.T) {
	source := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(source, ".git"), 0o755))
	require.NoError(t, verifyModule(t.Context(), source, &debug.Module{Path: source}))

	repository := t.TempDir()
	runGit(t, repository, "init", "--quiet")
	runGit(t, repository, "config", "user.email", "tests@example.invalid")
	runGit(t, repository, "config", "user.name", "qrl-tests")
	require.NoError(t, os.WriteFile(filepath.Join(repository, "README"), []byte("test\n"), 0o600))
	runGit(t, repository, "add", "README")
	runGit(t, repository, "commit", "--quiet", "-m", "test")
	head := runGit(t, repository, "rev-parse", "HEAD")
	require.NoError(t, verifyModule(t.Context(), repository, &debug.Module{
		Version: "v0.0.0-20260801000000-" + head[:12],
	}))
}

func runGit(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.CommandContext(context.Background(), "git", append([]string{"-C", directory}, arguments...)...)
	output, err := command.Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(output))
}
