//go:build e2e

package optimistic_test

import (
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
)

func TestE2E(t *testing.T) {
	testsuite.Run(t, "Optimistic Consensus Sync E2E Suite")
}
