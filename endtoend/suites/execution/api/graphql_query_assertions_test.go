// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package api

import (
	"context"
	"encoding/json"
	"math/big"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/consensus/misc/eip1559"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/go-qrl/rlp"
	"github.com/theQRL/go-qrl/rpc"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

type graphQLCallResult struct {
	Data    string `json:"data"`
	GasUsed string `json:"gasUsed"`
	Status  string `json:"status"`
}

type graphQLAccountState struct {
	Address          string `json:"address"`
	Balance          string `json:"balance"`
	TransactionCount string `json:"transactionCount"`
	Code             string `json:"code"`
	Storage          string `json:"storage"`
}

type graphQLWithdrawal struct {
	Index     string `json:"index"`
	Validator string `json:"validator"`
	Address   string `json:"address"`
	Amount    string `json:"amount"`
}

type graphQLQueryBlock struct {
	Number           string               `json:"number"`
	Hash             string               `json:"hash"`
	Parent           graphQLBlock         `json:"parent"`
	TransactionsRoot string               `json:"transactionsRoot"`
	TransactionCount string               `json:"transactionCount"`
	StateRoot        string               `json:"stateRoot"`
	ReceiptsRoot     string               `json:"receiptsRoot"`
	Miner            graphQLAccountState  `json:"miner"`
	ExtraData        string               `json:"extraData"`
	GasLimit         string               `json:"gasLimit"`
	GasUsed          string               `json:"gasUsed"`
	BaseFeePerGas    string               `json:"baseFeePerGas"`
	NextBaseFee      string               `json:"nextBaseFeePerGas"`
	Timestamp        string               `json:"timestamp"`
	LogsBloom        string               `json:"logsBloom"`
	Random           string               `json:"random"`
	Transactions     []graphQLTransaction `json:"transactions"`
	TransactionAt    graphQLTransaction   `json:"transactionAt"`
	Logs             []graphQLLog         `json:"logs"`
	Withdrawals      []graphQLWithdrawal  `json:"withdrawals"`
	Account          graphQLAccountState  `json:"account"`
	Call             graphQLCallResult    `json:"call"`
	EstimateGas      string               `json:"estimateGas"`
	RawHeader        string               `json:"rawHeader"`
	Raw              string               `json:"raw"`
	WithdrawalsRoot  *string              `json:"withdrawalsRoot"`
}

type graphQLPendingBlock struct {
	TransactionCount string               `json:"transactionCount"`
	Transactions     []graphQLTransaction `json:"transactions"`
	Account          graphQLAccountState  `json:"account"`
	Call             graphQLCallResult    `json:"call"`
	EstimateGas      string               `json:"estimateGas"`
}

type graphQLQueryResponse struct {
	Block       graphQLQueryBlock   `json:"block"`
	BlockByHash graphQLBlock        `json:"blockByHash"`
	Blocks      []graphQLBlock      `json:"blocks"`
	Pending     graphQLPendingBlock `json:"pending"`
	Transaction graphQLTransaction  `json:"transaction"`
	Logs        []graphQLLog        `json:"logs"`
	GasPrice    string              `json:"gasPrice"`
	PriorityFee string              `json:"maxPriorityFeePerGas"`
	Syncing     *json.RawMessage    `json:"syncing"`
	ChainID     string              `json:"chainID"`
}

type graphQLQueryExpectation struct {
	header          *types.Header
	rawHeader       []byte
	rawBlock        []byte
	contractBalance *big.Int
	contractNonce   uint64
	contractCode    []byte
	minerBalance    *big.Int
	minerNonce      uint64
	minerCode       []byte
	minerStorage    []byte
	pendingBalance  *big.Int
	pendingNonce    uint64
	pendingCode     []byte
	nextBaseFee     *big.Int
	blockEstimate   hexutil.Uint64
	pendingEstimate hexutil.Uint64
}

func (suite *liveSuite) graphQLQueryExpected(ctx context.Context) graphQLQueryExpectation {
	ginkgo.GinkgoHelper()

	fixture := suite.fixture
	header := fixture.block.Header()
	rawHeader, err := rlp.EncodeToBytes(header)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	rawBlock, err := rlp.EncodeToBytes(fixture.block)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	contractBalance, err := suite.client.BalanceAt(ctx, fixture.address, fixture.receipt.BlockNumber)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	contractNonce, err := suite.client.NonceAt(ctx, fixture.address, fixture.receipt.BlockNumber)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	contractCode, err := suite.client.CodeAt(ctx, fixture.address, fixture.receipt.BlockNumber)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	minerBalance, err := suite.client.BalanceAt(ctx, header.Coinbase, fixture.receipt.BlockNumber)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	minerNonce, err := suite.client.NonceAt(ctx, header.Coinbase, fixture.receipt.BlockNumber)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	minerCode, err := suite.client.CodeAt(ctx, header.Coinbase, fixture.receipt.BlockNumber)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	minerStorage, err := suite.client.StorageAt(ctx, header.Coinbase, common.Hash{}, fixture.receipt.BlockNumber)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	pendingBalance, err := suite.client.PendingBalanceAt(ctx, suite.from)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	pendingNonce, err := suite.client.PendingNonceAt(ctx, suite.from)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	pendingCode, err := suite.client.PendingCodeAt(ctx, suite.from)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	call := map[string]any{"from": suite.from, "to": fixture.address, "data": "0x"}
	var blockEstimate hexutil.Uint64
	gomega.Expect(suite.client.Client().CallContext(
		ctx,
		&blockEstimate,
		"qrl_estimateGas",
		call,
		rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(fixture.block.NumberU64())),
	)).To(gomega.Succeed())
	var pendingEstimate hexutil.Uint64
	gomega.Expect(suite.client.Client().CallContext(
		ctx,
		&pendingEstimate,
		"qrl_estimateGas",
		call,
		rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber),
	)).To(gomega.Succeed())

	return graphQLQueryExpectation{
		header:          header,
		rawHeader:       rawHeader,
		rawBlock:        rawBlock,
		contractBalance: contractBalance,
		contractNonce:   contractNonce,
		contractCode:    contractCode,
		minerBalance:    minerBalance,
		minerNonce:      minerNonce,
		minerCode:       minerCode,
		minerStorage:    minerStorage,
		pendingBalance:  pendingBalance,
		pendingNonce:    pendingNonce,
		pendingCode:     pendingCode,
		nextBaseFee:     eip1559.CalcBaseFee(params.AllBeaconProtocolChanges, header),
		blockEstimate:   blockEstimate,
		pendingEstimate: pendingEstimate,
	}
}

