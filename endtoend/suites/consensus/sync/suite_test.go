//go:build e2e

package sync

import (
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
)

func TestE2E(t *testing.T) {
	testsuite.Run(t, "Consensus startup and sync live E2E suite")
}
