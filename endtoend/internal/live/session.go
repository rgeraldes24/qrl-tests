// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

// Package live opens the shared clients and wallet used by live E2E suites.
package live

import (
	"context"
	_ "embed"
	"fmt"
	"math/big"
	"strings"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/theQRL/go-qrl/common"
	qrlwallet "github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/qrlclient"
)

//go:embed testdata/unsafe-development-wallet.seed
var unsafeDevelopmentWalletSeed string

type Session struct {
	Environment     devnet.Environment
	Client          *qrlclient.Client
	WebSocketClient *qrlclient.Client
	Wallet          qrlwallet.Wallet
	Address         common.Address
	ChainID         *big.Int
}

func Open(ctx context.Context, withWebSocket bool) (*Session, error) {
	environment, err := devnet.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	client, err := qrlclient.DialContext(ctx, environment.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial HTTP RPC: %w", err)
	}
	session := &Session{
		Environment: environment,
		Client:      client,
	}
	if withWebSocket {
		session.WebSocketClient, err = qrlclient.DialContext(ctx, environment.WebSocketURL)
		if err != nil {
			session.Close()
			return nil, fmt.Errorf("dial WebSocket RPC: %w", err)
		}
	}
	session.Wallet, err = qrlwallet.RestoreFromSeedHex(strings.TrimSpace(unsafeDevelopmentWalletSeed))
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("restore development wallet: %w", err)
	}
	session.Address = common.Address(session.Wallet.GetAddress())
	session.ChainID, err = client.ChainID(ctx)
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("read chain ID: %w", err)
	}
	return session, nil
}

func (session *Session) Close() {
	if session.WebSocketClient != nil {
		session.WebSocketClient.Close()
	}
	if session.Client != nil {
		session.Client.Close()
	}
}
