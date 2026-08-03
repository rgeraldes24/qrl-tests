//go:build e2e

package validator_test

import (
	"github.com/cyyber/qrl-tests/endtoend/internal/stability"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerSlashingSpec() {
	ginkgo.It("distributes proposer and attester slashings across every client pair", func(ctx ginkgo.SpecContext) {
		startFinalized, err := validatorOperations.beacon.FinalizedEpoch(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		validatorOperations.runSlashingWorkload(ctx)
		gomega.Expect(stability.Await(
			ctx, validatorOperations.sessions, startFinalized, 2,
		)).To(gomega.Succeed())
	}, ginkgo.SpecTimeout(operationsTimeout), ginkgo.Label(
		"scenario:dev:validator-proposer-slashing-test",
		"scenario:stable:kurtosis:validator-slashing-test",
		"behavior:validator:proposer-slashing-matrix",
		"behavior:validator:attester-slashing-matrix",
		"behavior:validator:operation-submission-matrix",
		"behavior:validator:slashing-proposer-matrix",
		"behavior:network:post-workload-stability",
	))
}
