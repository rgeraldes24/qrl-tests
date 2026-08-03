// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package execfixture

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
)

type receiptClient struct {
	sent    *types.Transaction
	receipt *types.Receipt
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
