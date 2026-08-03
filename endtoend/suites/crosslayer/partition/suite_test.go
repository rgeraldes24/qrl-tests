//go:build e2e

package partition

import (
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
)

func TestE2E(t *testing.T) {
	testsuite.Run(t, "QRL Network Partition E2E Suite")
}
