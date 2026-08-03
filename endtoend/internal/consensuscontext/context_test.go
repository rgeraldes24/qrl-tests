// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensuscontext

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/stretchr/testify/require"
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
	require.Equal(t, mustDomain(t, "010203045f4c4b0ed11ed93379263b2e23b10940f33d3d0aee534c93105e1b58"), previous)

	current, err := chain.Domain(domainType, 10)
	require.NoError(t, err)
	require.Equal(t, mustDomain(t, "01020304b89638cec7278d3fffb7ecd79da316be4154b8886a92c206be6c8d33"), current)

	depositDomain, err := chain.DepositDomain()
	require.NoError(t, err)
	require.Equal(t, mustDomain(t, "0300000018ae4ccbda9538839d79bb18ca09e23e24ae8c1550f56cbb3d84b053"), depositDomain)
}

func mustDomain(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	require.NoError(t, err)
	return decoded
}
