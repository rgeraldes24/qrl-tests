// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package execfixture

import (
	"context"
	"errors"
	"math/big"
	"time"

	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
)

const receiptPollInterval = time.Second

type TransactionParameters struct {
	FeeCap *big.Int
	TipCap *big.Int
	Gas    uint64
}

type ReceiptClient interface {
	SendTransaction(context.Context, *types.Transaction) error
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
}

func SignCall(
	ctx context.Context,
	session *endtoendlive.Session,
	nonce uint64,
	to common.Address,
	value *big.Int,
	data []byte,
) (*types.Transaction, error) {
	parameters, err := EstimateCall(ctx, session, to, value, data)
	if err != nil {
		return nil, err
	}
	return SignCallWithParameters(session, nonce, to, value, data, parameters)
}

func EstimateCall(
	ctx context.Context,
	session *endtoendlive.Session,
	to common.Address,
	value *big.Int,
	data []byte,
) (TransactionParameters, error) {
	feeCap, err := session.Execution.SuggestGasPrice(ctx)
	if err != nil {
		return TransactionParameters{}, err
	}
	tipCap, err := session.Execution.SuggestGasTipCap(ctx)
	if err != nil {
		return TransactionParameters{}, err
	}
	feeCap = new(big.Int).Mul(feeCap, big.NewInt(4))
	if feeCap.Cmp(tipCap) < 0 {
		feeCap.Set(tipCap)
	}
	gas, err := session.Execution.EstimateGas(ctx, qrl.CallMsg{
		From: session.Address, To: &to, Value: value, Data: data,
	})
	if err != nil {
		return TransactionParameters{}, err
	}
	return TransactionParameters{FeeCap: feeCap, TipCap: tipCap, Gas: gas + gas/5}, nil
}

func SignCallWithParameters(
	session *endtoendlive.Session,
	nonce uint64,
	to common.Address,
	value *big.Int,
	data []byte,
	parameters TransactionParameters,
) (*types.Transaction, error) {
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID: session.ChainID, Nonce: nonce,
		GasTipCap: new(big.Int).Set(parameters.TipCap),
		GasFeeCap: new(big.Int).Set(parameters.FeeCap),
		Gas:       parameters.Gas,
		To:        &to, Value: value, Data: data,
	})
	return types.SignTx(tx, types.LatestSignerForChainID(session.ChainID), session.Wallet)
}

func SendAndWait(ctx context.Context, client ReceiptClient, tx *types.Transaction) (*types.Receipt, error) {
	if err := client.SendTransaction(ctx, tx); err != nil {
		return nil, err
	}
	return WaitReceipt(ctx, client, tx.Hash())
}

func WaitReceipt(ctx context.Context, client ReceiptClient, hash common.Hash) (*types.Receipt, error) {
	ticker := time.NewTicker(receiptPollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			return receipt, nil
		}
		if err == nil {
			err = errors.New("transaction receipt is nil")
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, errors.Join(lastErr, ctx.Err())
		case <-ticker.C:
		}
	}
}
