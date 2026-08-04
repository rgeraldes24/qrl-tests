//go:build e2e

package transactions

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/behavior"
	"github.com/cyyber/qrl-tests/endtoend/internal/execfixture"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/core/types"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerTransactionScenarios(sessions *[]*endtoendlive.Session) {
	ginkgo.It("funds a deterministic wallet and verifies its receipt and balance", func(ctx ginkgo.SpecContext) {
		session := (*sessions)[0]
		recipient := execfixture.PatternedAddress(0x61)
		before, err := session.Execution.BalanceAt(ctx, recipient, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		amount := big.NewInt(123456789)
		tx := signTransaction(ctx, session, recipient, amount, nil)
		receipt := submitAndWait(ctx, session, tx)
		gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
		gomega.Expect(receipt.TxHash).To(gomega.Equal(tx.Hash()))

		after, err := session.Execution.BalanceAt(ctx, recipient, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(after).To(gomega.Equal(new(big.Int).Add(before, amount)))
	}, ginkgo.SpecTimeout(transactionTimeout), ginkgo.Label(
		"scenario:dev:fund-wallet",
		behavior.Name("transactions:fund-wallet"),
	))

	ginkgo.It("sustains deterministic large-calldata transactions and remains finalized", func(ctx ginkgo.SpecContext) {
		nodes := *sessions
		session := nodes[0]
		beacon := session.Consensus
		startFinalized, err := beacon.FinalizedEpoch(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		transactions := make([]*types.Transaction, 16)
		for transactionIndex := range transactions {
			dataSize := 1024
			if transactionIndex == 0 {
				dataSize = 64 * 1024
			}
			data := make([]byte, dataSize)
			for index := range data {
				data[index] = byte(index + transactionIndex)
			}
			recipient := execfixture.PatternedAddress(byte(0x71 + transactionIndex))
			transactions[transactionIndex] = signTransaction(ctx, session, recipient, new(big.Int), data)
			gomega.Expect(session.Execution.SendTransaction(ctx, transactions[transactionIndex])).To(gomega.Succeed())
		}

		for index, transaction := range transactions {
			receipt := waitForReceipt(ctx, session, transaction)
			gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
			stored, pending, err := session.Execution.TransactionByHash(ctx, transaction.Hash())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pending).To(gomega.BeFalse())
			gomega.Expect(bytes.Equal(stored.Data(), transaction.Data())).To(gomega.BeTrue(), "transaction %d calldata", index)
		}
		gomega.Eventually(func() uint64 {
			finalized, _ := beacon.FinalizedEpoch(ctx)
			return finalized
		}).WithContext(ctx).WithTimeout(10 * time.Minute).WithPolling(time.Second).Should(
			gomega.BeNumerically(">=", startFinalized+2),
		)
		for _, observer := range nodes {
			progress, err := observer.Execution.SyncProgress(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(progress).To(gomega.BeNil())
		}
	}, ginkgo.SpecTimeout(10*time.Minute), ginkgo.Label(
		"scenario:stable:big-calldata-tx-test",
		behavior.Name("transactions:calldata-boundary"),
		behavior.Name("transactions:finality-under-load"),
	))

	ginkgo.It("accepts QRL transactions through every execution client", func(ctx ginkgo.SpecContext) {
		nodes := *sessions
		for nodeIndex, session := range nodes {
			transactions := make([]*types.Transaction, 10)
			for index := range transactions {
				recipient := execfixture.PatternedAddress(byte(0x80 + nodeIndex*16 + index))
				transactions[index] = signTransaction(ctx, session, recipient, big.NewInt(int64(index+1)), nil)
				gomega.Expect(session.Execution.SendTransaction(ctx, transactions[index])).To(gomega.Succeed())
			}
			for _, transaction := range transactions {
				receipt := waitForReceipt(ctx, session, transaction)
				gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
				for _, observer := range nodes {
					gomega.Eventually(func() error {
						_, pending, err := observer.Execution.TransactionByHash(ctx, transaction.Hash())
						if err != nil {
							return err
						}
						if pending {
							return fmt.Errorf("transaction %s is still pending", transaction.Hash())
						}
						return nil
					}).WithContext(ctx).WithTimeout(transactionTimeout).WithPolling(time.Second).Should(gomega.Succeed())
				}
			}
		}
	}, ginkgo.SpecTimeout(10*time.Minute), ginkgo.Label(
		"scenario:stable:eoa-transactions-test",
		behavior.Name("transactions:all-execution-clients"),
		behavior.Name("transactions:network-wide-inclusion"),
	))

	ginkgo.It("propagates a pending transaction between execution peers", func(ctx ginkgo.SpecContext) {
		nodes := *sessions
		if len(nodes) < 2 {
			ginkgo.Skip("transaction propagation requires the multi-participant profile")
		}
		services := nodes[0].Services
		validators := make([]string, 0, len(nodes))
		for _, session := range nodes {
			validators = append(validators, session.Participant.Validator.Name)
		}
		gomega.Expect(services.Stop(ctx, validators...)).To(gomega.Succeed())
		stopped := true
		defer func() {
			if stopped {
				cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				gomega.Expect(services.Start(cleanup, validators...)).To(gomega.Succeed())
			}
		}()

		tx := signTransaction(ctx, nodes[0], execfixture.PatternedAddress(0x5a), big.NewInt(77), []byte("peer-propagation"))
		gomega.Expect(nodes[0].Execution.SendTransaction(ctx, tx)).To(gomega.Succeed())
		for _, observer := range nodes[1:] {
			gomega.Eventually(func() bool {
				stored, pending, err := observer.Execution.TransactionByHash(ctx, tx.Hash())
				return err == nil && pending && stored.Hash() == tx.Hash()
			}).WithContext(ctx).WithTimeout(transactionTimeout).WithPolling(250 * time.Millisecond).Should(gomega.BeTrue())
		}

		gomega.Expect(services.Start(ctx, validators...)).To(gomega.Succeed())
		stopped = false
		receipt := waitForReceipt(ctx, nodes[0], tx)
		for _, observer := range nodes[1:] {
			gomega.Eventually(func() error {
				observed, err := observer.Execution.TransactionReceipt(ctx, tx.Hash())
				if err == nil && observed.BlockHash != receipt.BlockHash {
					return fmt.Errorf("receipt block mismatch: got %s, want %s", observed.BlockHash, receipt.BlockHash)
				}
				return err
			}).WithContext(ctx).WithTimeout(transactionTimeout).WithPolling(time.Second).Should(gomega.Succeed())
		}
	}, ginkgo.SpecTimeout(10*time.Minute), ginkgo.Label(behavior.Name("transactions:p2p-propagation")))
}
