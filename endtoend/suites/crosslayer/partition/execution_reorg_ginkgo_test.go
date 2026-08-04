//go:build e2e

package partition

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/execfixture"
	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerExecutionReorgScenario(suite *liveSuite) {
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

		var winner, loser *types.Transaction
		var winningValue common.StorageValue64
		gomega.Eventually(func() bool {
			leftCanonical := receiptExists(ctx, suite.sessions[0], leftTx.Hash())
			rightCanonical := receiptExists(ctx, suite.sessions[0], rightTx.Hash())
			if leftCanonical == rightCanonical {
				return false
			}
			if leftCanonical {
				winner, loser, winningValue = leftTx, rightTx, leftValue
			} else {
				winner, loser, winningValue = rightTx, leftTx, rightValue
			}
			return true
		}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(time.Second).Should(gomega.BeTrue())

		var canonicalReceipt *types.Receipt
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
			if canonicalReceipt == nil {
				canonicalReceipt = receipt
			} else {
				gomega.Expect(receipt.BlockHash).To(gomega.Equal(canonicalReceipt.BlockHash))
				gomega.Expect(receipt.BlockNumber).To(gomega.Equal(canonicalReceipt.BlockNumber))
				gomega.Expect(receipt.TransactionIndex).To(gomega.Equal(canonicalReceipt.TransactionIndex))
			}
			gomega.Expect(receipt.Logs).To(gomega.HaveLen(1))
			gomega.Expect(receipt.Logs[0].Data).To(gomega.Equal(winningValue[:]))
			gomega.Eventually(func() bool {
				_, receiptErr := session.Execution.TransactionReceipt(ctx, loser.Hash())
				return errors.Is(receiptErr, qrl.NotFound)
			}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(time.Second).Should(gomega.BeTrue())
		}
		gomega.Expect(baselineReceipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
	}, ginkgo.SpecTimeout(partitionTimeout), ginkgo.Label("behavior:partition:execution-state-reorg"))
}
