// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

// Package clef exercises a standalone Clef signer and verifies its QRL
// signatures and signed transactions.
package clef

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/rpc"
)

const (
	readinessTimeout = 30 * time.Second
	requestTimeout   = 15 * time.Second
	pollInterval     = 500 * time.Millisecond
)

type clefSession struct {
	process         *clefProcess
	client          *rpc.Client
	clefPath        string
	workspace       string
	masterPassword  string
	accountPassword string
	account         common.Address
	chainID         *big.Int
	expectedWallet  wallet.Wallet
}

func newClefSession(
	ctx context.Context,
	processContext context.Context,
	clefPath,
	workspace string,
	chainID *big.Int,
	expectedWallet wallet.Wallet,
) (*clefSession, error) {
	if clefPath == "" {
		return nil, errors.New("Clef executable path is required")
	}
	seed, err := expectedWallet.GetSeed()
	if err != nil {
		return nil, fmt.Errorf("read expected wallet seed: %w", err)
	}

	masterPassword, err := randomSecret()
	if err != nil {
		return nil, err
	}
	accountPassword, err := randomSecret()
	if err != nil {
		return nil, err
	}
	account, err := initializeClef(
		ctx,
		clefPath,
		workspace,
		hex.EncodeToString(seed.ToBytes()),
		masterPassword,
		accountPassword,
	)
	if err != nil {
		return nil, err
	}
	if account != common.Address(expectedWallet.GetAddress()) {
		return nil, fmt.Errorf(
			"imported account %s does not match seed address %s",
			account.Hex(),
			common.Address(expectedWallet.GetAddress()).Hex(),
		)
	}

	process, endpoint, err := startClef(
		processContext,
		clefPath,
		workspace,
		masterPassword,
		accountPassword,
		chainID,
	)
	if err != nil {
		return nil, err
	}
	client, err := connectClef(ctx, endpoint, process)
	if err != nil {
		return nil, err
	}
	return &clefSession{
		process:         process,
		client:          client,
		clefPath:        clefPath,
		workspace:       workspace,
		masterPassword:  masterPassword,
		accountPassword: accountPassword,
		account:         account,
		chainID:         new(big.Int).Set(chainID),
		expectedWallet:  expectedWallet,
	}, nil
}

func (session *clefSession) close() error {
	if session.client != nil {
		session.client.Close()
	}
	if session.process != nil {
		return session.process.stop()
	}
	return nil
}

func (session *clefSession) restart(ctx, processContext context.Context) error {
	if err := session.close(); err != nil {
		return fmt.Errorf("stop Clef for restart: %w", err)
	}
	session.client = nil
	session.process = nil

	process, endpoint, err := startClef(
		processContext,
		session.clefPath,
		session.workspace,
		session.masterPassword,
		session.accountPassword,
		session.chainID,
	)
	if err != nil {
		return err
	}
	client, err := connectClef(ctx, endpoint, process)
	if err != nil {
		return err
	}
	session.process = process
	session.client = client
	return nil
}

func connectClef(
	ctx context.Context,
	endpoint string,
	process *clefProcess,
) (*rpc.Client, error) {
	client, err := rpc.DialOptions(
		ctx,
		endpoint,
		rpc.WithHTTPClient(&http.Client{Timeout: requestTimeout}),
	)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("connect to Clef: %w", err), process.stop())
	}
	if err := waitForClef(ctx, client, process); err != nil {
		client.Close()
		return nil, errors.Join(err, process.stop())
	}
	return client, nil
}

func waitForClef(
	ctx context.Context,
	client *rpc.Client,
	process *clefProcess,
) error {
	readyCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()
	var lastErr error
	for {
		var version string
		if err := callRPC(readyCtx, client, &version, "account_version"); err == nil {
			if version == "" {
				return errors.New("account_version returned an empty version")
			}
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-process.process.Done():
			if err := process.process.Err(); err != nil {
				return fmt.Errorf("Clef exited before readiness: %w", err)
			}
			return errors.New("Clef exited before readiness")
		case <-readyCtx.Done():
			return fmt.Errorf("wait for Clef: %w", errors.Join(readyCtx.Err(), lastErr))
		case <-time.After(pollInterval):
		}
	}
}

func callRPC(
	ctx context.Context,
	client *rpc.Client,
	result any,
	method string,
	args ...any,
) error {
	if err := client.CallContext(ctx, result, method, args...); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	return nil
}
