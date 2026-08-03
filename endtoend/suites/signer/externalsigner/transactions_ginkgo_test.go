// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package externalsigner

import (
	"time"

	qrlapi "github.com/cyyber/qrl-tests/endtoend/internal/rpctypes"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	qrvm "github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/crypto"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerSignTransactionSpec() {
	ginkgo.It("signs a transaction through the node", func(ctx ginkgo.SpecContext) {
		args := externalSignerSuite.transactionArgs(ctx)
		var signed qrlapi.SignTransactionResult
		gomega.Expect(externalSignerSuite.session.Execution.Client().CallContext(
			ctx, &signed, "qrl_signTransaction", args,
		)).To(gomega.Succeed())
		gomega.Expect(signed.Tx).NotTo(gomega.BeNil())
		gomega.Expect(signed.Raw).NotTo(gomega.BeEmpty())
		gomega.Expect(transactionSender(signed.Tx, externalSignerSuite.session.ChainID)).To(
			gomega.Equal(externalSignerSuite.account),
		)
		expectTransactionMatchesArgs(signed.Tx, args)

		var decoded types.Transaction
		gomega.Expect(decoded.UnmarshalBinary(signed.Raw)).To(gomega.Succeed())
		gomega.Expect(decoded.Hash()).To(gomega.Equal(signed.Tx.Hash()))
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}

func registerSubmitTransactionSpec() {
	ginkgo.It("signs, submits, and mines a transaction through the node", func(ctx ginkgo.SpecContext) {
		args := externalSignerSuite.transactionArgs(ctx)
		var hash common.Hash
		gomega.Expect(externalSignerSuite.session.Execution.Client().CallContext(
			ctx, &hash, "qrl_sendTransaction", args,
		)).To(gomega.Succeed())
		gomega.Expect(hash).NotTo(gomega.Equal(common.Hash{}))

		var receipt *types.Receipt
		gomega.Eventually(func() error {
			var err error
			receipt, err = externalSignerSuite.session.Execution.TransactionReceipt(ctx, hash)
			return err
		}).WithContext(ctx).WithTimeout(2 * time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
		gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
		gomega.Expect(receipt.TxHash).To(gomega.Equal(hash))

		tx, pending, err := externalSignerSuite.session.Execution.TransactionByHash(ctx, hash)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(pending).To(gomega.BeFalse())
		gomega.Expect(transactionSender(tx, externalSignerSuite.session.ChainID)).To(
			gomega.Equal(externalSignerSuite.account),
		)
		expectTransactionMatchesArgs(tx, args)
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}

func registerContractCreationSpec() {
	ginkgo.It("signs, submits, and mines contract creation through the node", func(ctx ginkgo.SpecContext) {
		args := externalSignerSuite.contractCreationArgs(ctx)
		var signed qrlapi.SignTransactionResult
		gomega.Expect(externalSignerSuite.session.Execution.Client().CallContext(
			ctx, &signed, "qrl_signTransaction", args,
		)).To(gomega.Succeed())
		gomega.Expect(signed.Tx).NotTo(gomega.BeNil())
		gomega.Expect(signed.Tx.To()).To(gomega.BeNil())
		gomega.Expect(transactionSender(signed.Tx, externalSignerSuite.session.ChainID)).To(
			gomega.Equal(externalSignerSuite.account),
		)
		expectTransactionMatchesArgs(signed.Tx, args)

		var hash common.Hash
		gomega.Expect(externalSignerSuite.session.Execution.Client().CallContext(
			ctx, &hash, "qrl_sendTransaction", args,
		)).To(gomega.Succeed())

		var receipt *types.Receipt
		gomega.Eventually(func() error {
			var err error
			receipt, err = externalSignerSuite.session.Execution.TransactionReceipt(ctx, hash)
			return err
		}).WithContext(ctx).WithTimeout(2 * time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
		gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
		gomega.Expect(receipt.ContractAddress).To(
			gomega.Equal(crypto.CreateAddress(externalSignerSuite.account, uint64(*args.Nonce))),
		)
		code, err := externalSignerSuite.session.Execution.CodeAt(ctx, receipt.ContractAddress, receipt.BlockNumber)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(code).To(gomega.Equal([]byte{byte(qrvm.STOP)}))
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}
