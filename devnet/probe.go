// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package devnet

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/devnet/internal/rpcjson"
)

const chainAdvancementWindow = 30 * time.Second

func probeNetwork(ctx context.Context, rpcURL, address string) error {
	firstBlock, err := blockNumber(ctx, rpcURL)
	if err != nil {
		return fmt.Errorf("read block number: %w", err)
	}

	advancementCtx, cancel := context.WithTimeout(ctx, chainAdvancementWindow)
	defer cancel()
	if err := retryUntil(advancementCtx, func() error {
		block, err := blockNumber(advancementCtx, rpcURL)
		if err != nil {
			return fmt.Errorf("read advancing block number: %w", err)
		}
		if block <= firstBlock {
			return fmt.Errorf("block number remains at %d", block)
		}
		return nil
	}); err != nil {
		return fmt.Errorf(
			"chain did not advance beyond block %d within %s: %w",
			firstBlock,
			chainAdvancementWindow,
			err,
		)
	}

	var encodedBalance string
	if err := rpcjson.Call(
		ctx,
		rpcURL,
		"qrl_getBalance",
		[]any{address, "latest"},
		&encodedBalance,
	); err != nil {
		return fmt.Errorf("read development wallet balance: %w", err)
	}
	balance, ok := new(big.Int).SetString(strings.TrimPrefix(encodedBalance, "0x"), 16)
	if !ok {
		return fmt.Errorf("invalid development wallet balance %q", encodedBalance)
	}
	if balance.Sign() <= 0 {
		return fmt.Errorf("development wallet %s has no balance", address)
	}

	return nil
}

func blockNumber(ctx context.Context, rpcURL string) (uint64, error) {
	var encoded string
	if err := rpcjson.Call(ctx, rpcURL, "qrl_blockNumber", nil, &encoded); err != nil {
		return 0, err
	}
	block, err := strconv.ParseUint(strings.TrimPrefix(encoded, "0x"), 16, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid qrl_blockNumber %q: %w", encoded, err)
	}
	return block, nil
}
