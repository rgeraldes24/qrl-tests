// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

// Package devnet controls separately managed QRL development networks.
package devnet

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/cyyber/qrl-tests/devnet/internal/kurtosis"
)

type kurtosisClient interface {
	EnclaveExists(context.Context, string) (bool, error)
	CreateAndRunRemotePackage(context.Context, string, string, string) error
	Service(context.Context, string, string) (kurtosis.Service, error)
	DestroyEnclave(context.Context, string) error
}

const (
	DefaultEnclaveName  = "go-qrl-devnet"
	DefaultStartTimeout = 30 * time.Minute
	// DevelopmentWalletAddress is funded only by the built-in disposable profile.
	DevelopmentWalletAddress = "QBb81a0496aa34a64f96c2bCd28793165e1e6C08af0605b119cc768764901d2E4B48b5b9c049C57469CcA8a0421D2E31DF5C637a9cee8f3DA83964261B6CF9a22"

	destroyConfirmationTimeout = 15 * time.Second
)

type Environment struct {
	RPCURL       string
	GraphQLURL   string
	WebSocketURL string
	ConsensusURL string
}

type StartOptions struct {
	EnclaveName    string
	ExecutionImage string
	Parameters     []byte
}

type Manager struct {
	newClient func() (kurtosisClient, error)
	probe     func(context.Context, string, string) error
}

func NewManager() *Manager {
	return &Manager{
		newClient: func() (kurtosisClient, error) {
			client, err := kurtosis.NewSDKClient()
			if err != nil {
				return nil, fmt.Errorf("connect to Kurtosis engine: %w", err)
			}
			return client, nil
		},
		probe: probeNetwork,
	}
}

func Inspect(ctx context.Context) (Environment, error) {
	return NewManager().Inspect(ctx, cmp.Or(os.Getenv("DEVNET_ENCLAVE_NAME"), DefaultEnclaveName))
}

func (manager *Manager) Start(ctx context.Context, options StartOptions) error {
	parameters, err := effectiveParameters(DevelopmentWalletAddress, options.ExecutionImage, options.Parameters)
	if err != nil {
		return fmt.Errorf("prepare qrl-package parameters: %w", err)
	}
	client, err := manager.newClient()
	if err != nil {
		return err
	}
	if found, err := client.EnclaveExists(ctx, options.EnclaveName); err != nil {
		return err
	} else if found {
		return errors.New("network already exists or provisioning is incomplete; stop it before retrying")
	}
	if err := client.CreateAndRunRemotePackage(
		ctx,
		options.EnclaveName,
		packageLocator,
		parameters,
	); err != nil {
		return fmt.Errorf("create enclave or run pinned qrl-package; enclave may remain until stopped: %w", err)
	}

	// Endpoints are fixed once the package run completes; only the probe has to
	// wait for the chain to come up.
	environment, err := resolveEnvironment(ctx, client, options.EnclaveName)
	if err != nil {
		return fmt.Errorf("resolve network endpoints; enclave remains until stopped: %w", err)
	}
	if err := retryUntil(ctx, func() error {
		return manager.probe(ctx, environment.RPCURL, DevelopmentWalletAddress)
	}); err != nil {
		return fmt.Errorf("wait for network readiness; enclave remains until stopped: %w", err)
	}
	return nil
}

func (manager *Manager) Inspect(ctx context.Context, name string) (Environment, error) {
	client, err := manager.newClient()
	if err != nil {
		return Environment{}, err
	}
	found, err := client.EnclaveExists(ctx, name)
	if err != nil {
		return Environment{}, err
	}
	if !found {
		return Environment{}, errors.New("network is not running")
	}
	environment, err := resolveEnvironment(ctx, client, name)
	if err != nil {
		return Environment{}, err
	}
	if err := manager.probe(ctx, environment.RPCURL, DevelopmentWalletAddress); err != nil {
		return Environment{}, err
	}
	return environment, nil
}

func (manager *Manager) Stop(ctx context.Context, name string) error {
	client, err := manager.newClient()
	if err != nil {
		return err
	}
	found, err := client.EnclaveExists(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	destroyErr := client.DestroyEnclave(ctx, name)
	// Confirm the deterministic slot is actually free — on a fresh context so
	// cancellation cannot fake a successful stop — because the next start
	// trusts this result.
	confirmCtx, cancel := context.WithTimeout(context.Background(), destroyConfirmationTimeout)
	defer cancel()
	if found, err := client.EnclaveExists(confirmCtx, name); err != nil {
		return errors.Join(destroyErr, fmt.Errorf("confirm enclave destruction: %w", err))
	} else if found {
		return errors.Join(destroyErr, errors.New("enclave still occupies its slot"))
	}
	return nil
}

func resolveEnvironment(ctx context.Context, client kurtosisClient, name string) (Environment, error) {
	execution, err := client.Service(ctx, name, executionServiceName)
	if err != nil {
		return Environment{}, err
	}
	rpcURL, err := execution.PublicEndpoint(rpcPortID, "http")
	if err != nil {
		return Environment{}, fmt.Errorf("execution service %q: %w", executionServiceName, err)
	}
	webSocketURL, err := execution.PublicEndpoint(webSocketPortID, "ws")
	if err != nil {
		return Environment{}, fmt.Errorf("execution service %q: %w", executionServiceName, err)
	}
	consensus, err := client.Service(ctx, name, consensusServiceName)
	if err != nil {
		return Environment{}, err
	}
	consensusURL, err := consensus.PublicEndpoint(consensusHTTPPortID, "http")
	if err != nil {
		return Environment{}, fmt.Errorf("consensus service %q: %w", consensusServiceName, err)
	}
	return Environment{
		RPCURL:       rpcURL,
		GraphQLURL:   rpcURL + graphQLPath,
		WebSocketURL: webSocketURL,
		ConsensusURL: consensusURL,
	}, nil
}

func retryUntil(ctx context.Context, operation func() error) error {
	policy := backoff.NewExponentialBackOff()
	policy.InitialInterval = 500 * time.Millisecond
	policy.MaxInterval = 2 * time.Second
	_, err := backoff.Retry(
		ctx,
		func() (struct{}, error) { return struct{}{}, operation() },
		backoff.WithBackOff(policy),
		backoff.WithMaxElapsedTime(0),
	)
	return err
}
