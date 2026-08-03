// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package devnet

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const networkToolImage = "nicolaka/netshoot:v0.13"

var ErrNetworkPartitionUnsupported = errors.New("network partitions are not supported by the Kubernetes backend")

type NetworkPartition interface {
	Apply(context.Context, ...[]Participant) error
	Clear(context.Context) error
}

type dockerNetworkPartition struct {
	run   func(context.Context, ...string) (string, error)
	rules []networkRule
}

type networkRule struct {
	container string
	peerIP    string
}

func NewNetworkPartition(backend Backend) (NetworkPartition, error) {
	backend, err := ParseBackend(string(backend))
	if err != nil {
		return nil, err
	}
	if backend == BackendKubernetes {
		return nil, ErrNetworkPartitionUnsupported
	}
	return &dockerNetworkPartition{run: runDocker}, nil
}

func (partition *dockerNetworkPartition) Apply(ctx context.Context, groups ...[]Participant) error {
	if len(groups) < 2 {
		return errors.New("network partition requires at least two groups")
	}
	for sourceGroup, sources := range groups {
		for destinationGroup, destinations := range groups {
			if sourceGroup == destinationGroup {
				continue
			}
			for _, source := range sources {
				for _, destination := range destinations {
					for _, endpoints := range []struct {
						serviceID string
						peerIP    string
					}{
						{source.ExecutionServiceID, destination.ExecutionPrivateIP},
						{source.ConsensusServiceID, destination.ConsensusPrivateIP},
					} {
						if endpoints.serviceID == "" || endpoints.peerIP == "" {
							return errors.New("participant is missing a service ID or private IP")
						}
						container, err := partition.containerID(ctx, endpoints.serviceID)
						if err != nil {
							partition.clearApplied(context.Background())
							return err
						}
						rule := networkRule{container: container, peerIP: endpoints.peerIP}
						if err := partition.updateRule(ctx, "-I", rule); err != nil {
							partition.clearApplied(context.Background())
							return err
						}
						partition.rules = append(partition.rules, rule)
					}
				}
			}
		}
	}
	return nil
}

func (partition *dockerNetworkPartition) Clear(ctx context.Context) error {
	var result error
	for index := len(partition.rules) - 1; index >= 0; index-- {
		result = errors.Join(result, partition.updateRule(ctx, "-D", partition.rules[index]))
	}
	partition.rules = nil
	return result
}

func (partition *dockerNetworkPartition) clearApplied(ctx context.Context) {
	_ = partition.Clear(ctx)
}

func (partition *dockerNetworkPartition) containerID(ctx context.Context, serviceID string) (string, error) {
	output, err := partition.run(ctx,
		"ps", "--all", "--quiet",
		"--filter", "label=com.kurtosistech.guid="+serviceID,
	)
	if err != nil {
		return "", fmt.Errorf("find Kurtosis service container %s: %w", serviceID, err)
	}
	identifiers := strings.Fields(output)
	if len(identifiers) != 1 {
		return "", fmt.Errorf("expected one container for Kurtosis service %s, found %d", serviceID, len(identifiers))
	}
	return identifiers[0], nil
}

func (partition *dockerNetworkPartition) updateRule(ctx context.Context, operation string, rule networkRule) error {
	_, err := partition.run(ctx,
		"run", "--rm",
		"--network", "container:"+rule.container,
		"--cap-add", "NET_ADMIN",
		networkToolImage,
		"iptables", operation, "OUTPUT", "-d", rule.peerIP, "-j", "DROP",
	)
	if err != nil {
		return fmt.Errorf("update partition rule for %s to %s: %w", rule.container, rule.peerIP, err)
	}
	return nil
}

func runDocker(ctx context.Context, arguments ...string) (string, error) {
	output, err := exec.CommandContext(ctx, "docker", arguments...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
