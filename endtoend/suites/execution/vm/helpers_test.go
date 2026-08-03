// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

//go:build e2e

package vm

import (
	"context"
	"math/big"

	qrlapi "github.com/cyyber/qrl-tests/endtoend/internal/rpctypes"
	"github.com/theQRL/go-qrl/accounts/abi"
	"github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/core/types"
	qrvm "github.com/theQRL/go-qrl/core/vm"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func (suite *liveSuite) callCode(
	ctx context.Context,
	code []byte,
	extra qrlapi.StateOverride,
) []byte {
	return suite.callCodeWithInput(ctx, code, nil, extra)
}

func (suite *liveSuite) callCodeWithInput(
	ctx context.Context,
	code []byte,
	input []byte,
	extra qrlapi.StateOverride,
) []byte {
	ginkgo.GinkgoHelper()
	output, err := suite.callCodeResult(ctx, code, input, extra)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return output
}

func (suite *liveSuite) callCodeResult(
	ctx context.Context,
	code []byte,
	input []byte,
	extra qrlapi.StateOverride,
) ([]byte, error) {
	return suite.callCodeResultAt(ctx, code, input, extra, "latest", nil)
}

func (suite *liveSuite) callCodeAt(
	ctx context.Context,
	code []byte,
	extra qrlapi.StateOverride,
	block any,
	fields map[string]any,
) []byte {
	ginkgo.GinkgoHelper()
	output, err := suite.callCodeResultAt(ctx, code, nil, extra, block, fields)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return output
}

func (suite *liveSuite) callCodeResultAt(
	ctx context.Context,
	code []byte,
	input []byte,
	extra qrlapi.StateOverride,
	block any,
	fields map[string]any,
) ([]byte, error) {
	ginkgo.GinkgoHelper()

	overrides := qrlapi.StateOverride{suite.target: codeOverride(code)}
	for address, account := range extra {
		overrides[address] = account
	}
	call := map[string]any{
		"from":  suite.session.Address,
		"to":    suite.target,
		"gas":   hexutil.Uint64(10_000_000),
		"input": hexutil.Bytes(input),
	}
	for name, value := range fields {
		call[name] = value
	}
	var output hexutil.Bytes
	err := suite.session.Execution.Client().CallContext(
		ctx,
		&output,
		"qrl_call",
		call,
		block,
		overrides,
	)
	return output, err
}

func codeOverride(code []byte) qrlapi.OverrideAccount {
	encoded := hexutil.Bytes(code)
	nonce := hexutil.Uint64(0)
	return qrlapi.OverrideAccount{Nonce: &nonce, Code: &encoded}
}

func codeAndBalanceOverride(code []byte, balance *big.Int) qrlapi.OverrideAccount {
	override := codeOverride(code)
	rpcBalance := (*hexutil.Big)(balance)
	override.Balance = &rpcBalance
	return override
}

func vmWord(value *big.Int) []byte {
	return value.FillBytes(make([]byte, qrvm.WordBytes))
}

func vmUnsigned(value, modulus *big.Int) *big.Int {
	return new(big.Int).Mod(new(big.Int).Set(value), modulus)
}

func (suite *liveSuite) mineLog(ctx context.Context, data []byte, topics []common.LogTopic) *types.Receipt {
	ginkgo.GinkgoHelper()

	auth, err := bind.NewKeyedTransactorWithChainID(suite.session.Wallet, suite.session.ChainID)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	auth.Context = ctx
	auth.NoSend = true
	_, tx, _, err := bind.DeployContract(
		auth,
		abi.ABI{},
		logInitCode(data, topics),
		suite.session.Execution,
	)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(suite.session.Execution.SendTransaction(ctx, tx)).To(gomega.Succeed())
	receipt, err := bind.WaitMined(ctx, suite.session.Execution, tx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
	return receipt
}

func (suite *liveSuite) deployRuntime(ctx context.Context, runtime []byte) common.Address {
	ginkgo.GinkgoHelper()

	auth, err := bind.NewKeyedTransactorWithChainID(suite.session.Wallet, suite.session.ChainID)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	auth.Context = ctx
	auth.NoSend = true
	_, tx, _, err := bind.DeployContract(
		auth,
		abi.ABI{},
		runtimeInitCode(runtime),
		suite.session.Execution,
	)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(suite.session.Execution.SendTransaction(ctx, tx)).To(gomega.Succeed())
	receipt, err := bind.WaitMined(ctx, suite.session.Execution, tx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
	gomega.Expect(receipt.ContractAddress).NotTo(gomega.Equal(common.Address{}))
	return receipt.ContractAddress
}

func (suite *liveSuite) mineCall(ctx context.Context, target common.Address) *types.Receipt {
	ginkgo.GinkgoHelper()

	nonce, err := suite.session.Execution.PendingNonceAt(ctx, suite.session.Address)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	feeCap, err := suite.session.Execution.SuggestGasPrice(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	tipCap, err := suite.session.Execution.SuggestGasTipCap(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	feeCap = new(big.Int).Mul(feeCap, big.NewInt(4))
	if feeCap.Cmp(tipCap) < 0 {
		feeCap.Set(tipCap)
	}
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   suite.session.ChainID,
		Nonce:     nonce,
		GasTipCap: tipCap,
		GasFeeCap: feeCap,
		Gas:       1_000_000,
		To:        &target,
	})
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(suite.session.ChainID), suite.session.Wallet)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(suite.session.Execution.SendTransaction(ctx, signed)).To(gomega.Succeed())
	receipt, err := bind.WaitMined(ctx, suite.session.Execution, signed)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return receipt
}
