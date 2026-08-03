//go:build e2e

package network

import (
	"fmt"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerParticipationSpecs() {
	ginkgo.It("observes every active validator participating", func(ctx ginkgo.SpecContext) {
		beacon := networkSuite.nodes[0].consensus
		slotsPerEpoch, err := beacon.SpecUint(ctx, "SLOTS_PER_EPOCH")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		indices, err := beacon.ActiveValidatorIndices(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(indices).NotTo(gomega.BeEmpty())
		startSlot, err := beacon.HeadSlot(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		firstEpoch := startSlot/slotsPerEpoch + 1
		lastEpoch := firstEpoch + 2

		gomega.Eventually(func(g gomega.Gomega) {
			head, err := beacon.HeadSlot(ctx)
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(head / slotsPerEpoch).To(gomega.BeNumerically(">", lastEpoch))
		}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(pollInterval).Should(gomega.Succeed())

		observed := make(map[uint64]bool, len(indices))
		for epoch := firstEpoch; epoch <= lastEpoch; epoch++ {
			participation, err := beacon.Liveness(ctx, epoch, indices)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(participation).To(gomega.HaveLen(len(indices)))
			for _, validator := range participation {
				observed[validator.Index] = observed[validator.Index] || validator.IsLive
			}
		}
		for _, index := range indices {
			gomega.Expect(observed[index]).To(
				gomega.BeTrue(),
				fmt.Sprintf("validator %d did not participate between epochs %d and %d", index, firstEpoch, lastEpoch),
			)
		}
	}, ginkgo.Label(
		"scenario:dev:generate-attestations",
		"behavior:network:validator-liveness",
	))

	ginkgo.It("maintains target and head participation thresholds", func(ctx ginkgo.SpecContext) {
		beacon := networkSuite.nodes[0].consensus
		slotsPerEpoch, err := beacon.SpecUint(ctx, "SLOTS_PER_EPOCH")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Eventually(func(g gomega.Gomega) {
			head, err := beacon.HeadSlot(ctx)
			g.Expect(err).NotTo(gomega.HaveOccurred())
			currentEpoch := head / slotsPerEpoch
			g.Expect(currentEpoch).To(gomega.BeNumerically(">=", 3))
		}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(pollInterval).Should(gomega.Succeed())

		participation, err := beacon.ValidatorParticipation(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(participation.PreviousActive).NotTo(gomega.BeZero())
		targetPercent := 100 * float64(participation.PreviousTarget) / float64(participation.PreviousActive)
		headPercent := 100 * float64(participation.PreviousHead) / float64(participation.PreviousActive)
		gomega.Expect(targetPercent).To(gomega.BeNumerically(">=", minimumTarget))
		gomega.Expect(headPercent).To(gomega.BeNumerically(">=", minimumHead))
	}, ginkgo.Label(
		"scenario:stable:stability-check",
		"behavior:network:attestation-thresholds",
	))
}
