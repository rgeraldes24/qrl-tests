//go:build e2e

package network

import (
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/behavior"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerReadinessSpecs() {
	ginkgo.It("reports every execution and consensus client synchronized", func(ctx ginkgo.SpecContext) {
		for _, current := range networkSuite.nodes {
			progress, err := current.session.Execution.SyncProgress(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(progress).To(gomega.BeNil())

			gomega.Expect(current.consensus.Health(ctx)).To(gomega.Succeed())
			status, err := current.consensus.Syncing(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(status.Syncing || status.Optimistic || status.ELOffline).To(gomega.BeFalse())
		}
	}, ginkgo.Label(
		"scenario:dev:synchronized-check",
		behavior.Name("network:clients-synchronized"),
	))

	ginkgo.It("observes new execution and consensus blocks on every client pair", func(ctx ginkgo.SpecContext) {
		for _, current := range networkSuite.nodes {
			startBlock, err := current.session.Execution.BlockNumber(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			startSlot, err := current.consensus.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(func(g gomega.Gomega) {
				block, err := current.session.Execution.BlockNumber(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(block).To(gomega.BeNumerically(">", startBlock))

				slot, err := current.consensus.HeadSlot(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(slot).To(gomega.BeNumerically(">=", startSlot+2))
			}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(pollInterval).Should(gomega.Succeed())
		}
	}, ginkgo.Label(
		"scenario:dev:wait-for-slot",
		behavior.Name("network:slot-progress"),
	))

	ginkgo.It("observes proposals from every validator pair", func(ctx ginkgo.SpecContext) {
		expected := make(map[string]struct{}, len(networkSuite.nodes))
		for _, current := range networkSuite.nodes {
			name := strings.TrimPrefix(current.session.Participant.Validator.Name, "vc-")
			gomega.Expect(name).NotTo(gomega.BeEmpty())
			expected[name] = struct{}{}
		}
		observed := make(map[string]struct{}, len(expected))
		lastSlot := uint64(0)
		gomega.Eventually(func(g gomega.Gomega) {
			head, err := networkSuite.nodes[0].consensus.Head(ctx)
			g.Expect(err).NotTo(gomega.HaveOccurred())
			if head.Slot != lastSlot {
				graffiti, err := networkSuite.nodes[0].consensus.BlockGraffitiText(ctx, "head")
				g.Expect(err).NotTo(gomega.HaveOccurred())
				observed[graffiti] = struct{}{}
				lastSlot = head.Slot
			}
			for name := range expected {
				g.Expect(observed).To(gomega.HaveKey(name))
			}
		}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(pollInterval).Should(gomega.Succeed())
	}, ginkgo.Label(
		"scenario:stable:block-proposal-check",
		behavior.Name("network:proposer-coverage"),
	))
}
