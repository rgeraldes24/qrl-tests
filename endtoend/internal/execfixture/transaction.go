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
	qrlwallet "github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
)

const receiptPollInterval = time.Second

type TransactionParameters struct {
	FeeCap *big.Int
	TipCap *big.Int
	Gas    uint64
}

type TransactionClient interface {
	SuggestGasPrice(context.Context) (*big.Int, error)
	SuggestGasTipCap(context.Context) (*big.Int, error)
	EstimateGas(context.Context, qrl.CallMsg) (uint64, error)
}

type ReceiptClient interface {
	SendTransaction(context.Context, *types.Transaction) error
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
}

type TransactionRequest struct {
	Nonce      uint64
	To         *common.Address
	Value      *big.Int
	Data       []byte
	AccessList types.AccessList
	Gas        uint64
}

type TransactionSigner struct {
	Client  TransactionClient
	Wallet  qrlwallet.Wallet
	From    common.Address
	ChainID *big.Int
}

func NewTransactionSigner(session *endtoendlive.Session) TransactionSigner {
	return TransactionSigner{
		Client:  session.Execution,
		Wallet:  session.Wallet,
		From:    session.Address,
		ChainID: session.ChainID,
	}
}

func (signer TransactionSigner) Estimate(ctx context.Context, request TransactionRequest) (TransactionParameters, error) {
	feeCap, err := signer.Client.SuggestGasPrice(ctx)
	if err != nil {
		return TransactionParameters{}, err
	}
	tipCap, err := signer.Client.SuggestGasTipCap(ctx)
	if err != nil {
		return TransactionParameters{}, err
	}
	feeCap = new(big.Int).Mul(feeCap, big.NewInt(4))
	if feeCap.Cmp(tipCap) < 0 {
		feeCap.Set(tipCap)
	}
	gas := request.Gas
	if gas == 0 {
		gas, err = signer.Client.EstimateGas(ctx, qrl.CallMsg{
			From:       signer.From,
			To:         request.To,
			Value:      request.value(),
			Data:       request.Data,
			AccessList: request.AccessList,
		})
		if err != nil {
			return TransactionParameters{}, err
		}
		gas += gas / 5
	}
	return TransactionParameters{FeeCap: feeCap, TipCap: tipCap, Gas: gas}, nil
}

func (signer TransactionSigner) Sign(ctx context.Context, request TransactionRequest) (*types.Transaction, error) {
	parameters, err := signer.Estimate(ctx, request)
	if err != nil {
		return nil, err
	}
	return signer.SignWithParameters(request, parameters)
}

func (signer TransactionSigner) SignWithParameters(request TransactionRequest, parameters TransactionParameters) (*types.Transaction, error) {
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:    signer.ChainID,
		Nonce:      request.Nonce,
		GasTipCap:  new(big.Int).Set(parameters.TipCap),
		GasFeeCap:  new(big.Int).Set(parameters.FeeCap),
		Gas:        parameters.Gas,
		To:         request.To,
		Value:      request.value(),
		Data:       request.Data,
		AccessList: request.AccessList,
	})
	return types.SignTx(tx, types.LatestSignerForChainID(signer.ChainID), signer.Wallet)
}

func (request TransactionRequest) value() *big.Int {
	if request.Value == nil {
		return new(big.Int)
	}
	return request.Value
}

func SignCall(
	ctx context.Context,
	session *endtoendlive.Session,
	nonce uint64,
	to common.Address,
	value *big.Int,
	data []byte,
) (*types.Transaction, error) {
	return NewTransactionSigner(session).Sign(ctx, TransactionRequest{
		Nonce: nonce, To: &to, Value: value, Data: data,
	})
}

func EstimateCall(
	ctx context.Context,
	session *endtoendlive.Session,
	to common.Address,
	value *big.Int,
	data []byte,
) (TransactionParameters, error) {
	return NewTransactionSigner(session).Estimate(ctx, TransactionRequest{To: &to, Value: value, Data: data})
}

func SignCallWithParameters(
	session *endtoendlive.Session,
	nonce uint64,
	to common.Address,
	value *big.Int,
	data []byte,
	parameters TransactionParameters,
) (*types.Transaction, error) {
	return NewTransactionSigner(session).SignWithParameters(TransactionRequest{
		Nonce: nonce, To: &to, Value: value, Data: data,
	}, parameters)
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
