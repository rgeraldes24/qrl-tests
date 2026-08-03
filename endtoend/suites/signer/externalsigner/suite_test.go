// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package externalsigner

import (
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
)

func TestE2E(t *testing.T) {
	testsuite.Run(t, "External signer live E2E suite")
}
