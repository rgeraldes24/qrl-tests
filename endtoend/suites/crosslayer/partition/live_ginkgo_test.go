//go:build e2e

package partition

import (
	"context"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/behavior"
	consensus "github.com/cyyber/qrl-tests/endtoend/internal/consensus/client"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const partitionTimeout = 15 * time.Minute

type liveSuite struct {
	environment devnet.Environment
	sessions    []*endtoendlive.Session
	beacons     []*consensus.Client
	partition   devnet.NetworkPartition
}

var _ = ginkgo.Describe(
	"two-way QRL network partitions",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "partition", "multi-node", "mutates-network", "scenario"),
	func() {
		var suite liveSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			runtime, err := endtoendlive.Load(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			sessions, err := runtime.OpenAll(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			environment := sessions[0].Environment
			if len(environment.Participants) < 4 {
				ginkgo.Skip("partition scenarios require the four-participant chaos profile")
			}
			partition, err := devnet.NewNetworkPartition(environment.Backend)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite = liveSuite{environment: environment, sessions: sessions, partition: partition}
			for _, session := range sessions {
				suite.beacons = append(suite.beacons, session.Consensus)
			}
		})

		ginkgo.AfterEach(func() {
			if suite.partition == nil {
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
			behavior.Name("partition:finality-stall"),
			behavior.Name("partition:finality-recovery"),
		))

		ginkgo.It("converges after competing heads form across a split", func(ctx ginkgo.SpecContext) {
			startFinalized, err := suite.beacons[0].FinalizedEpoch(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			slotsPerEpoch, err := suite.beacons[0].SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			start := suite.heads(ctx)
			streams := make([]<-chan consensus.Event, len(suite.beacons))
			failures := make([]<-chan error, len(suite.beacons))
			for index, beacon := range suite.beacons {
				streams[index], failures[index], err = beacon.Events(ctx, "chain_reorg")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			}

			gomega.Expect(suite.apply(ctx)).To(gomega.Succeed())
			gomega.Eventually(func(g gomega.Gomega) {
				heads := suite.heads(ctx)
				g.Expect(heads[0].Slot).To(gomega.BeNumerically(">=", start[0].Slot+slotsPerEpoch))
				g.Expect(heads[2].Slot).To(gomega.BeNumerically(">=", start[2].Slot+slotsPerEpoch))
				g.Expect(heads[0].Root).NotTo(gomega.Equal(heads[2].Root))
			}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(time.Second).Should(gomega.Succeed())

			gomega.Expect(suite.partition.Clear(ctx)).To(gomega.Succeed())
			suite.awaitConvergenceAndFinality(ctx, startFinalized)
			gomega.Eventually(func() bool {
				for index := range streams {
					select {
					case event, ok := <-streams[index]:
						if ok && event.Topic == "chain_reorg" && len(event.Data) > 0 {
							return true
						}
					case streamErr, ok := <-failures[index]:
						if ok {
							gomega.Expect(streamErr).NotTo(gomega.HaveOccurred())
						}
					default:
					}
				}
				return false
			}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(100 * time.Millisecond).Should(gomega.BeTrue())
		}, ginkgo.SpecTimeout(partitionTimeout), ginkgo.Label(
			"scenario:dev:two-way-network-split-reorg-trigger",
			behavior.Name("partition:competing-heads"),
			behavior.Name("partition:reorg-recovery"),
			behavior.Name("consensus-api:chain-reorg-event"),
		))

		registerExecutionReorgScenario(&suite)
		registerTransactionReinjectionScenario(&suite)
	},
)
