// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package externalsigner

import (
	"context"
	"fmt"
	"math/big"
	"time"

	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	qrlapi "github.com/cyyber/qrl-tests/endtoend/internal/rpctypes"
	"github.com/cyyber/qrl-tests/internal/fixture"
	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/core/types"
	qrvm "github.com/theQRL/go-qrl/core/vm"
	qrlwallet "github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func newLiveSuite(ctx context.Context) *liveSuite {
	ginkgo.GinkgoHelper()

	runtime, err := endtoendlive.Load(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	session, err := runtime.Primary(ctx, false)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	wallet, err := qrlwallet.RestoreFromSeedHex(fixture.RemoteSignerSeed)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	account := common.Address(wallet.GetAddress())

	return &liveSuite{session: session, wallet: wallet, account: account}
}

func (suite *liveSuite) transactionArgs(ctx context.Context) qrlapi.TransactionArgs {
	ginkgo.GinkgoHelper()

	nonce, err := suite.session.Execution.PendingNonceAt(ctx, suite.account)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	feeCap, err := suite.session.Execution.SuggestGasPrice(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	tipCap, err := suite.session.Execution.SuggestGasTipCap(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	input := make(hexutil.Bytes, 65)
	for index := range input {
		input[index] = byte(index + 1)
	}
	accessList := types.AccessList{{
		Address:     suite.session.Address,
		StorageKeys: []common.Hash{{0x01}},
	}}
	gas, err := suite.session.Execution.EstimateGas(ctx, qrl.CallMsg{
		From:       suite.account,
		To:         &suite.session.Address,
		Data:       input,
		AccessList: accessList,
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	from := suite.account
	to := suite.session.Address
	gasLimit := hexutil.Uint64(gas)
	nonceValue := hexutil.Uint64(nonce)
	return qrlapi.TransactionArgs{
		From:                 &from,
		To:                   &to,
		Gas:                  &gasLimit,
		MaxFeePerGas:         (*hexutil.Big)(feeCap),
		MaxPriorityFeePerGas: (*hexutil.Big)(tipCap),
		Value:                (*hexutil.Big)(big.NewInt(1)),
		Nonce:                &nonceValue,
		Input:                &input,
		AccessList:           &accessList,
		ChainID:              (*hexutil.Big)(suite.session.ChainID),
	}
}

func (suite *liveSuite) contractCreationArgs(ctx context.Context) qrlapi.TransactionArgs {
	ginkgo.GinkgoHelper()

	args := suite.transactionArgs(ctx)
	input := hexutil.Bytes{
		byte(qrvm.PUSH1), 1,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	}
	value := (*hexutil.Big)(new(big.Int))
	accessList := types.AccessList{}
	gas, err := suite.session.Execution.EstimateGas(ctx, qrl.CallMsg{
		From:       suite.account,
		Value:      value.ToInt(),
		Data:       input,
		AccessList: accessList,
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gasLimit := hexutil.Uint64(gas)

	args.To = nil
	args.Gas = &gasLimit
	args.Value = value
	args.Input = &input
	args.AccessList = &accessList
	return args
}

func expectTransactionMatchesArgs(tx *types.Transaction, args qrlapi.TransactionArgs) {
	ginkgo.GinkgoHelper()

	if args.To == nil {
		gomega.Expect(tx.To()).To(gomega.BeNil())
	} else {
		gomega.Expect(tx.To()).NotTo(gomega.BeNil())
		gomega.Expect(*tx.To()).To(gomega.Equal(*args.To))
	}
	gomega.Expect(tx.Nonce()).To(gomega.Equal(uint64(*args.Nonce)))
	gomega.Expect(tx.Gas()).To(gomega.Equal(uint64(*args.Gas)))
	gomega.Expect(tx.GasFeeCap()).To(gomega.Equal(args.MaxFeePerGas.ToInt()))
	gomega.Expect(tx.GasTipCap()).To(gomega.Equal(args.MaxPriorityFeePerGas.ToInt()))
	gomega.Expect(tx.Value()).To(gomega.Equal(args.Value.ToInt()))
	gomega.Expect(tx.ChainId()).To(gomega.Equal(args.ChainID.ToInt()))
	gomega.Expect(args.Input).NotTo(gomega.BeNil())
	gomega.Expect(tx.Data()).To(gomega.Equal([]byte(*args.Input)))
	gomega.Expect(args.AccessList).NotTo(gomega.BeNil())
	gomega.Expect(tx.AccessList()).To(gomega.Equal(*args.AccessList))
}

func expectNoTransactionAtNonce(ctx context.Context, suite *liveSuite, nonce uint64) {
	ginkgo.GinkgoHelper()

	gomega.Consistently(func() uint64 {
		pending, err := suite.session.Execution.PendingNonceAt(ctx, suite.account)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		return pending
	}).WithContext(ctx).WithTimeout(5 * time.Second).WithPolling(time.Second).Should(
		gomega.Equal(nonce),
	)

	var content map[string]map[string]*qrlapi.RPCTransaction
	gomega.Expect(suite.session.Execution.Client().CallContext(
		ctx,
		&content,
		"txpool_contentFrom",
		suite.account,
	)).To(gomega.Succeed())
	gomega.Expect(content["pending"]).NotTo(gomega.HaveKey(fmt.Sprint(nonce)))
	gomega.Expect(content["queued"]).NotTo(gomega.HaveKey(fmt.Sprint(nonce)))
}

func transactionSender(tx *types.Transaction, chainID *big.Int) common.Address {
	ginkgo.GinkgoHelper()

	from, err := types.Sender(types.LatestSignerForChainID(chainID), tx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return from
}
