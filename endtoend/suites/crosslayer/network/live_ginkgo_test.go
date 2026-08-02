//go:build e2e

package network

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	pollInterval    = time.Second
	progressTimeout = 10 * time.Minute
	minimumTarget   = 98.0
	minimumHead     = 80.0
)

type node struct {
	session   *endtoendlive.Session
	consensus *consensus.Client
}

type liveSuite struct {
	nodes []node
}

var _ = ginkgo.Describe(
	"QRL network health",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "network", "scenario"),
	func() {
		var suite *liveSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			sessions, err := endtoendlive.OpenAll(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite = new(liveSuite)
			for _, session := range sessions {
				ginkgo.DeferCleanup(session.Close)
				beacon, err := consensus.New(session.Participant.ConsensusURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				suite.nodes = append(suite.nodes, node{session: session, consensus: beacon})
			}
		})

		ginkgo.It("reports every execution and consensus client synchronized", func(ctx ginkgo.SpecContext) {
			for _, current := range suite.nodes {
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
			"behavior:network:clients-synchronized",
		))

		ginkgo.It("observes new execution and consensus blocks on every client pair", func(ctx ginkgo.SpecContext) {
			for _, current := range suite.nodes {
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
			"behavior:network:slot-progress",
		))

		ginkgo.It("observes proposals from every validator pair", func(ctx ginkgo.SpecContext) {
			expected := make(map[string]struct{}, len(suite.nodes))
			for _, current := range suite.nodes {
				name := strings.TrimPrefix(current.session.Participant.ValidatorServiceName, "vc-")
				gomega.Expect(name).NotTo(gomega.BeEmpty())
				expected[name] = struct{}{}
			}
			observed := make(map[string]struct{}, len(expected))
			lastSlot := uint64(0)
			gomega.Eventually(func(g gomega.Gomega) {
				head, err := suite.nodes[0].consensus.Head(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				if head.Slot != lastSlot {
					graffiti, err := suite.nodes[0].consensus.BlockGraffiti(ctx, "head")
					g.Expect(err).NotTo(gomega.HaveOccurred())
					observed[decodeGraffiti(graffiti)] = struct{}{}
					lastSlot = head.Slot
				}
				for name := range expected {
					g.Expect(observed).To(gomega.HaveKey(name))
				}
			}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(pollInterval).Should(gomega.Succeed())
		}, ginkgo.Label(
			"scenario:stable:block-proposal-check",
			"behavior:network:proposer-coverage",
		))

		ginkgo.It("finalizes two new epochs without excessive finality lag", func(ctx ginkgo.SpecContext) {
			type baseline struct {
				finalized     uint64
				slotsPerEpoch uint64
			}
			baselines := make([]baseline, len(suite.nodes))
			for index, current := range suite.nodes {
				start, err := current.consensus.FinalizedEpoch(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				slotsPerEpoch, err := current.consensus.SpecUint(ctx, "SLOTS_PER_EPOCH")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				baselines[index] = baseline{finalized: start, slotsPerEpoch: slotsPerEpoch}
			}

			gomega.Eventually(func(g gomega.Gomega) {
				for index, current := range suite.nodes {
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
			"behavior:network:finality",
		))

		ginkgo.It("observes every active validator participating", func(ctx ginkgo.SpecContext) {
			beacon := suite.nodes[0].consensus
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
			beacon := suite.nodes[0].consensus
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

		ginkgo.It("keeps client heads converged for one epoch", func(ctx ginkgo.SpecContext) {
			slotsPerEpoch, err := suite.nodes[0].consensus.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			forks, forkDistance, reorgs := suite.observeCanonicalHistory(ctx, slotsPerEpoch)
			gomega.Expect(reorgs).To(gomega.BeNumerically("<=", 2))
			gomega.Expect(forks).To(gomega.BeNumerically("<=", 3))
			gomega.Expect(forkDistance).To(gomega.BeNumerically("<=", 3))
		}, ginkgo.Label(
			"scenario:stable:stability-check",
			"behavior:network:reorg-budget",
			"behavior:network:fork-budget",
		))
	},
)

func (suite *liveSuite) observeCanonicalHistory(ctx ginkgo.SpecContext, slotCount uint64) (int, int, int) {
	ginkgo.GinkgoHelper()

	start := minimumHeadSlot(ctx, suite.nodes)
	nextSlot := start + 1
	endSlot := start + slotCount
	observed := make(map[uint64]map[int]string)
	forkCount := 0
	forkDistance := 0
	currentForkDistance := 0

	gomega.Eventually(func(g gomega.Gomega) {
		minimum := minimumHeadSlotWithGomega(ctx, suite.nodes, g)
		for nextSlot <= minimum && nextSlot <= endSlot {
			roots := make(map[int]string, len(suite.nodes))
			distinct := make(map[string]struct{})
			for _, current := range suite.nodes {
				header, err := current.consensus.Header(ctx, fmt.Sprint(nextSlot))
				if consensus.IsNotFound(err) {
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

func minimumHeadSlot(ctx ginkgo.SpecContext, nodes []node) uint64 {
	ginkgo.GinkgoHelper()
	minimum := ^uint64(0)
	for _, current := range nodes {
		head, err := current.consensus.HeadSlot(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		if head < minimum {
			minimum = head
		}
	}
	return minimum
}

func minimumHeadSlotWithGomega(ctx ginkgo.SpecContext, nodes []node, g gomega.Gomega) uint64 {
	minimum := ^uint64(0)
	for _, current := range nodes {
		head, err := current.consensus.HeadSlot(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		if head < minimum {
			minimum = head
		}
	}
	return minimum
}

func decodeGraffiti(value string) string {
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(decoded), "\x00")
}
