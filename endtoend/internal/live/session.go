// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

// Package live opens the shared clients and wallet used by live E2E suites.
package live

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/clients/beacon"
	"github.com/cyyber/qrl-tests/endtoend/internal/runenv"
	"github.com/cyyber/qrl-tests/internal/devwallet"
	"github.com/theQRL/go-qrl/common"
	qrlwallet "github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/go-qrl/qrlclient"
)

// Runtime owns the network metadata and shared resources for one live suite.
type Runtime struct {
	Environment devnet.Environment
	Profile     devnet.Profile
	Wallet      qrlwallet.Wallet
	Address     common.Address
	ChainID     *big.Int
	Services    *devnet.ServiceController

	manager  *devnet.Manager
	sessions []*Session
	tools    runenv.Tools
}

type Session struct {
	*Runtime
	Participant        devnet.Participant
	Execution          *qrlclient.Client
	ExecutionWebSocket *qrlclient.Client
	Consensus          *beacon.Client

	closeOnce sync.Once
}

// Load resolves the configured test environment and restores the disposable
// development wallet once for the suite.
func Load() (*Runtime, error) {
	manifest, err := runenv.Required()
	if err != nil {
		return nil, err
	}
	wallet, err := devwallet.Restore()
	if err != nil {
		return nil, err
	}
	runtime := &Runtime{
		Environment: manifest.Environment,
		Profile:     manifest.Profile,
		Wallet:      wallet,
		Address:     common.Address(wallet.GetAddress()),
		manager:     devnet.NewManager(),
		tools:       manifest.Tools,
	}
	runtime.Services = runtime.manager.ServiceController(manifest.Environment.EnclaveName)
	return runtime, nil
}

func (runtime *Runtime) Primary(ctx context.Context) (*Session, error) {
	participant, err := runtime.Environment.Primary()
	if err != nil {
		return nil, err
	}
	return runtime.open(ctx, participant, false)
}

func (runtime *Runtime) PrimaryWithWebSocket(ctx context.Context) (*Session, error) {
	participant, err := runtime.Environment.Primary()
	if err != nil {
		return nil, err
	}
	return runtime.open(ctx, participant, true)
}

func (runtime *Runtime) OpenAll(ctx context.Context) ([]*Session, error) {
	sessions := make([]*Session, 0, len(runtime.Environment.Participants))
	for _, participant := range runtime.Environment.Participants {
		session, err := runtime.open(ctx, participant, false)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (runtime *Runtime) OpenParticipant(ctx context.Context, index int) (*Session, error) {
	participant, err := runtime.participant(index)
	if err != nil {
		return nil, err
	}
	return runtime.open(ctx, participant, false)
}

func (runtime *Runtime) ConsensusClient(index int) (*beacon.Client, error) {
	participant, err := runtime.participant(index)
	if err != nil {
		return nil, err
	}
	return beacon.New(participant.Consensus.URL)
}

// RefreshEnvironment reloads service endpoints after a service restart.
func (runtime *Runtime) RefreshEnvironment(ctx context.Context) error {
	environment, err := runtime.manager.Inspect(ctx, runtime.Environment.EnclaveName, runtime.Environment.Backend)
	if err != nil {
		return err
	}
	runtime.Environment = environment
	return nil
}

func (runtime *Runtime) GQRL() (string, error) {
	if runtime.tools.GQRL == "" {
		return "", errors.New("gqrl test tool is not configured")
	}
	return runtime.tools.GQRL, nil
}

func (runtime *Runtime) Clef() (string, error) {
	if runtime.tools.Clef == "" {
		return "", errors.New("Clef test tool is not configured")
	}
	return runtime.tools.Clef, nil
}

func (runtime *Runtime) participant(index int) (devnet.Participant, error) {
	for _, participant := range runtime.Environment.Participants {
		if participant.Index == index {
			return participant, nil
		}
	}
	return devnet.Participant{}, fmt.Errorf("participant %d not found", index)
}

func (runtime *Runtime) open(ctx context.Context, participant devnet.Participant, withWebSocket bool) (*Session, error) {
	client, err := qrlclient.DialContext(ctx, participant.Execution.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("open participant %d HTTP RPC: %w", participant.Index, err)
	}
	if runtime.ChainID == nil {
		runtime.ChainID, err = client.ChainID(ctx)
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("read participant %d chain ID: %w", participant.Index, err)
		}
	}
	beaconClient, err := beacon.New(participant.Consensus.URL)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("open participant %d consensus client: %w", participant.Index, err)
	}
	session := &Session{Runtime: runtime, Participant: participant, Execution: client, Consensus: beaconClient}
	if withWebSocket {
		session.ExecutionWebSocket, err = qrlclient.DialContext(ctx, participant.Execution.WebSocketURL)
		if err != nil {
			session.Close()
			return nil, fmt.Errorf("open participant %d WebSocket RPC: %w", participant.Index, err)
		}
	}
	runtime.sessions = append(runtime.sessions, session)
	return session, nil
}

func (runtime *Runtime) Close() {
	for _, session := range runtime.sessions {
		session.Close()
	}
	runtime.sessions = nil
}

func (session *Session) Close() {
	session.closeOnce.Do(func() {
		if session.ExecutionWebSocket != nil {
			session.ExecutionWebSocket.Close()
		}
		if session.Execution != nil {
			session.Execution.Close()
		}
	})
}
