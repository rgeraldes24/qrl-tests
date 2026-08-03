//go:build e2e

package sync_test

import (
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
)

func TestE2E(t *testing.T) {
	testsuite.Run(t, "QRL Execution Sync E2E Suite")
}
