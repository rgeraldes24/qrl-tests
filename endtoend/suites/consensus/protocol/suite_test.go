//go:build e2e

package protocol_test

import (
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
)

func TestE2E(t *testing.T) {
	testsuite.Run(t, "Consensus Protocol E2E Suite")
}
