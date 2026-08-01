//go:build e2e

package network

import (
	"context"
	"fmt"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	pollInterval    = time.Second
	progressTimeout = 2 * time.Minute
	stabilityWindow = 30 * time.Second
)

type liveSuite struct {
	session   *endtoendlive.Session
	consensus *consensus.Client
}

var _ = ginkgo.Describe(
	"QRL network health",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "network", "assertoor"),
	func() {
		var suite *liveSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			session, err := endtoendlive.Open(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(session.Close)

			beacon, err := consensus.New(session.Environment.ConsensusURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite = &liveSuite{session: session, consensus: beacon}
		})

		ginkgo.It("reports synchronized execution and consensus clients", func(ctx ginkgo.SpecContext) {
			progress, err := suite.session.Client.SyncProgress(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(progress).To(gomega.BeNil())

			gomega.Expect(suite.consensus.Health(ctx)).To(gomega.Succeed())
			status, err := suite.consensus.Syncing(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(status.Syncing).To(gomega.BeFalse())
			gomega.Expect(status.Optimistic).To(gomega.BeFalse())
			gomega.Expect(status.ELOffline).To(gomega.BeFalse())
		})

		ginkgo.It("observes new execution and consensus blocks", func(ctx ginkgo.SpecContext) {
			startBlock, err := suite.session.Client.BlockNumber(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			startSlot, err := suite.consensus.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(func(g gomega.Gomega) {
				block, err := suite.session.Client.BlockNumber(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(block).To(gomega.BeNumerically(">", startBlock))

				slot, err := suite.consensus.HeadSlot(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(slot).To(gomega.BeNumerically(">=", startSlot+2))
			}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(pollInterval).Should(gomega.Succeed())
		})

		ginkgo.It("reaches a finalized consensus epoch", func(ctx ginkgo.SpecContext) {
			gomega.Eventually(func(g gomega.Gomega) {
				epoch, err := suite.consensus.FinalizedEpoch(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(epoch).To(gomega.BeNumerically(">=", 1))
			}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(pollInterval).Should(gomega.Succeed())
		})

		ginkgo.It("observes active validators producing attestations", func(ctx ginkgo.SpecContext) {
			active, err := suite.consensus.ActiveValidatorCount(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(active).To(gomega.BeNumerically(">", 0))

			startSlot, err := suite.consensus.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Eventually(func(g gomega.Gomega) {
				slot, err := suite.consensus.HeadSlot(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(slot).To(gomega.BeNumerically(">", startSlot))

				attestations, err := suite.consensus.BlockAttestationCount(ctx, "head")
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(attestations).To(gomega.BeNumerically(">", 0))
			}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(pollInterval).Should(gomega.Succeed())
		})

		ginkgo.It("keeps heads and finality monotonic while the chain advances", func(ctx ginkgo.SpecContext) {
			startBlock, startSlot, startFinalized := suite.snapshot(ctx)
			lastBlock, lastSlot, lastFinalized := startBlock, startSlot, startFinalized
			deadline := time.NewTimer(stabilityWindow)
			defer deadline.Stop()
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					ginkgo.Fail(ctx.Err().Error())
				case <-deadline.C:
					gomega.Expect(lastBlock).To(gomega.BeNumerically(">", startBlock))
					gomega.Expect(lastSlot).To(gomega.BeNumerically(">", startSlot))
					return
				case <-ticker.C:
					block, slot, finalized := suite.snapshot(ctx)
					gomega.Expect(block).To(gomega.BeNumerically(">=", lastBlock))
					gomega.Expect(slot).To(gomega.BeNumerically(">=", lastSlot))
					gomega.Expect(finalized).To(gomega.BeNumerically(">=", lastFinalized))
					lastBlock, lastSlot, lastFinalized = block, slot, finalized
				}
			}
		}, ginkgo.SpecTimeout(stabilityWindow+30*time.Second))
	},
)

func (suite *liveSuite) snapshot(ctx context.Context) (uint64, uint64, uint64) {
	ginkgo.GinkgoHelper()
	progress, err := suite.session.Client.SyncProgress(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(progress).To(gomega.BeNil(), fmt.Sprintf("execution client is syncing: %+v", progress))

	status, err := suite.consensus.Syncing(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(status.Syncing || status.Optimistic || status.ELOffline).To(gomega.BeFalse())

	block, err := suite.session.Client.BlockNumber(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	slot, err := suite.consensus.HeadSlot(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	finalized, err := suite.consensus.FinalizedEpoch(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return block, slot, finalized
}
