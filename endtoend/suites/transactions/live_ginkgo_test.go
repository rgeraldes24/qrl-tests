//go:build e2e

package transactions

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const transactionTimeout = 3 * time.Minute

var _ = ginkgo.Describe(
	"QRL transaction scenarios",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "transactions", "assertoor", "mutates-chain"),
	func() {
		var sessions []*endtoendlive.Session

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			sessions, err = endtoendlive.OpenAll(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			for _, session := range sessions {
				ginkgo.DeferCleanup(session.Close)
			}
		})

		ginkgo.It("funds a deterministic wallet and verifies its receipt and balance", func(ctx ginkgo.SpecContext) {
			session := sessions[0]
			recipient := patternedAddress(0x61)
			before, err := session.Client.BalanceAt(ctx, recipient, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			amount := big.NewInt(123456789)
			tx := signTransaction(ctx, session, recipient, amount, nil)
			receipt := submitAndWait(ctx, session, tx)
			gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
			gomega.Expect(receipt.TxHash).To(gomega.Equal(tx.Hash()))

			after, err := session.Client.BalanceAt(ctx, recipient, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(after).To(gomega.Equal(new(big.Int).Add(before, amount)))
		}, ginkgo.SpecTimeout(transactionTimeout), ginkgo.Label("assertoor:dev:fund-wallet"))

		ginkgo.It("sustains deterministic large-calldata transactions and remains finalized", func(ctx ginkgo.SpecContext) {
			session := sessions[0]
			beacon, err := consensus.New(session.Participant.ConsensusURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
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
				recipient := patternedAddress(byte(0x71 + transactionIndex))
				transactions[transactionIndex] = signTransaction(ctx, session, recipient, new(big.Int), data)
				gomega.Expect(session.Client.SendTransaction(ctx, transactions[transactionIndex])).To(gomega.Succeed())
			}

			for index, transaction := range transactions {
				receipt := waitForReceipt(ctx, session, transaction)
				gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
				stored, pending, err := session.Client.TransactionByHash(ctx, transaction.Hash())
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
				progress, err := observer.Client.SyncProgress(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(progress).To(gomega.BeNil())
			}
		}, ginkgo.SpecTimeout(10*time.Minute), ginkgo.Label("assertoor:stable:big-calldata-tx-test"))

		ginkgo.It("accepts QRL transactions through every execution client", func(ctx ginkgo.SpecContext) {
			for nodeIndex, session := range sessions {
				transactions := make([]*types.Transaction, 10)
				for index := range transactions {
					recipient := patternedAddress(byte(0x80 + nodeIndex*16 + index))
					transactions[index] = signTransaction(ctx, session, recipient, big.NewInt(int64(index+1)), nil)
					gomega.Expect(session.Client.SendTransaction(ctx, transactions[index])).To(gomega.Succeed())
				}
				for _, transaction := range transactions {
					receipt := waitForReceipt(ctx, session, transaction)
					gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
					for _, observer := range sessions {
						gomega.Eventually(func() error {
							_, pending, err := observer.Client.TransactionByHash(ctx, transaction.Hash())
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
		}, ginkgo.SpecTimeout(10*time.Minute), ginkgo.Label("assertoor:stable:eoa-transactions-test"))
	},
)

func signTransaction(
	ctx context.Context,
	session *endtoendlive.Session,
	to common.Address,
	value *big.Int,
	data []byte,
) *types.Transaction {
	ginkgo.GinkgoHelper()

	nonce, err := session.Client.PendingNonceAt(ctx, session.Address)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	feeCap, err := session.Client.SuggestGasPrice(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	tipCap, err := session.Client.SuggestGasTipCap(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	feeCap = new(big.Int).Mul(feeCap, big.NewInt(4))
	if feeCap.Cmp(tipCap) < 0 {
		feeCap.Set(tipCap)
	}
	gas, err := session.Client.EstimateGas(ctx, qrl.CallMsg{
		From:  session.Address,
		To:    &to,
		Value: value,
		Data:  data,
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   session.ChainID,
		Nonce:     nonce,
		GasTipCap: tipCap,
		GasFeeCap: feeCap,
		Gas:       gas + gas/5,
		To:        &to,
		Value:     value,
		Data:      data,
	})
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(session.ChainID), session.Wallet)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return signed
}

func submitAndWait(ctx context.Context, session *endtoendlive.Session, tx *types.Transaction) *types.Receipt {
	ginkgo.GinkgoHelper()

	gomega.Expect(session.Client.SendTransaction(ctx, tx)).To(gomega.Succeed())
	return waitForReceipt(ctx, session, tx)
}

func waitForReceipt(ctx context.Context, session *endtoendlive.Session, tx *types.Transaction) *types.Receipt {
	ginkgo.GinkgoHelper()

	var receipt *types.Receipt
	gomega.Eventually(func() error {
		var err error
		receipt, err = session.Client.TransactionReceipt(ctx, tx.Hash())
		return err
	}).WithContext(ctx).WithTimeout(transactionTimeout).WithPolling(time.Second).Should(gomega.Succeed())
	gomega.Expect(receipt).NotTo(gomega.BeNil())
	return receipt
}

func patternedAddress(seed byte) common.Address {
	var address common.Address
	for index := range address {
		address[index] = seed + byte(index)
	}
	return address
}
