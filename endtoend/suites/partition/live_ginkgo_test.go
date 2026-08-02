//go:build e2e

package partition

import (
	"context"
	"fmt"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensus"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const partitionTimeout = 15 * time.Minute

type liveSuite struct {
	environment devnet.Environment
	beacons     []*consensus.Client
	partition   *devnet.NetworkPartition
}

var _ = ginkgo.Describe(
	"two-way QRL network partitions",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "partition", "multi-node", "mutates-network", "scenario"),
	func() {
		var suite *liveSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			environment, err := devnet.Inspect(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			if len(environment.Participants) < 4 {
				ginkgo.Skip("partition scenarios require the four-participant chaos profile")
			}
			suite = &liveSuite{environment: environment, partition: devnet.NewNetworkPartition()}
			for _, participant := range environment.Participants {
				beacon, err := consensus.New(participant.ConsensusURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				suite.beacons = append(suite.beacons, beacon)
			}
		})

		ginkgo.AfterEach(func() {
			if suite == nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			gomega.Expect(suite.partition.Clear(ctx)).To(gomega.Succeed())
		})

		ginkgo.It("stalls finality during a balanced split and resumes after healing", func(ctx ginkgo.SpecContext) {
			startFinalized, err := suite.beacons[0].FinalizedEpoch(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			slotsPerEpoch, err := suite.beacons[0].SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			startHeads := suite.heads(ctx)

			gomega.Expect(suite.apply(ctx)).To(gomega.Succeed())
			gomega.Eventually(func(g gomega.Gomega) {
				heads := suite.heads(ctx)
				for index, head := range heads {
					g.Expect(head.Slot).To(gomega.BeNumerically(">=", startHeads[index].Slot+2*slotsPerEpoch))
					finalized, err := suite.beacons[index].FinalizedEpoch(ctx)
					g.Expect(err).NotTo(gomega.HaveOccurred())
					headEpoch := head.Slot / slotsPerEpoch
					g.Expect(headEpoch).To(gomega.BeNumerically(">=", finalized))
					g.Expect(headEpoch - finalized).To(gomega.BeNumerically(">=", 2))
				}
			}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(2 * time.Second).Should(gomega.Succeed())

			gomega.Expect(suite.partition.Clear(ctx)).To(gomega.Succeed())
			suite.awaitConvergenceAndFinality(ctx, startFinalized)
		}, ginkgo.SpecTimeout(partitionTimeout), ginkgo.Label(
			"scenario:dev:two-way-network-split-non-finality",
			"scenario:dev:validator-lifecycle-test",
			"behavior:partition:finality-stall",
			"behavior:partition:finality-recovery",
		))

		ginkgo.It("converges after competing heads form across a split", func(ctx ginkgo.SpecContext) {
			startFinalized, err := suite.beacons[0].FinalizedEpoch(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			slotsPerEpoch, err := suite.beacons[0].SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			start := suite.heads(ctx)

			gomega.Expect(suite.apply(ctx)).To(gomega.Succeed())
			gomega.Eventually(func(g gomega.Gomega) {
				heads := suite.heads(ctx)
				g.Expect(heads[0].Slot).To(gomega.BeNumerically(">=", start[0].Slot+slotsPerEpoch))
				g.Expect(heads[2].Slot).To(gomega.BeNumerically(">=", start[2].Slot+slotsPerEpoch))
				g.Expect(heads[0].Root).NotTo(gomega.Equal(heads[2].Root))
			}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(time.Second).Should(gomega.Succeed())

			gomega.Expect(suite.partition.Clear(ctx)).To(gomega.Succeed())
			suite.awaitConvergenceAndFinality(ctx, startFinalized)
		}, ginkgo.SpecTimeout(partitionTimeout), ginkgo.Label(
			"scenario:dev:two-way-network-split-reorg-trigger",
			"behavior:partition:competing-heads",
			"behavior:partition:reorg-recovery",
		))
	},
)

func (suite *liveSuite) apply(ctx context.Context) error {
	participants := suite.environment.Participants
	middle := len(participants) / 2
	return suite.partition.Apply(ctx, participants[:middle], participants[middle:])
}

func (suite *liveSuite) heads(ctx context.Context) []consensus.Head {
	ginkgo.GinkgoHelper()
	heads := make([]consensus.Head, len(suite.beacons))
	for index, beacon := range suite.beacons {
		head, err := beacon.Head(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		heads[index] = head
	}
	return heads
}

func (suite *liveSuite) awaitConvergenceAndFinality(ctx context.Context, previousFinalized uint64) {
	ginkgo.GinkgoHelper()
	gomega.Eventually(func() error {
		heads := suite.heads(ctx)
		for index := 1; index < len(heads); index++ {
			if heads[index].Root != heads[0].Root {
				return fmt.Errorf("participant heads have not converged")
			}
		}
		for _, beacon := range suite.beacons {
			finalized, err := beacon.FinalizedEpoch(ctx)
			if err != nil {
				return err
			}
			if finalized <= previousFinalized {
				return fmt.Errorf("finalized epoch %d has not advanced past %d", finalized, previousFinalized)
			}
		}
		return nil
	}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(2 * time.Second).Should(gomega.Succeed())
}
