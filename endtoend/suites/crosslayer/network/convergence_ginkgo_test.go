//go:build e2e

package network

import (
	"context"
	"fmt"

	"github.com/cyyber/qrl-tests/endtoend/internal/behavior"
	"github.com/cyyber/qrl-tests/endtoend/internal/clients/beacon"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerFinalizationSpec() {
	ginkgo.It("finalizes two new epochs without excessive finality lag", func(ctx ginkgo.SpecContext) {
		type baseline struct {
			finalized     uint64
			slotsPerEpoch uint64
		}
		baselines := make([]baseline, len(networkSuite.nodes))
		for index, current := range networkSuite.nodes {
			start, err := current.consensus.FinalizedEpoch(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			slotsPerEpoch, err := current.consensus.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			baselines[index] = baseline{finalized: start, slotsPerEpoch: slotsPerEpoch}
		}

		gomega.Eventually(func(g gomega.Gomega) {
			for index, current := range networkSuite.nodes {
				finalized, err := current.consensus.FinalizedEpoch(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(finalized).To(gomega.BeNumerically(">=", baselines[index].finalized+2))
				head, err := current.consensus.HeadSlot(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				headEpoch := head / baselines[index].slotsPerEpoch
				g.Expect(headEpoch).To(gomega.BeNumerically(">=", finalized))
				g.Expect(headEpoch - finalized).To(gomega.BeNumerically("<=", 3))
			}
		}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(pollInterval).Should(gomega.Succeed())
	}, ginkgo.Label(
		"scenario:stable:stability-check",
		behavior.Name("network:finality"),
	))
}

func registerHeadConvergenceSpec() {
	ginkgo.It("keeps client heads converged for one epoch", func(ctx ginkgo.SpecContext) {
		slotsPerEpoch, err := networkSuite.nodes[0].consensus.SpecUint(ctx, "SLOTS_PER_EPOCH")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		forks, forkDistance, reorgs := networkSuite.observeCanonicalHistory(ctx, slotsPerEpoch)
		gomega.Expect(reorgs).To(gomega.BeNumerically("<=", 2))
		gomega.Expect(forks).To(gomega.BeNumerically("<=", 3))
		gomega.Expect(forkDistance).To(gomega.BeNumerically("<=", 3))
	}, ginkgo.Label(
		"scenario:stable:stability-check",
		behavior.Name("network:reorg-budget"),
		behavior.Name("network:fork-budget"),
	))
}

func (suite *liveSuite) observeCanonicalHistory(ctx ginkgo.SpecContext, slotCount uint64) (int, int, int) {
	ginkgo.GinkgoHelper()

	start, err := minimumHeadSlot(ctx, suite.nodes)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	nextSlot := start + 1
	endSlot := start + slotCount
	observed := make(map[uint64]map[int]string)
	forkCount := 0
	forkDistance := 0
	currentForkDistance := 0

	gomega.Eventually(func(g gomega.Gomega) {
		minimum, err := minimumHeadSlot(ctx, suite.nodes)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		for nextSlot <= minimum && nextSlot <= endSlot {
			roots := make(map[int]string, len(suite.nodes))
			distinct := make(map[string]struct{})
			for _, current := range suite.nodes {
				header, err := current.consensus.Header(ctx, fmt.Sprint(nextSlot))
				if beacon.IsNotFound(err) {
					continue
				}
				g.Expect(err).NotTo(gomega.HaveOccurred())
				roots[current.session.Participant.Index] = header.Root
				distinct[header.Root] = struct{}{}
			}
			if len(roots) > 0 {
				observed[nextSlot] = roots
			}
			if len(roots) > 0 && (len(roots) != len(suite.nodes) || len(distinct) > 1) {
				forkCount++
				currentForkDistance++
				if currentForkDistance > forkDistance {
					forkDistance = currentForkDistance
				}
			} else {
				currentForkDistance = 0
			}
			nextSlot++
		}
		g.Expect(nextSlot).To(gomega.BeNumerically(">", endSlot))
	}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(pollInterval).Should(gomega.Succeed())

	reorgedSlots := make(map[uint64]struct{})
	for slot, roots := range observed {
		for _, current := range suite.nodes {
			previous, ok := roots[current.session.Participant.Index]
			if !ok {
				continue
			}
			header, err := current.consensus.Header(ctx, fmt.Sprint(slot))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			if header.Root != previous {
				reorgedSlots[slot] = struct{}{}
			}
		}
	}
	return forkCount, forkDistance, len(reorgedSlots)
}

func minimumHeadSlot(ctx context.Context, nodes []node) (uint64, error) {
	minimum := ^uint64(0)
	for _, current := range nodes {
		head, err := current.consensus.HeadSlot(ctx)
		if err != nil {
			return 0, err
		}
		if head < minimum {
			minimum = head
		}
	}
	return minimum, nil
}
