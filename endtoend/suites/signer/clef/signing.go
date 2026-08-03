// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package clef

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	qrlaccounts "github.com/theQRL/go-qrl/accounts"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/rpc"
	signercore "github.com/theQRL/go-qrl/signer/core"
	"github.com/theQRL/go-qrl/signer/core/apitypes"
)

func verifyDataSigning(ctx context.Context, client *rpc.Client, account common.Address, expectedWallet wallet.Wallet) error {
	var signature hexutil.Bytes
	if err := callRPC(ctx, client, &signature, "account_signData",
		qrlaccounts.MimetypeTextPlain,
		account.Hex(),
		hexutil.Encode([]byte(expectedText)),
	); err != nil {
		return err
	}
	return verifySignature("account_signData", signature, qrlaccounts.TextHash([]byte(expectedText)), expectedWallet)
}

func verifyDataRejection(ctx context.Context, client *rpc.Client, account common.Address) error {
	var signature hexutil.Bytes
	err := callRPC(ctx, client, &signature, "account_signData",
		qrlaccounts.MimetypeTextPlain,
		account.Hex(),
		hexutil.Encode([]byte(rejectedText)),
	)
	if err == nil {
		return errors.New("account_signData unexpectedly approved rejected data")
	}
	if !strings.Contains(err.Error(), signercore.ErrRequestDenied.Error()) {
		return fmt.Errorf("account_signData returned unexpected rejection: %w", err)
	}
	return nil
}

func verifyValidatorDataSigning(ctx context.Context, client *rpc.Client, account common.Address, expectedWallet wallet.Wallet) error {
	validator := common.MustParseAddress(expectedRecipient)
	message := []byte(expectedValidatorText)
	var signature hexutil.Bytes
	if err := callRPC(ctx, client, &signature, "account_signData",
		qrlaccounts.MimetypeDataWithValidator,
		account.Hex(),
		map[string]any{
			"address": hexutil.Encode(validator.Bytes()),
			"message": hexutil.Encode(message),
		},
	); err != nil {
		return err
	}
	digest, _ := signercore.SignTextValidator(apitypes.ValidatorData{Address: validator, Message: message})
	return verifySignature("account_signData data/validator", signature, digest, expectedWallet)
}

func verifyTypedDataSigning(ctx context.Context, client *rpc.Client, account common.Address, chainID *big.Int, expectedWallet wallet.Wallet) error {
	signature, digest, err := signTypedData(ctx, client, account, chainID)
	if err != nil {
		return err
	}
	return verifySignature("account_signTypedData", signature, digest, expectedWallet)
}

func verifyTypedDataChainIDRejection(ctx context.Context, client *rpc.Client, account common.Address, chainID *big.Int) error {
	wrongChainID := new(big.Int).Add(chainID, big.NewInt(1))
	var signature hexutil.Bytes
	err := callRPC(ctx, client, &signature, "account_signTypedData", account.Hex(), expectedTypedData(account, wrongChainID))
	if err == nil {
		return errors.New("account_signTypedData unexpectedly approved the wrong chain ID")
	}
	if !strings.Contains(err.Error(), "does not match the configuration of the signer") {
		return fmt.Errorf("account_signTypedData returned unexpected chain-ID error: %w", err)
	}
	return nil
}

func signTypedData(ctx context.Context, client *rpc.Client, account common.Address, chainID *big.Int) (hexutil.Bytes, []byte, error) {
	typedData := expectedTypedData(account, chainID)
	var signature hexutil.Bytes
	if err := callRPC(ctx, client, &signature, "account_signTypedData", account.Hex(), typedData); err != nil {
		return nil, nil, err
	}
	digest, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return nil, nil, fmt.Errorf("hash typed data: %w", err)
	}
	return signature, digest, nil
}
