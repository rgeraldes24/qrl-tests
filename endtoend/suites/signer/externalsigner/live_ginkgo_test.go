// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

//go:build e2e

package externalsigner

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/fixture"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	qrlapi "github.com/cyyber/qrl-tests/endtoend/internal/rpctypes"
	"github.com/theQRL/go-qrl/accounts"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/core/types"
	qrvm "github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/crypto"
	"github.com/theQRL/go-qrl/crypto/pqcrypto"
	qrlwallet "github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	liveSpecTimeout = 5 * time.Minute
)

type liveSuite struct {
	session *endtoendlive.Session
	wallet  qrlwallet.Wallet
	account common.Address
}

var _ = ginkgo.Describe(
	"go-qrl configured with Clef",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "external-signer", "mutates-chain"),
	func() {
		var suite *liveSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			suite = newLiveSuite(ctx)
			gomega.Expect(suite).NotTo(gomega.BeNil())
			ginkgo.DeferCleanup(suite.session.Close)

			var accounts []common.Address
			err := suite.session.Execution.Client().CallContext(ctx, &accounts, "qrl_accounts")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(accounts).To(gomega.ContainElement(suite.account))
		})

		ginkgo.It("discovers the node-managed Clef account", func(ctx ginkgo.SpecContext) {
			var managed []common.Address
			gomega.Expect(suite.session.Execution.Client().CallContext(
				ctx,
				&managed,
				"qrl_accounts",
			)).To(gomega.Succeed())
			gomega.Expect(managed).To(gomega.Equal([]common.Address{suite.account}))
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("signs text through the node", func(ctx ginkgo.SpecContext) {
			message := []byte("go-qrl external signer E2E")
			var signature hexutil.Bytes
			gomega.Expect(suite.session.Execution.Client().CallContext(
				ctx,
				&signature,
				"qrl_sign",
				suite.account,
				hexutil.Bytes(message),
			)).To(gomega.Succeed())

			valid, err := pqcrypto.MLDSA87VerifySignature(
				signature,
				accounts.TextHash(message),
				suite.wallet.GetPK(),
				suite.wallet.GetDescriptor(),
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(valid).To(gomega.BeTrue())
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("propagates Clef signing rejection through the node", func(ctx ginkgo.SpecContext) {
			var signature hexutil.Bytes
			err := suite.session.Execution.Client().CallContext(
				ctx,
				&signature,
				"qrl_sign",
				suite.account,
				hexutil.Bytes(fixture.RemoteSignerRejectedText),
			)
			gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("request denied")))
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("signs a transaction through the node", func(ctx ginkgo.SpecContext) {
			args := suite.transactionArgs(ctx)
			var signed qrlapi.SignTransactionResult
			gomega.Expect(suite.session.Execution.Client().CallContext(
				ctx,
				&signed,
				"qrl_signTransaction",
				args,
			)).To(gomega.Succeed())
			gomega.Expect(signed.Tx).NotTo(gomega.BeNil())
			gomega.Expect(signed.Raw).NotTo(gomega.BeEmpty())
			gomega.Expect(transactionSender(signed.Tx, suite.session.ChainID)).To(gomega.Equal(suite.account))
			expectTransactionMatchesArgs(signed.Tx, args)

			var decoded types.Transaction
			gomega.Expect(decoded.UnmarshalBinary(signed.Raw)).To(gomega.Succeed())
			gomega.Expect(decoded.Hash()).To(gomega.Equal(signed.Tx.Hash()))
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("propagates Clef transaction rejection through the node", func(ctx ginkgo.SpecContext) {
			before, err := suite.session.Execution.PendingNonceAt(ctx, suite.account)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			args := suite.transactionArgs(ctx)
			args.Value = (*hexutil.Big)(big.NewInt(fixture.RemoteSignerRejectedTransaction))

			var signed qrlapi.SignTransactionResult
			err = suite.session.Execution.Client().CallContext(
				ctx,
				&signed,
				"qrl_signTransaction",
				args,
			)
			gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("request denied")))

			var hash common.Hash
			err = suite.session.Execution.Client().CallContext(
				ctx,
				&hash,
				"qrl_sendTransaction",
				args,
			)
			gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("request denied")))

			expectNoTransactionAtNonce(ctx, suite, before)
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("signs, submits, and mines a transaction through the node", func(ctx ginkgo.SpecContext) {
			args := suite.transactionArgs(ctx)
			var hash common.Hash
			gomega.Expect(suite.session.Execution.Client().CallContext(
				ctx,
				&hash,
				"qrl_sendTransaction",
				args,
			)).To(gomega.Succeed())
			gomega.Expect(hash).NotTo(gomega.Equal(common.Hash{}))

			var receipt *types.Receipt
			gomega.Eventually(func() error {
				var err error
				receipt, err = suite.session.Execution.TransactionReceipt(ctx, hash)
				return err
			}).WithContext(ctx).WithTimeout(2 * time.Minute).WithPolling(time.Second).Should(
				gomega.Succeed(),
			)
			gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
			gomega.Expect(receipt.TxHash).To(gomega.Equal(hash))

			tx, pending, err := suite.session.Execution.TransactionByHash(ctx, hash)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(pending).To(gomega.BeFalse())
			gomega.Expect(transactionSender(tx, suite.session.ChainID)).To(gomega.Equal(suite.account))
			expectTransactionMatchesArgs(tx, args)
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("signs, submits, and mines contract creation through the node", func(ctx ginkgo.SpecContext) {
			args := suite.contractCreationArgs(ctx)
			var signed qrlapi.SignTransactionResult
			gomega.Expect(suite.session.Execution.Client().CallContext(
				ctx,
				&signed,
				"qrl_signTransaction",
				args,
			)).To(gomega.Succeed())
			gomega.Expect(signed.Tx).NotTo(gomega.BeNil())
			gomega.Expect(signed.Tx.To()).To(gomega.BeNil())
			gomega.Expect(transactionSender(signed.Tx, suite.session.ChainID)).To(gomega.Equal(suite.account))
			expectTransactionMatchesArgs(signed.Tx, args)

			var hash common.Hash
			gomega.Expect(suite.session.Execution.Client().CallContext(
				ctx,
				&hash,
				"qrl_sendTransaction",
				args,
			)).To(gomega.Succeed())

			var receipt *types.Receipt
			gomega.Eventually(func() error {
				var err error
				receipt, err = suite.session.Execution.TransactionReceipt(ctx, hash)
				return err
			}).WithContext(ctx).WithTimeout(2 * time.Minute).WithPolling(time.Second).Should(
				gomega.Succeed(),
			)
			gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
			gomega.Expect(receipt.ContractAddress).To(
				gomega.Equal(crypto.CreateAddress(suite.account, uint64(*args.Nonce))),
			)
			code, err := suite.session.Execution.CodeAt(ctx, receipt.ContractAddress, receipt.BlockNumber)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(code).To(gomega.Equal([]byte{byte(qrvm.STOP)}))
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("does not submit a transaction after signing is canceled", func(ctx ginkgo.SpecContext) {
			before, err := suite.session.Execution.PendingNonceAt(ctx, suite.account)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			args := suite.transactionArgs(ctx)
			args.Value = (*hexutil.Big)(big.NewInt(fixture.RemoteSignerDelayedTransaction))
			requestCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			var hash common.Hash
			err = suite.session.Execution.Client().CallContext(
				requestCtx,
				&hash,
				"qrl_sendTransaction",
				args,
			)
			gomega.Expect(err).To(gomega.MatchError(context.DeadlineExceeded))

			expectNoTransactionAtNonce(ctx, suite, before)
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("fails while Clef is unavailable and recovers after restart", func(ctx ginkgo.SpecContext) {
			gomega.Expect(clefService(ctx, "stop")).To(gomega.Succeed())
			stopped := true
			defer func() {
				if stopped {
					_ = clefService(context.Background(), "start")
				}
			}()

			requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			var signature hexutil.Bytes
			err := suite.session.Execution.Client().CallContext(
				requestCtx,
				&signature,
				"qrl_sign",
				suite.account,
				hexutil.Bytes("Clef unavailable"),
			)
			gomega.Expect(err).To(gomega.HaveOccurred())

			gomega.Expect(clefService(ctx, "start")).To(gomega.Succeed())
			stopped = false
			gomega.Eventually(func() error {
				var managed []common.Address
				if err := suite.session.Execution.Client().CallContext(ctx, &managed, "qrl_accounts"); err != nil {
					return err
				}
				if len(managed) != 1 || managed[0] != suite.account {
					return fmt.Errorf("unexpected managed accounts after restart: %v", managed)
				}
				return nil
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("reconnects to Clef after the signer restarts", func(ctx ginkgo.SpecContext) {
			gomega.Expect(restartClef(ctx)).To(gomega.Succeed())

			message := []byte("go-qrl external signer restart E2E")
			gomega.Eventually(func() error {
				var managed []common.Address
				if err := suite.session.Execution.Client().CallContext(ctx, &managed, "qrl_accounts"); err != nil {
					return err
				}
				if len(managed) != 1 || managed[0] != suite.account {
					return fmt.Errorf("unexpected managed accounts after restart: %v", managed)
				}

				var signature hexutil.Bytes
				if err := suite.session.Execution.Client().CallContext(
					ctx,
					&signature,
					"qrl_sign",
					suite.account,
					hexutil.Bytes(message),
				); err != nil {
					return err
				}
				valid, err := pqcrypto.MLDSA87VerifySignature(
					signature,
					accounts.TextHash(message),
					suite.wallet.GetPK(),
					suite.wallet.GetDescriptor(),
				)
				if err != nil {
					return err
				}
				if !valid {
					return fmt.Errorf("invalid signature after Clef restart")
				}
				return nil
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(time.Second).Should(gomega.Succeed())

			args := suite.transactionArgs(ctx)
			var signed qrlapi.SignTransactionResult
			gomega.Eventually(func() error {
				return suite.session.Execution.Client().CallContext(
					ctx,
					&signed,
					"qrl_signTransaction",
					args,
				)
			}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
			gomega.Expect(signed.Tx).NotTo(gomega.BeNil())
			gomega.Expect(transactionSender(signed.Tx, suite.session.ChainID)).To(
				gomega.Equal(suite.account),
			)
			expectTransactionMatchesArgs(signed.Tx, args)
		}, ginkgo.SpecTimeout(liveSpecTimeout))
	},
)
