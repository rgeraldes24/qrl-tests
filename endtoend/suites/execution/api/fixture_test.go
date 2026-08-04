// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package api

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/execfixture"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	qrlwallet "github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/qrlclient"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

type liveSuite struct {
	graphQLURL string
	client     *qrlclient.Client
	wsClient   *qrlclient.Client
	wallet     qrlwallet.Wallet
	from       common.Address
	chainID    *big.Int
	fixture    *liveFixture
}

type liveFixture struct {
	address common.Address
	tx      *types.Transaction
	receipt *types.Receipt
	block   *types.Block
	value   common.StorageValue64
	topic   common.LogTopic
}

func setupLiveSuite(ctx context.Context) *liveSuite {
	ginkgo.GinkgoHelper()

	runtime, err := endtoendlive.Load(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	ginkgo.DeferCleanup(runtime.Close)
	session, err := runtime.PrimaryWithWebSocket(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	suite := &liveSuite{
		graphQLURL: session.Participant.Execution.GraphQLURL,
		client:     session.Execution,
		wsClient:   session.ExecutionWebSocket,
		wallet:     session.Wallet,
		from:       session.Address,
		chainID:    session.ChainID,
	}
	suite.fixture = suite.deployFixture(ctx)
	return suite
}

func (suite *liveSuite) deployFixture(ctx context.Context) *liveFixture {
	ginkgo.GinkgoHelper()

	var value common.StorageValue64
	for index := range value {
		value[index] = byte(index + 1)
	}
	var topic common.LogTopic
	for index := range topic {
		topic[index] = byte(0xff - index)
	}

	tx, err := suite.signTransaction(ctx, nil, new(big.Int), apiContractCode(value, topic))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	receipt := suite.submitAndWait(ctx, tx)
	gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
	gomega.Expect(receipt.ContractAddress).NotTo(gomega.Equal(common.Address{}))
	gomega.Expect(receipt.Logs).To(gomega.HaveLen(1))
	gomega.Expect(receipt.Logs[0].Address).To(gomega.Equal(receipt.ContractAddress))
	gomega.Expect(receipt.Logs[0].Topics).To(gomega.Equal([]common.LogTopic{topic}))
	gomega.Expect(receipt.Logs[0].Data).To(gomega.Equal(value[:]))

	block, err := suite.client.BlockByNumber(ctx, receipt.BlockNumber)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(block).NotTo(gomega.BeNil())

	return &liveFixture{
		address: receipt.ContractAddress,
		tx:      tx,
		receipt: receipt,
		block:   block,
		value:   value,
		topic:   topic,
	}
}

func (suite *liveSuite) signTransaction(
	ctx context.Context,
	to *common.Address,
	value *big.Int,
	data []byte,
) (*types.Transaction, error) {
	return suite.signTransactionWithAccessList(ctx, to, value, data, nil)
}

func (suite *liveSuite) signTransactionWithAccessList(
	ctx context.Context,
	to *common.Address,
	value *big.Int,
	data []byte,
	accessList types.AccessList,
) (*types.Transaction, error) {
	if value == nil {
		value = new(big.Int)
	}
	nonce, err := suite.client.PendingNonceAt(ctx, suite.from)
	if err != nil {
		return nil, fmt.Errorf("read pending nonce: %w", err)
	}
	return suite.signTransactionForWallet(
		ctx,
		suite.wallet,
		suite.from,
		nonce,
		to,
		value,
		data,
		accessList,
	)
}

func (suite *liveSuite) signTransactionForWallet(
	ctx context.Context,
	signerWallet qrlwallet.Wallet,
	from common.Address,
	nonce uint64,
	to *common.Address,
	value *big.Int,
	data []byte,
	accessList types.AccessList,
) (*types.Transaction, error) {
	signer := execfixture.TransactionSigner{
		Client: suite.client, Wallet: signerWallet, From: from, ChainID: suite.chainID,
	}
	return signer.Sign(ctx, execfixture.TransactionRequest{
		Nonce: nonce, To: to, Value: value, Data: data, AccessList: accessList,
	})
}

func (suite *liveSuite) submitAndWait(ctx context.Context, tx *types.Transaction) *types.Receipt {
	ginkgo.GinkgoHelper()
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	receipt, err := execfixture.SendAndWait(waitCtx, suite.client, tx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	assertMinedReceipt(receipt)
	return receipt
}

func (suite *liveSuite) waitReceipt(ctx context.Context, hash common.Hash) *types.Receipt {
	ginkgo.GinkgoHelper()

	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	receipt, err := execfixture.WaitReceipt(waitCtx, suite.client, hash)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	assertMinedReceipt(receipt)
	return receipt
}

func assertMinedReceipt(receipt *types.Receipt) {
	ginkgo.GinkgoHelper()
	gomega.Expect(receipt).NotTo(gomega.BeNil())
	gomega.Expect(receipt.BlockNumber).NotTo(gomega.BeNil())
}