func assertGraphQLBlock(got graphQLQueryBlock, expected graphQLQueryExpectation, fixture *liveFixture) {
	ginkgo.GinkgoHelper()

	header := expected.header
	block := hexutil.EncodeBig(fixture.receipt.BlockNumber)
	gomega.Expect(got.Number).To(gomega.Equal(block))
	gomega.Expect(got.Hash).To(gomega.Equal(fixture.block.Hash().Hex()))
	gomega.Expect(got.Parent.Number).To(gomega.Equal(hexutil.EncodeUint64(fixture.block.NumberU64() - 1)))
	gomega.Expect(got.Parent.Hash).To(gomega.Equal(header.ParentHash.Hex()))
	gomega.Expect(got.TransactionsRoot).To(gomega.Equal(header.TxHash.Hex()))
	gomega.Expect(got.TransactionCount).To(gomega.Equal(hexutil.EncodeUint64(uint64(len(fixture.block.Transactions())))))
	gomega.Expect(got.StateRoot).To(gomega.Equal(header.Root.Hex()))
	gomega.Expect(got.ReceiptsRoot).To(gomega.Equal(header.ReceiptHash.Hex()))
	gomega.Expect(got.Miner.Address).To(gomega.Equal(header.Coinbase.Hex()))
	gomega.Expect(got.Miner.Balance).To(gomega.Equal(hexutil.EncodeBig(expected.minerBalance)), "GraphQL block miner balance")
	gomega.Expect(got.Miner.TransactionCount).To(gomega.Equal(hexutil.EncodeUint64(expected.minerNonce)))
	gomega.Expect(got.Miner.Code).To(gomega.Equal(hexutil.Encode(expected.minerCode)))
	gomega.Expect(got.Miner.Storage).To(gomega.Equal(common.BytesToStorageValue64(expected.minerStorage).Hex()))
	gomega.Expect(got.ExtraData).To(gomega.Equal(hexutil.Encode(header.Extra)))
	gomega.Expect(got.GasLimit).To(gomega.Equal(hexutil.EncodeUint64(header.GasLimit)))
	gomega.Expect(got.GasUsed).To(gomega.Equal(hexutil.EncodeUint64(header.GasUsed)))
	gomega.Expect(got.BaseFeePerGas).To(gomega.Equal(hexutil.EncodeBig(header.BaseFee)))
	gomega.Expect(got.NextBaseFee).To(gomega.Equal(hexutil.EncodeBig(expected.nextBaseFee)), "GraphQL next base fee")
	gomega.Expect(got.Timestamp).To(gomega.Equal(hexutil.EncodeUint64(header.Time)))
	gomega.Expect(got.LogsBloom).To(gomega.Equal(hexutil.Encode(header.Bloom.Bytes())))
	gomega.Expect(got.Random).To(gomega.Equal(header.Random.Hex()))
	gomega.Expect(got.TransactionAt.Hash).To(gomega.Equal(fixture.tx.Hash().Hex()))
	gomega.Expect(got.RawHeader).To(gomega.Equal(hexutil.Encode(expected.rawHeader)))
	gomega.Expect(got.Raw).To(gomega.Equal(hexutil.Encode(expected.rawBlock)))
	gomega.Expect(got.Account.Address).To(gomega.Equal(fixture.address.Hex()))
	gomega.Expect(got.Account.Balance).To(gomega.Equal(hexutil.EncodeBig(expected.contractBalance)))
	gomega.Expect(got.Account.TransactionCount).To(gomega.Equal(hexutil.EncodeUint64(expected.contractNonce)))
	gomega.Expect(got.Account.Code).To(gomega.Equal(hexutil.Encode(expected.contractCode)))
	gomega.Expect(got.Account.Storage).To(gomega.Equal(fixture.value.Hex()), "GraphQL account storage")
}

