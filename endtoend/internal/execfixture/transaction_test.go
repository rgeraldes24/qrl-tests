// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package execfixture

import (
	"context"
	"math/big"
	"testing"

	"github.com/cyyber/qrl-tests/internal/devwallet"
	"github.com/stretchr/testify/require"
	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
)

type receiptClient struct {
	sent    *types.Transaction
	receipt *types.Receipt
}

type transactionClient struct {
	receiptClient
	call qrl.CallMsg
}

func (*transactionClient) SuggestGasPrice(context.Context) (*big.Int, error) {
	return big.NewInt(10), nil
}

func (*transactionClient) SuggestGasTipCap(context.Context) (*big.Int, error) {
	return big.NewInt(2), nil
}

func (client *transactionClient) EstimateGas(_ context.Context, call qrl.CallMsg) (uint64, error) {
	client.call = call
	return 100, nil
}

func (client *receiptClient) SendTransaction(_ context.Context, transaction *types.Transaction) error {
	client.sent = transaction
	return nil
}

func (client *receiptClient) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	return client.receipt, nil
}

func TestSendAndWait(t *testing.T) {
	transaction := types.NewTx(&types.DynamicFeeTx{
		ChainID: big.NewInt(1),
		To:      new(common.Address),
		Value:   new(big.Int),
	})
	want := &types.Receipt{TxHash: transaction.Hash(), Status: types.ReceiptStatusSuccessful}
	client := &receiptClient{receipt: want}

	got, err := SendAndWait(context.Background(), client, transaction)
	require.NoError(t, err)
	require.Same(t, transaction, client.sent)
	require.Same(t, want, got)
}

func TestTransactionSigner(t *testing.T) {
	wallet, err := devwallet.Restore()
	require.NoError(t, err)
	client := new(transactionClient)
	signer := TransactionSigner{
		Client: client, Wallet: wallet,
		From: common.Address(wallet.GetAddress()), ChainID: big.NewInt(1337),
	}
	storageKey := common.Hash{1}
	request := TransactionRequest{
		Nonce: 3, Data: []byte{4},
		AccessList: types.AccessList{{Address: common.Address{2}, StorageKeys: []common.Hash{storageKey}}},
	}

	transaction, err := signer.Sign(context.Background(), request)
	require.NoError(t, err)
	require.Nil(t, transaction.To())
	require.Equal(t, uint64(120), transaction.Gas())
	require.Equal(t, request.AccessList, transaction.AccessList())
	require.Nil(t, client.call.To)
	require.Equal(t, request.AccessList, client.call.AccessList)

	sender, err := types.Sender(types.LatestSignerForChainID(signer.ChainID), transaction)
	require.NoError(t, err)
	require.Equal(t, signer.From, sender)
}
