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
	Environment        devnet.Environment
	Participant        devnet.Participant
	Execution          *qrlclient.Client
	ExecutionWebSocket *qrlclient.Client
	Wallet             qrlwallet.Wallet
	Address            common.Address
	ChainID            *big.Int
}

func Open(ctx context.Context, withWebSocket bool) (*Session, error) {
	environment, err := devnet.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	return open(ctx, environment, environment.Participants[0], withWebSocket)
}

func OpenAll(ctx context.Context, withWebSocket bool) ([]*Session, error) {
	environment, err := devnet.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	sessions := make([]*Session, 0, len(environment.Participants))
	for _, participant := range environment.Participants {
		session, err := open(ctx, environment, participant, withWebSocket)
		if err != nil {
			for _, opened := range sessions {
				opened.Close()
			}
			return nil, fmt.Errorf("open participant %d: %w", participant.Index, err)
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func OpenParticipant(ctx context.Context, index int, withWebSocket bool) (*Session, error) {
	environment, err := devnet.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	for _, participant := range environment.Participants {
		if participant.Index == index {
			return open(ctx, environment, participant, withWebSocket)
		}
	}
	return nil, fmt.Errorf("participant %d not found", index)
}

func open(ctx context.Context, environment devnet.Environment, participant devnet.Participant, withWebSocket bool) (*Session, error) {
	client, err := qrlclient.DialContext(ctx, participant.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial HTTP RPC: %w", err)
	}
	session := &Session{
		Environment: environment,
		Participant: participant,
		Execution:   client,
	}
	if withWebSocket {
		session.ExecutionWebSocket, err = qrlclient.DialContext(ctx, participant.WebSocketURL)
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
	if session.ExecutionWebSocket != nil {
		session.ExecutionWebSocket.Close()
	}
	if session.Execution != nil {
		session.Execution.Close()
	}
}