func assertGraphQLSelections(root graphQLQueryResponse, expected graphQLQueryExpectation, suite *liveSuite) {
	ginkgo.GinkgoHelper()

	fixture := suite.fixture
	block := hexutil.EncodeBig(fixture.receipt.BlockNumber)
	gomega.Expect(root.BlockByHash).To(gomega.Equal(graphQLBlock{Number: block, Hash: fixture.block.Hash().Hex()}))
	gomega.Expect(root.Blocks).To(gomega.Equal([]graphQLBlock{{Number: block, Hash: fixture.block.Hash().Hex()}}))
	gomega.Expect(root.Pending.TransactionCount).To(gomega.Equal(hexutil.EncodeUint64(uint64(len(root.Pending.Transactions)))))
	gomega.Expect(root.Pending.Account.Address).To(gomega.Equal(suite.from.Hex()))
	gomega.Expect(root.Pending.Account.Balance).To(gomega.Equal(hexutil.EncodeBig(expected.pendingBalance)))
	gomega.Expect(root.Pending.Account.TransactionCount).To(gomega.Equal(hexutil.EncodeUint64(expected.pendingNonce)))
	gomega.Expect(root.Pending.Account.Code).To(gomega.Equal(hexutil.Encode(expected.pendingCode)))
	for _, call := range []graphQLCallResult{root.Block.Call, root.Pending.Call} {
		gomega.Expect(call.Data).To(gomega.Equal(hexutil.Encode(fixture.value[:])))
		gomega.Expect(call.Status).To(gomega.Equal("0x1"))
		gasUsed, err := hexutil.DecodeUint64(call.GasUsed)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(gasUsed).To(gomega.BeNumerically(">", 0))
	}
	gomega.Expect(root.Block.EstimateGas).To(gomega.Equal(expected.blockEstimate.String()))
	gomega.Expect(root.Pending.EstimateGas).To(gomega.Equal(expected.pendingEstimate.String()))
	gomega.Expect(root.GasPrice).NotTo(gomega.BeEmpty())
	gomega.Expect(root.PriorityFee).NotTo(gomega.BeEmpty())
	gomega.Expect(root.ChainID).To(gomega.Equal(hexutil.EncodeBig(suite.chainID)))
	gomega.Expect(root.Syncing).To(gomega.BeNil())
}

func assertGraphQLWithdrawals(got graphQLQueryBlock, fixture *liveFixture) {
	ginkgo.GinkgoHelper()

	header := fixture.block.Header()
	if header.WithdrawalsHash == nil {
		gomega.Expect(got.WithdrawalsRoot).To(gomega.BeNil())
		gomega.Expect(got.Withdrawals).To(gomega.BeNil())
		return
	}
	gomega.Expect(got.WithdrawalsRoot).NotTo(gomega.BeNil())
	gomega.Expect(*got.WithdrawalsRoot).To(gomega.Equal(header.WithdrawalsHash.Hex()))
	gomega.Expect(got.Withdrawals).To(gomega.HaveLen(len(fixture.block.Withdrawals())))
	for index, want := range fixture.block.Withdrawals() {
		item := got.Withdrawals[index]
		gomega.Expect(item.Index).To(gomega.Equal(hexutil.EncodeUint64(want.Index)))
		gomega.Expect(item.Validator).To(gomega.Equal(hexutil.EncodeUint64(want.Validator)))
		gomega.Expect(item.Address).To(gomega.Equal(want.Address.Hex()))
		gomega.Expect(item.Amount).To(gomega.Equal(hexutil.EncodeUint64(want.Amount)))
	}
}

func assertGraphQLFixtureResults(root graphQLQueryResponse, suite *liveSuite) {
	ginkgo.GinkgoHelper()

	fixture := suite.fixture
	var blockTransaction *graphQLTransaction
	for index := range root.Block.Transactions {
		if root.Block.Transactions[index].Hash == fixture.tx.Hash().Hex() {
			blockTransaction = &root.Block.Transactions[index]
			break
		}
	}
	gomega.Expect(blockTransaction).NotTo(gomega.BeNil())
	assertGraphQLTransaction(*blockTransaction, fixture.tx, fixture.receipt, fixture.block, suite.chainID)
	assertGraphQLTransactionEnvelope(root.Transaction, fixture.tx, fixture.receipt)
	gomega.Expect(root.Block.Logs).To(gomega.HaveLen(1))
	assertGraphQLLog(root.Block.Logs[0], fixture.receipt.Logs[0])
	gomega.Expect(root.Logs).To(gomega.HaveLen(1))
	assertGraphQLLog(root.Logs[0], fixture.receipt.Logs[0])
}
