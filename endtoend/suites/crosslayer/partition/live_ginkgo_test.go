//go:build e2e

package partition

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/cyyber/qrl-tests/endtoend/internal/execfixture"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"

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
		var suite *liveSuite

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
			suite = &liveSuite{environment: environment, sessions: sessions, partition: partition}
			for _, session := range sessions {
				suite.beacons = append(suite.beacons, session.Consensus)
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
			"behavior:partition:competing-heads",
			"behavior:partition:reorg-recovery",
			"behavior:consensus-api:chain-reorg-event",
		))

		ginkgo.It("reverts losing execution state after competing transactions", func(ctx ginkgo.SpecContext) {
			contract, err := execfixture.DeployStateContract(ctx, suite.sessions[0], execfixture.FullTopic(0x90))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			baseline := execfixture.FullWord(0x10)
			nonce, err := suite.sessions[0].Execution.PendingNonceAt(ctx, suite.sessions[0].Address)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			baselineTx, err := execfixture.SignCall(ctx, suite.sessions[0], nonce, contract.Address, new(big.Int), baseline[:])
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(suite.sessions[0].Execution.SendTransaction(ctx, baselineTx)).To(gomega.Succeed())
			baselineReceipt := awaitReceipt(ctx, suite.sessions[0], baselineTx.Hash())
			for _, session := range suite.sessions {
				gomega.Eventually(func() bool {
					stored, err := session.Execution.StorageAt(ctx, contract.Address, common.Hash{}, nil)
					return err == nil && common.BytesToStorageValue64(stored) == baseline
				}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(time.Second).Should(gomega.BeTrue())
			}

			conflictNonce := baselineTx.Nonce() + 1
			leftValue := execfixture.FullWord(0x30)
			rightValue := execfixture.FullWord(0xb0)
			gomega.Expect(suite.apply(ctx)).To(gomega.Succeed())
			leftTx, err := execfixture.SignCall(ctx, suite.sessions[0], conflictNonce, contract.Address, big.NewInt(11), leftValue[:])
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			rightTx, err := execfixture.SignCall(ctx, suite.sessions[2], conflictNonce, contract.Address, big.NewInt(22), rightValue[:])
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(leftTx.Hash()).NotTo(gomega.Equal(rightTx.Hash()))
			gomega.Expect(suite.sessions[0].Execution.SendTransaction(ctx, leftTx)).To(gomega.Succeed())
			gomega.Expect(suite.sessions[2].Execution.SendTransaction(ctx, rightTx)).To(gomega.Succeed())
			leftReceipt := awaitReceipt(ctx, suite.sessions[0], leftTx.Hash())
			rightReceipt := awaitReceipt(ctx, suite.sessions[2], rightTx.Hash())
			gomega.Expect(leftReceipt.BlockHash).NotTo(gomega.Equal(rightReceipt.BlockHash))

			startFinalized, err := suite.beacons[0].FinalizedEpoch(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(suite.partition.Clear(ctx)).To(gomega.Succeed())
			suite.awaitConvergenceAndFinality(ctx, startFinalized)

			var winner *types.Transaction
			var winningValue common.StorageValue64
			gomega.Eventually(func() bool {
				leftCanonical := receiptExists(ctx, suite.sessions[0], leftTx.Hash())
				rightCanonical := receiptExists(ctx, suite.sessions[0], rightTx.Hash())
				if leftCanonical == rightCanonical {
					return false
				}
				if leftCanonical {
					winner, winningValue = leftTx, leftValue
				} else {
					winner, winningValue = rightTx, rightValue
				}
				return true
			}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(time.Second).Should(gomega.BeTrue())

			for _, session := range suite.sessions {
				gomega.Eventually(func() error {
					stored, err := session.Execution.StorageAt(ctx, contract.Address, common.Hash{}, nil)
					if err != nil {
						return err
					}
					if common.BytesToStorageValue64(stored) != winningValue {
						return fmt.Errorf("storage value does not match canonical transaction")
					}
					nonce, err := session.Execution.NonceAt(ctx, suite.sessions[0].Address, nil)
					if err != nil {
						return err
					}
					if nonce != conflictNonce+1 {
						return fmt.Errorf("account nonce is %d, want %d", nonce, conflictNonce+1)
					}
					return nil
				}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(time.Second).Should(gomega.Succeed())
				receipt := awaitReceipt(ctx, session, winner.Hash())
				gomega.Expect(receipt.Logs).To(gomega.HaveLen(1))
				gomega.Expect(receipt.Logs[0].Data).To(gomega.Equal(winningValue[:]))
			}
			gomega.Expect(baselineReceipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
		}, ginkgo.SpecTimeout(partitionTimeout), ginkgo.Label("behavior:partition:execution-state-reorg"))
	},
)

func awaitReceipt(ctx context.Context, session *endtoendlive.Session, hash common.Hash) *types.Receipt {
	ginkgo.GinkgoHelper()
	waitCtx, cancel := context.WithTimeout(ctx, partitionTimeout)
	defer cancel()
	receipt, err := execfixture.WaitReceipt(waitCtx, session.Execution, hash)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return receipt
}

func receiptExists(ctx context.Context, session *endtoendlive.Session, hash common.Hash) bool {
	_, err := session.Execution.TransactionReceipt(ctx, hash)
	return err == nil
}

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
		minimumHead, maximumHead := heads[0].Slot, heads[0].Slot
		var expected consensus.Checkpoint
		for index, beacon := range suite.beacons {
			status, err := beacon.Syncing(ctx)
			if err != nil {
				return err
			}
			if status.Syncing || status.Optimistic || status.ELOffline {
				return fmt.Errorf("participant %d is not ready: %+v", index+1, status)
			}
			checkpoint, err := beacon.FinalizedCheckpoint(ctx)
			if err != nil {
				return err
			}
			if checkpoint.Epoch <= previousFinalized {
				return fmt.Errorf("finalized epoch %d has not advanced past %d", checkpoint.Epoch, previousFinalized)
			}
			if index == 0 {
				expected = checkpoint
			} else if checkpoint != expected {
				return fmt.Errorf("participant checkpoints have not converged: %+v != %+v", checkpoint, expected)
			}
			if heads[index].Slot < minimumHead {
				minimumHead = heads[index].Slot
			}
			if heads[index].Slot > maximumHead {
				maximumHead = heads[index].Slot
			}
		}
		if maximumHead-minimumHead > 1 {
			return fmt.Errorf("participant heads differ by %d slots", maximumHead-minimumHead)
		}
		return nil
	}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(2 * time.Second).Should(gomega.Succeed())
}
