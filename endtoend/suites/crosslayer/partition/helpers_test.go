//go:build e2e

package partition

import (
	"context"
	"fmt"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/cyyber/qrl-tests/endtoend/internal/execfixture"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
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
