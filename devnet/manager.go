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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/cyyber/qrl-tests/devnet/internal/kurtosis"
)

type kurtosisClient interface {
	EnclaveExists(context.Context, string) (bool, error)
	CreateAndRunRemotePackage(context.Context, string, string, string) error
	Service(context.Context, string, string) (kurtosis.Service, error)
	Services(context.Context, string) (map[string]kurtosis.Service, error)
	DestroyEnclave(context.Context, string) error
}

const (
	DefaultEnclaveName  = "go-qrl-devnet"
	DefaultStartTimeout = 30 * time.Minute
	// DevelopmentWalletAddress is funded only by the built-in disposable profile.
	DevelopmentWalletAddress = "QBb81a0496aa34a64f96c2bCd28793165e1e6C08af0605b119cc768764901d2E4B48b5b9c049C57469CcA8a0421D2E31DF5C637a9cee8f3DA83964261B6CF9a22"

	destroyConfirmationTimeout = 2 * time.Minute
)

type Environment struct {
	EnclaveName  string
	Backend      Backend
	Participants []Participant

	// Primary participant aliases retained for the existing single-node suites.
	RPCURL       string
	GraphQLURL   string
	WebSocketURL string
	ConsensusURL string
}

type Participant struct {
	Index                int
	ExecutionServiceName string
	ExecutionServiceID   string
	ExecutionPrivateIP   string
	ConsensusServiceName string
	ConsensusServiceID   string
	ConsensusPrivateIP   string
	ValidatorServiceName string
	ValidatorServiceID   string
	RPCURL               string
	GraphQLURL           string
	WebSocketURL         string
	EngineURL            string
	ConsensusURL         string
	ConsensusMetricsURL  string
	ValidatorURL         string
	ValidatorMetricsURL  string
}

type StartOptions struct {
	EnclaveName string
	Backend     Backend
	Images      Images
	Parameters  []byte
	Profile     Profile
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
	backend, err := ParseBackend(os.Getenv("DEVNET_BACKEND"))
	if err != nil {
		return Environment{}, err
	}
	return NewManager().inspect(ctx, cmp.Or(os.Getenv("DEVNET_ENCLAVE_NAME"), DefaultEnclaveName), backend)
}

func (manager *Manager) Start(ctx context.Context, options StartOptions) error {
	backend, err := ParseBackend(string(options.Backend))
	if err != nil {
		return err
	}
	images := options.Images.withDefaults()
	if err := images.validate(backend); err != nil {
		return err
	}
	parameters, err := effectiveParametersForProfile(
		DevelopmentWalletAddress,
		images,
		options.Parameters,
		options.Profile,
	)
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
	environment, err := resolveEnvironment(ctx, client, options.EnclaveName, backend)
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
	backend, err := ParseBackend(os.Getenv("DEVNET_BACKEND"))
	if err != nil {
		return Environment{}, err
	}
	return manager.inspect(ctx, name, backend)
}

func (manager *Manager) inspect(ctx context.Context, name string, backend Backend) (Environment, error) {
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
	environment, err := resolveEnvironment(ctx, client, name, backend)
	if err != nil {
		return Environment{}, err
	}
	if err := manager.probe(ctx, environment.RPCURL, DevelopmentWalletAddress); err != nil {
		return Environment{}, err
	}
	return environment, nil
}

func (manager *Manager) ConsensusEndpoint(ctx context.Context, enclaveName, serviceName string) (string, error) {
	client, err := manager.newClient()
	if err != nil {
		return "", err
	}
	service, err := client.Service(ctx, enclaveName, serviceName)
	if err != nil {
		return "", err
	}
	endpoint, err := service.PublicEndpoint(consensusHTTPPortID, "http")
	if err != nil {
		return "", fmt.Errorf("consensus service %q: %w", serviceName, err)
	}
	return endpoint, nil
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
	confirmErr := retryUntil(confirmCtx, func() error {
		found, err := client.EnclaveExists(confirmCtx, name)
		if err != nil {
			return fmt.Errorf("confirm enclave destruction: %w", err)
		}
		if found {
			return errors.New("enclave still occupies its slot")
		}
		return nil
	})
	if confirmErr != nil {
		return errors.Join(destroyErr, confirmErr)
	}
	return nil
}

