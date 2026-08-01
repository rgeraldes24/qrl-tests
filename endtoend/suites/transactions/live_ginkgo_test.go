//go:build e2e

package transactions

import (
	"bytes"
	"context"
	"math/big"
	"time"

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
		var session *endtoendlive.Session

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			session, err = endtoendlive.Open(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(session.Close)
		})

		ginkgo.It("funds a deterministic wallet and verifies its receipt and balance", func(ctx ginkgo.SpecContext) {
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
		}, ginkgo.SpecTimeout(transactionTimeout))

		ginkgo.It("includes a transaction with 64 KiB of deterministic calldata", func(ctx ginkgo.SpecContext) {
			data := make([]byte, 64*1024)
			for index := range data {
				data[index] = byte(index)
			}
			recipient := patternedAddress(0x71)
			tx := signTransaction(ctx, session, recipient, new(big.Int), data)
			receipt := submitAndWait(ctx, session, tx)
			gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))

			stored, pending, err := session.Client.TransactionByHash(ctx, tx.Hash())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pending).To(gomega.BeFalse())
			gomega.Expect(bytes.Equal(stored.Data(), data)).To(gomega.BeTrue())
		}, ginkgo.SpecTimeout(transactionTimeout))
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
