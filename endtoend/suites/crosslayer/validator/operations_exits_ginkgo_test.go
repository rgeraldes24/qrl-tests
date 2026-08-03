//go:build e2e

package validator_test

import (
	"github.com/cyyber/qrl-tests/endtoend/internal/stability"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerExitSpec() {
	ginkgo.It("submits and includes 64 voluntary exits through every consensus client", func(ctx ginkgo.SpecContext) {
		startFinalized, err := validatorOperations.beacon.FinalizedEpoch(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		validatorOperations.runExitWorkload(ctx)
		gomega.Expect(stability.Await(
			ctx, validatorOperations.sessions, startFinalized, 2,
		)).To(gomega.Succeed())
	}, ginkgo.SpecTimeout(operationsTimeout), ginkgo.Label(
		"scenario:pectra-dev:kurtosis:voluntary-exits",
		"scenario:stable:kurtosis:validator-exit-test",
		"behavior:validator:voluntary-exit-64",
		"behavior:validator:operation-submission-matrix",
		"behavior:validator:exit-proposer-matrix",
		"behavior:network:post-workload-stability",
	))
}
