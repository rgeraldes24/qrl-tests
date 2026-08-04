// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package partition

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/behavior"
	"github.com/cyyber/qrl-tests/endtoend/internal/execfixture"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	qrlwallet "github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/params"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerTransactionReinjectionScenario(suite *liveSuite) {
	ginkgo.It("reinjects transactions from the orphaned execution branch", func(ctx ginkgo.SpecContext) {
		wallets := make([]qrlwallet.Wallet, 2)
		addresses := make([]common.Address, len(wallets))
		funding := new(big.Int).Mul(big.NewInt(10), big.NewInt(params.Quanta))
		for index := range wallets {
			var err error
			wallets[index], err = qrlwallet.Generate(qrlwallet.ML_DSA_87)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			addresses[index] = common.Address(wallets[index].GetAddress())
			nonce, err := suite.sessions[0].Execution.PendingNonceAt(ctx, suite.sessions[0].Address)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			fundingTx, err := execfixture.SignCall(
				ctx,
				suite.sessions[0],
				nonce,
				addresses[index],
				funding,
				nil,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(suite.sessions[0].Execution.SendTransaction(ctx, fundingTx)).To(gomega.Succeed())
			gomega.Expect(awaitReceipt(ctx, suite.sessions[0], fundingTx.Hash()).Status).To(
				gomega.Equal(types.ReceiptStatusSuccessful),
			)
		}
		for _, session := range suite.sessions {
			for _, address := range addresses {
				gomega.Eventually(func() *big.Int {
					balance, err := session.Execution.BalanceAt(ctx, address, nil)
					if err != nil {
						return nil
					}
					return balance
				}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(time.Second).Should(
					gomega.Equal(funding),
				)
			}
		}

		startFinalized, err := suite.beacons[0].FinalizedEpoch(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(suite.apply(ctx)).To(gomega.Succeed())

		recipients := []common.Address{
			execfixture.PatternedAddress(0xe1),
			execfixture.PatternedAddress(0xe2),
		}
		amounts := []*big.Int{big.NewInt(101), big.NewInt(202)}
		branchSessions := []*endtoendlive.Session{suite.sessions[0], suite.sessions[2]}
		transactions := make([]*types.Transaction, len(wallets))
		originalReceipts := make([]*types.Receipt, len(wallets))
		for index := range wallets {
			transactions[index] = signWalletTransfer(
				ctx,
				branchSessions[index],
				wallets[index],
				recipients[index],
				amounts[index],
			)
			gomega.Expect(branchSessions[index].Execution.SendTransaction(ctx, transactions[index])).To(gomega.Succeed())
			originalReceipts[index] = awaitReceipt(ctx, branchSessions[index], transactions[index].Hash())
			gomega.Expect(originalReceipts[index].Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
		}
		gomega.Expect(originalReceipts[0].BlockHash).NotTo(gomega.Equal(originalReceipts[1].BlockHash))

		gomega.Expect(suite.partition.Clear(ctx)).To(gomega.Succeed())
		suite.awaitConvergenceAndFinality(ctx, startFinalized)

		canonicalHashes := make([]common.Hash, len(transactions))
		gomega.Eventually(func() error {
			for txIndex, transaction := range transactions {
				var expected common.Hash
				for sessionIndex, session := range suite.sessions {
					receipt, receiptErr := session.Execution.TransactionReceipt(ctx, transaction.Hash())
					if receiptErr != nil {
						return receiptErr
					}
					if sessionIndex == 0 {
						expected = receipt.BlockHash
						canonicalHashes[txIndex] = receipt.BlockHash
					} else if receipt.BlockHash != expected {
						return fmt.Errorf("transaction %s receipt block mismatch", transaction.Hash())
					}
				}
			}
			return nil
		}).WithContext(ctx).WithTimeout(partitionTimeout).WithPolling(time.Second).Should(gomega.Succeed())

		reinjected := false
		for index := range transactions {
			if canonicalHashes[index] != originalReceipts[index].BlockHash {
				reinjected = true
			}
			for _, session := range suite.sessions {
				balance, err := session.Execution.BalanceAt(ctx, recipients[index], nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(balance).To(gomega.Equal(amounts[index]))
			}
		}
		gomega.Expect(reinjected).To(gomega.BeTrue())
	}, ginkgo.SpecTimeout(partitionTimeout), ginkgo.Label(behavior.Name("partition:transaction-reinjection")))
}

func signWalletTransfer(
	ctx context.Context,
	session *endtoendlive.Session,
	wallet qrlwallet.Wallet,
	recipient common.Address,
	value *big.Int,
) *types.Transaction {
	ginkgo.GinkgoHelper()
	signer := execfixture.TransactionSigner{
		Client: session.Execution, Wallet: wallet,
		From: common.Address(wallet.GetAddress()), ChainID: session.ChainID,
	}
	signed, err := signer.Sign(ctx, execfixture.TransactionRequest{
		To: &recipient, Value: value, Gas: params.TxGas,
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return signed
}
