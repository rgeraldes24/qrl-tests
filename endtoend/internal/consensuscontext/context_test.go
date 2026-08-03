// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensuscontext

import (
	"context"
	"strings"
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/stretchr/testify/require"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/config/params"
)

type contextSource struct {
	genesis consensus.Genesis
	fork    consensus.Fork
	slots   uint64
}

func (source contextSource) Genesis(context.Context) (consensus.Genesis, error) {
	return source.genesis, nil
}

func (source contextSource) Fork(context.Context) (consensus.Fork, error) {
	return source.fork, nil
}

func (source contextSource) SpecUint(context.Context, string) (uint64, error) {
	return source.slots, nil
}

func TestContextDomains(t *testing.T) {
	root := "11" + strings.Repeat("00", 31)
	source := contextSource{
		genesis: consensus.Genesis{ValidatorsRoot: "0x" + root, ForkVersion: "0x00000001"},
		fork: consensus.Fork{
			PreviousVersion: "0x00000002",
			CurrentVersion:  "0x00000003",
			Epoch:           10,
		},
		slots: 8,
	}
	chain, err := Load(t.Context(), source)
	require.NoError(t, err)
	require.Equal(t, uint64(2), chain.Epoch(17))
	require.Equal(t, [4]byte{0, 0, 0, 1}, chain.GenesisForkVersion())

	domainType := [4]byte{1, 2, 3, 4}
	previous, err := chain.Domain(domainType, 9)
	require.NoError(t, err)
	wantPrevious, err := signing.ComputeDomain(domainType, []byte{0, 0, 0, 2}, append([]byte{0x11}, make([]byte, 31)...))
	require.NoError(t, err)
	require.Equal(t, wantPrevious, previous)

	current, err := chain.Domain(domainType, 10)
	require.NoError(t, err)
	wantCurrent, err := signing.ComputeDomain(domainType, []byte{0, 0, 0, 3}, append([]byte{0x11}, make([]byte, 31)...))
	require.NoError(t, err)
	require.Equal(t, wantCurrent, current)

	depositDomain, err := chain.DepositDomain()
	require.NoError(t, err)
	wantDepositDomain, err := signing.ComputeDomain(params.BeaconConfig().DomainDeposit, []byte{0, 0, 0, 1}, nil)
	require.NoError(t, err)
	require.Equal(t, wantDepositDomain, depositDomain)
}