func resolveEnvironment(ctx context.Context, client kurtosisClient, name string, backend Backend) (Environment, error) {
	services, err := client.Services(ctx, name)
	if err != nil {
		return Environment{}, err
	}
	participants, err := participantsFromServices(services)
	if err != nil {
		return Environment{}, err
	}
	primary := participants[0]
	return Environment{
		EnclaveName:  name,
		Backend:      backend,
		Participants: participants,
		RPCURL:       primary.RPCURL,
		GraphQLURL:   primary.GraphQLURL,
		WebSocketURL: primary.WebSocketURL,
		ConsensusURL: primary.ConsensusURL,
	}, nil
}

func participantsFromServices(services map[string]kurtosis.Service) ([]Participant, error) {
	byIndex := make(map[int]*Participant)
	for name, service := range services {
		clientType := service.Labels["qrl-package.client-type"]
		if clientType != "execution" && clientType != "beacon" && clientType != "validator" {
			continue
		}
		index, err := serviceIndex(name)
		if err != nil {
			return nil, err
		}
		participant := byIndex[index]
		if participant == nil {
			participant = &Participant{Index: index}
			byIndex[index] = participant
		}
		switch clientType {
		case "execution":
			participant.ExecutionServiceName = name
			participant.ExecutionServiceID = service.UUID
			participant.ExecutionPrivateIP = service.PrivateIP
			participant.RPCURL, err = service.PublicEndpoint(rpcPortID, "http")
			if err != nil {
				return nil, fmt.Errorf("execution service %q: %w", name, err)
			}
			participant.GraphQLURL = participant.RPCURL + graphQLPath
			participant.WebSocketURL, err = service.PublicEndpoint(webSocketPortID, "ws")
			if err != nil {
				return nil, fmt.Errorf("execution service %q: %w", name, err)
			}
			participant.EngineURL = optionalPublicEndpoint(service, "engine-rpc", "http")
		case "beacon":
			participant.ConsensusServiceName = name
			participant.ConsensusServiceID = service.UUID
			participant.ConsensusPrivateIP = service.PrivateIP
			participant.ConsensusURL, err = service.PublicEndpoint(consensusHTTPPortID, "http")
			if err != nil {
				return nil, fmt.Errorf("consensus service %q: %w", name, err)
			}
			participant.ConsensusMetricsURL = optionalPublicEndpoint(service, metricsPortID, "http")
		case "validator":
			participant.ValidatorServiceName = name
			participant.ValidatorServiceID = service.UUID
			participant.ValidatorURL = optionalPublicEndpoint(service, "http-validator", "http")
			participant.ValidatorMetricsURL = optionalPublicEndpoint(service, metricsPortID, "http")
		}
	}
	if len(byIndex) == 0 {
		return nil, errors.New("no qrl-package participants found")
	}
	participants := make([]Participant, 0, len(byIndex))
	for _, participant := range byIndex {
		if participant.RPCURL == "" || participant.ConsensusURL == "" {
			return nil, fmt.Errorf("participant %d is missing an execution or consensus endpoint", participant.Index)
		}
		participants = append(participants, *participant)
	}
	sort.Slice(participants, func(i, j int) bool { return participants[i].Index < participants[j].Index })
	return participants, nil
}

func serviceIndex(name string) (int, error) {
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return 0, fmt.Errorf("qrl-package service %q has no participant index", name)
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 1 {
		return 0, fmt.Errorf("qrl-package service %q has invalid participant index", name)
	}
	return index, nil
}

func optionalPublicEndpoint(service kurtosis.Service, portID, scheme string) string {
	endpoint, _ := service.PublicEndpoint(portID, scheme)
	return endpoint
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
