//go:build e2e

package transactions

import (
	"time"

	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const transactionTimeout = 3 * time.Minute

const (
	fullCalldataTransactionCount = 1000
	fullTransactionsPerBlock     = 10
	fullCalldataSize             = 1000
	fullWorkloadTimeout          = 45 * time.Minute
)

var _ = ginkgo.Describe(
	"QRL transaction scenarios",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "transactions", "scenario", "mutates-chain"),
	func() {
		var sessions []*endtoendlive.Session

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			runtime, loadErr := endtoendlive.Load(ctx)
			gomega.Expect(loadErr).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			sessions, err = runtime.OpenAll(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		registerTransactionScenarios(&sessions)
		registerTransactionWorkloads(&sessions)
	},
)
