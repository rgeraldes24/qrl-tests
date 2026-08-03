//go:build e2e

package engine_test

import (
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
)

func TestE2E(t *testing.T) {
	testsuite.Run(t, "Engine and cross-layer E2E Suite")
}
