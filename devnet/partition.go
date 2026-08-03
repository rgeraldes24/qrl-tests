// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package devnet

import (
	"context"
	"errors"
)

var ErrNetworkPartitionUnsupported = errors.New("network partitions are not supported by the Kubernetes backend")

type NetworkPartition interface {
	Apply(context.Context, ...[]Participant) error
	Clear(context.Context) error
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
