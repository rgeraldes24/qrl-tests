//go:build e2e

package network

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	pollInterval    = time.Second
	progressTimeout = 10 * time.Minute
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
	ginkgo.Label("e2e", "live", "network", "assertoor"),
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
				progress, err := current.session.Client.SyncProgress(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(progress).To(gomega.BeNil())

				gomega.Expect(current.consensus.Health(ctx)).To(gomega.Succeed())
				status, err := current.consensus.Syncing(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(status.Syncing || status.Optimistic || status.ELOffline).To(gomega.BeFalse())
			}
		}, ginkgo.Label("assertoor:dev:synchronized-check"))

		ginkgo.It("observes new execution and consensus blocks on every client pair", func(ctx ginkgo.SpecContext) {
			for _, current := range suite.nodes {
				startBlock, err := current.session.Client.BlockNumber(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				startSlot, err := current.consensus.HeadSlot(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				gomega.Eventually(func(g gomega.Gomega) {
					block, err := current.session.Client.BlockNumber(ctx)
					g.Expect(err).NotTo(gomega.HaveOccurred())
					g.Expect(block).To(gomega.BeNumerically(">", startBlock))

					slot, err := current.consensus.HeadSlot(ctx)
					g.Expect(err).NotTo(gomega.HaveOccurred())
					g.Expect(slot).To(gomega.BeNumerically(">=", startSlot+2))
				}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(pollInterval).Should(gomega.Succeed())
			}
		}, ginkgo.Label("assertoor:dev:wait-for-slot"))

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
		}, ginkgo.Label("assertoor:stable:block-proposal-check"))

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
		}, ginkgo.Label("assertoor:stable:stability-check"))

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
		}, ginkgo.Label("assertoor:dev:generate-attestations"))

		ginkgo.It("keeps client heads converged for one epoch", func(ctx ginkgo.SpecContext) {
			slotsPerEpoch, err := suite.nodes[0].consensus.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			start, err := suite.nodes[0].consensus.Head(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			roots := make(map[int]map[uint64]string, len(suite.nodes))

			gomega.Eventually(func(g gomega.Gomega) {
				var expectedRoot string
				for _, current := range suite.nodes {
					head, err := current.consensus.Head(ctx)
					g.Expect(err).NotTo(gomega.HaveOccurred())
					if roots[current.session.Participant.Index] == nil {
						roots[current.session.Participant.Index] = make(map[uint64]string)
					}
					if previous := roots[current.session.Participant.Index][head.Slot]; previous != "" {
						g.Expect(head.Root).To(gomega.Equal(previous), "head root changed at an observed slot")
					}
					roots[current.session.Participant.Index][head.Slot] = head.Root
					if expectedRoot == "" {
						expectedRoot = head.Root
					} else {
						g.Expect(head.Root).To(gomega.Equal(expectedRoot))
					}
					g.Expect(head.Slot).To(gomega.BeNumerically(">=", start.Slot))
				}
				head, err := suite.nodes[0].consensus.HeadSlot(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(head).To(gomega.BeNumerically(">=", start.Slot+slotsPerEpoch))
			}).WithContext(ctx).WithTimeout(progressTimeout).WithPolling(2 * time.Second).Should(gomega.Succeed())
		}, ginkgo.Label("assertoor:stable:stability-check"))
	},
)

func decodeGraffiti(value string) string {
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(decoded), "\x00")
}
