//go:build e2e

package transactions

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/cyyber/qrl-tests/endtoend/internal/execfixture"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/cyyber/qrl-tests/endtoend/internal/stability"
	"github.com/theQRL/go-qrl/core/types"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const transactionTimeout = 3 * time.Minute

const (
	fullCalldataTransactionCount = 1000
	fullTransactionsPerBlock     = 10
	fullCalldataSize             = 1000
	fullWorkloadTimeout          = 45 * time.Minute
)

var _ = ginkgo.Describe(
	"QRL transaction scenarios",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "transactions", "scenario", "mutates-chain"),
	func() {
		var sessions []*endtoendlive.Session

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			runtime, loadErr := endtoendlive.Load(ctx)
			gomega.Expect(loadErr).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			sessions, err = runtime.OpenAll(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("funds a deterministic wallet and verifies its receipt and balance", func(ctx ginkgo.SpecContext) {
			session := sessions[0]
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
			"behavior:transactions:fund-wallet",
		))

		ginkgo.It("sustains deterministic large-calldata transactions and remains finalized", func(ctx ginkgo.SpecContext) {
			session := sessions[0]
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
			for _, observer := range sessions {
				progress, err := observer.Execution.SyncProgress(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(progress).To(gomega.BeNil())
			}
		}, ginkgo.SpecTimeout(10*time.Minute), ginkgo.Label(
			"scenario:stable:big-calldata-tx-test",
			"behavior:transactions:calldata-boundary",
			"behavior:transactions:finality-under-load",
		))

		ginkgo.It("accepts QRL transactions through every execution client", func(ctx ginkgo.SpecContext) {
			for nodeIndex, session := range sessions {
				transactions := make([]*types.Transaction, 10)
				for index := range transactions {
					recipient := execfixture.PatternedAddress(byte(0x80 + nodeIndex*16 + index))
					transactions[index] = signTransaction(ctx, session, recipient, big.NewInt(int64(index+1)), nil)
					gomega.Expect(session.Execution.SendTransaction(ctx, transactions[index])).To(gomega.Succeed())
				}
				for _, transaction := range transactions {
					receipt := waitForReceipt(ctx, session, transaction)
					gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
					for _, observer := range sessions {
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
			"behavior:transactions:all-execution-clients",
			"behavior:transactions:network-wide-inclusion",
		))

		ginkgo.It("propagates a pending transaction between execution peers", func(ctx ginkgo.SpecContext) {
			if len(sessions) < 2 {
				ginkgo.Skip("transaction propagation requires the multi-participant profile")
			}
			services := sessions[0].Services
			validators := make([]string, 0, len(sessions))
			for _, session := range sessions {
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

			tx := signTransaction(ctx, sessions[0], execfixture.PatternedAddress(0x5a), big.NewInt(77), []byte("peer-propagation"))
			gomega.Expect(sessions[0].Execution.SendTransaction(ctx, tx)).To(gomega.Succeed())
			for _, observer := range sessions[1:] {
				gomega.Eventually(func() bool {
					stored, pending, err := observer.Execution.TransactionByHash(ctx, tx.Hash())
					return err == nil && pending && stored.Hash() == tx.Hash()
				}).WithContext(ctx).WithTimeout(transactionTimeout).WithPolling(250 * time.Millisecond).Should(gomega.BeTrue())
			}

			gomega.Expect(services.Start(ctx, validators...)).To(gomega.Succeed())
			stopped = false
			receipt := waitForReceipt(ctx, sessions[0], tx)
			for _, observer := range sessions[1:] {
				gomega.Eventually(func() error {
					observed, err := observer.Execution.TransactionReceipt(ctx, tx.Hash())
					if err == nil && observed.BlockHash != receipt.BlockHash {
						return fmt.Errorf("receipt block mismatch: got %s, want %s", observed.BlockHash, receipt.BlockHash)
					}
					return err
				}).WithContext(ctx).WithTimeout(transactionTimeout).WithPolling(time.Second).Should(gomega.Succeed())
			}
		}, ginkgo.SpecTimeout(10*time.Minute), ginkgo.Label("behavior:transactions:p2p-propagation"))

		ginkgo.It("runs the complete 1000-transaction calldata workload", func(ctx ginkgo.SpecContext) {
			session := sessions[0]
			beacon := session.Consensus
			startFinalized, err := beacon.FinalizedEpoch(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			data := make([]byte, fullCalldataSize)
			recipient := execfixture.PatternedAddress(0xa1)
			parameters := loadTransactionParameters(ctx, session, recipient, new(big.Int), data)
			nonce, err := session.Execution.PendingNonceAt(ctx, session.Address)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			includedByBlock := make(map[uint64]int)

			for batch := 0; batch < fullCalldataTransactionCount/fullTransactionsPerBlock; batch++ {
				transactions := make([]*types.Transaction, fullTransactionsPerBlock)
				for index := range transactions {
					sequence := batch*fullTransactionsPerBlock + index
					to := execfixture.PatternedAddress(byte(0xa1 + sequence))
					transactions[index] = signTransactionAt(
						session,
						nonce+uint64(sequence),
						to,
						new(big.Int),
						data,
						parameters,
					)
					gomega.Expect(session.Execution.SendTransaction(ctx, transactions[index])).To(gomega.Succeed())
				}
				for _, transaction := range transactions {
					receipt := waitForReceipt(ctx, session, transaction)
					gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
					gomega.Expect(receipt.BlockNumber).NotTo(gomega.BeNil())
					includedByBlock[receipt.BlockNumber.Uint64()]++
				}
			}

			total := 0
			for block, count := range includedByBlock {
				gomega.Expect(count).To(
					gomega.BeNumerically("<=", fullTransactionsPerBlock),
					fmt.Sprintf("execution block %d exceeded the workload limit", block),
				)
				total += count
			}
			gomega.Expect(total).To(gomega.Equal(fullCalldataTransactionCount))
			gomega.Expect(stability.Await(ctx, sessions, startFinalized, 2)).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(fullWorkloadTimeout), ginkgo.Label(
			"scenario-full",
			"scenario:stable:big-calldata-tx-test",
			"behavior:transactions:big-calldata-1000",
			"behavior:network:post-workload-stability",
		))

		ginkgo.It("sustains ten transactions per block through every client and proposer", func(ctx ginkgo.SpecContext) {
			beacon := sessions[0].Consensus
			startFinalized, err := beacon.FinalizedEpoch(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			expectedProposers := make(map[string]struct{}, len(sessions))
			for _, session := range sessions {
				name := strings.TrimPrefix(session.Participant.Validator.Name, "vc-")
				gomega.Expect(name).NotTo(gomega.BeEmpty())
				expectedProposers[name] = struct{}{}
			}
			observedProposers := make(map[string]struct{}, len(expectedProposers))
			usedClients := make(map[int]struct{}, len(sessions))
			lastSlot, err := beacon.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			for batch := 0; batch < 64 && len(observedProposers) < len(expectedProposers); batch++ {
				session := sessions[batch%len(sessions)]
				usedClients[session.Participant.Index] = struct{}{}
				nonce, err := session.Execution.PendingNonceAt(ctx, session.Address)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				to := execfixture.PatternedAddress(byte(0xc0 + batch))
				parameters := loadTransactionParameters(ctx, session, to, big.NewInt(1), nil)
				transactions := make([]*types.Transaction, fullTransactionsPerBlock)
				blocks := make(map[uint64]int)

				for index := range transactions {
					transactions[index] = signTransactionAt(
						session,
						nonce+uint64(index),
						execfixture.PatternedAddress(byte(0xd0+batch+index)),
						big.NewInt(int64(index+1)),
						nil,
						parameters,
					)
					gomega.Expect(session.Execution.SendTransaction(ctx, transactions[index])).To(gomega.Succeed())
				}
				for _, transaction := range transactions {
					receipt := waitForReceipt(ctx, session, transaction)
					gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
					blocks[receipt.BlockNumber.Uint64()]++
				}

				currentSlot, err := beacon.HeadSlot(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				for slot := lastSlot + 1; slot <= currentSlot; slot++ {
					payload, err := beacon.BlockExecutionPayload(ctx, strconv.FormatUint(slot, 10))
					if consensus.IsNotFound(err) {
						continue
					}
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					if blocks[payload.BlockNumber] != fullTransactionsPerBlock {
						continue
					}
					graffiti, err := beacon.BlockGraffitiText(ctx, strconv.FormatUint(slot, 10))
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					observedProposers[graffiti] = struct{}{}
				}
				lastSlot = currentSlot
			}

			gomega.Expect(usedClients).To(gomega.HaveLen(len(sessions)))
			for proposer := range expectedProposers {
				gomega.Expect(observedProposers).To(gomega.HaveKey(proposer))
			}
			gomega.Expect(stability.Await(ctx, sessions, startFinalized, 2)).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(fullWorkloadTimeout), ginkgo.Label(
			"scenario-full",
			"scenario:stable:eoa-transactions-test",
			"behavior:transactions:sustained-10-per-block",
			"behavior:transactions:proposer-inclusion-matrix",
			"behavior:network:post-workload-stability",
		))
	},
)
