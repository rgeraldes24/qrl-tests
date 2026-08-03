//go:build e2e

package soak_test

import (
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
)

func TestE2E(t *testing.T) {
	testsuite.Run(t, "QRL Soak E2E Suite")
}
