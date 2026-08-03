// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package clef

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/rpc"
	signercore "github.com/theQRL/go-qrl/signer/core"
	"github.com/theQRL/go-qrl/signer/core/apitypes"
)

type signTransactionResult struct {
	Raw hexutil.Bytes      `json:"raw"`
	Tx  *types.Transaction `json:"tx"`
}

func signTransaction(ctx context.Context, client *rpc.Client, transaction apitypes.SendTxArgs) (signTransactionResult, error) {
	var signed signTransactionResult
	if err := callRPC(ctx, client, &signed, "account_signTransaction", transaction); err != nil {
		return signTransactionResult{}, err
	}
	return signed, nil
}

func verifyTransactionRejection(ctx context.Context, client *rpc.Client, transaction apitypes.SendTxArgs) error {
	transaction.Value = hexutil.Big(*big.NewInt(rejectedValue))
	_, err := signTransaction(ctx, client, transaction)
	if err == nil {
		return errors.New("account_signTransaction unexpectedly approved the rejected transaction")
	}
	if !strings.Contains(err.Error(), signercore.ErrRequestDenied.Error()) {
		return fmt.Errorf("account_signTransaction returned unexpected rejection: %w", err)
	}
	return nil
}
