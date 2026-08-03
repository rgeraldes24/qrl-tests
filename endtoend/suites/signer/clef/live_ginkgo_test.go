// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package clef

import (
	"context"
	"testing"
	"time"

	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	qrl "github.com/theQRL/go-qrl"
	qrlbind "github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	qrvm "github.com/theQRL/go-qrl/core/vm"
	qrllibwallet "github.com/theQRL/go-qrllib/wallet/common"
)

const liveSpecTimeout = 10 * time.Minute

func TestE2E(t *testing.T) {
	testsuite.Run(t, "Clef live E2E suite")
}

var _ = ginkgo.Describe(
	"standalone Clef against a live qrl-package network",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "clef", "mutates-chain"),
	func() {
		var (
			session *clefSession
			network *endtoendlive.Session
		)

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			runtime, loadErr := endtoendlive.Load(ctx)
			gomega.Expect(loadErr).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			network, err = runtime.Primary(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			workDir := ginkgo.GinkgoT().TempDir()
			clefPath, err := runtime.Clef()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			session, err = newClefSession(
				ctx,
				context.WithoutCancel(ctx),
				clefPath,
				workDir,
				network.ChainID,
				network.Wallet,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(func() {
				gomega.Expect(session.close()).To(gomega.Succeed())
			})
		})

		ginkgo.It("lists the imported QRL account", func(ctx ginkgo.SpecContext) {
			gomega.Expect(
				verifyAccountListing(ctx, session.client, session.account),
			).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("reports the external API version", func(ctx ginkgo.SpecContext) {
			gomega.Expect(verifyVersion(ctx, session.client)).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("signs and verifies plain-text data", func(ctx ginkgo.SpecContext) {
			gomega.Expect(
				verifyDataSigning(ctx, session.client, session.account, session.expectedWallet),
			).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("rejects data denied by the ruleset", func(ctx ginkgo.SpecContext) {
			gomega.Expect(
				verifyDataRejection(ctx, session.client, session.account),
			).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("signs and verifies validator-bound data", func(ctx ginkgo.SpecContext) {
			gomega.Expect(
				verifyValidatorDataSigning(
					ctx,
					session.client,
					session.account,
					session.expectedWallet,
				),
			).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("signs and verifies QRL typed data", func(ctx ginkgo.SpecContext) {
			gomega.Expect(
				verifyTypedDataSigning(
					ctx,
					session.client,
					session.account,
					session.chainID,
					session.expectedWallet,
				),
			).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("rejects typed data for a different chain", func(ctx ginkgo.SpecContext) {
			gomega.Expect(
				verifyTypedDataChainIDRejection(
					ctx,
					session.client,
					session.account,
					session.chainID,
				),
			).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("verifies a Clef typed-data signature through the precompile", func(ctx ginkgo.SpecContext) {
			signature, digest, err := signTypedData(
				ctx,
				session.client,
				session.account,
				session.chainID,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			context := qrllibwallet.SigningContext(session.expectedWallet.GetDescriptor())
			input := make([]byte, 0, len(digest)+len(session.expectedWallet.GetPK())+len(signature)+1+len(context))
			input = append(input, digest...)
			input = append(input, session.expectedWallet.GetPK()...)
			input = append(input, signature...)
			input = append(input, byte(len(context)))
			input = append(input, context...)

			address := common.BytesToAddress([]byte{3})
			output, err := network.Execution.CallContract(ctx, qrl.CallMsg{
				From: session.account,
				To:   &address,
				Gas:  500_000,
				Data: input,
			}, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(output).To(gomega.Equal(common.LeftPadBytes([]byte{1}, qrvm.WordBytes)))
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("signs, submits, and confirms a transaction", func(ctx ginkgo.SpecContext) {
			nonce, err := network.Execution.PendingNonceAt(ctx, session.account)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			tip, err := network.Execution.SuggestGasTipCap(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			feeCap, err := network.Execution.SuggestGasPrice(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			request := transactionArgs(session.account, session.chainID, nonce, tip, feeCap)
			signed, err := signTransaction(ctx, session.client, request)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(
				verifyTransaction(signed, request, session.account, session.expectedWallet),
			).To(gomega.Succeed())

			gomega.Expect(network.Execution.SendTransaction(ctx, signed.Tx)).To(gomega.Succeed())
			receipt, err := qrlbind.WaitMined(ctx, network.Execution, signed.Tx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
			gomega.Expect(receipt.TxHash).To(gomega.Equal(signed.Tx.Hash()))
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("rejects a transaction denied by the ruleset", func(ctx ginkgo.SpecContext) {
			nonce, err := network.Execution.PendingNonceAt(ctx, session.account)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			tip, err := network.Execution.SuggestGasTipCap(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			feeCap, err := network.Execution.SuggestGasPrice(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(verifyTransactionRejection(
				ctx,
				session.client,
				transactionArgs(session.account, session.chainID, nonce, tip, feeCap),
			)).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("persists a new password-protected account across restart", func(ctx ginkgo.SpecContext) {
			account, err := verifyNewAccount(ctx, session)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(session.restart(ctx, context.WithoutCancel(ctx))).To(gomega.Succeed())
			gomega.Expect(verifyAccountPresent(ctx, session.client, account)).To(gomega.Succeed())

			nonce, err := network.Execution.PendingNonceAt(ctx, account)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			tip, err := network.Execution.SuggestGasTipCap(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			feeCap, err := network.Execution.SuggestGasPrice(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			signed, err := signTransaction(
				ctx,
				session.client,
				transactionArgs(account, session.chainID, nonce, tip, feeCap),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(verifyTransactionSender(signed, account)).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(liveSpecTimeout))
	},
)
