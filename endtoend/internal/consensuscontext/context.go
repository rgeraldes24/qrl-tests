// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

// Package consensuscontext resolves the chain configuration used to sign and
// verify consensus objects.
package consensuscontext

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/config/params"
)

type Source interface {
	Genesis(context.Context) (consensus.Genesis, error)
	Fork(context.Context) (consensus.Fork, error)
	SpecUint(context.Context, string) (uint64, error)
}

type Context struct {
	SlotsPerEpoch uint64

	genesisRoot        [32]byte
	genesisForkVersion [4]byte
	previousVersion    [4]byte
	currentVersion     [4]byte
	forkEpoch          uint64
}

func Load(ctx context.Context, source Source) (Context, error) {
	genesis, err := source.Genesis(ctx)
	if err != nil {
		return Context{}, err
	}
	var genesisRoot [32]byte
	if err := decodeFixed("genesis validators root", genesis.ValidatorsRoot, genesisRoot[:]); err != nil {
		return Context{}, err
	}
	var genesisForkVersion [4]byte
	if err := decodeFixed("genesis fork version", genesis.ForkVersion, genesisForkVersion[:]); err != nil {
		return Context{}, err
	}
	fork, err := source.Fork(ctx)
	if err != nil {
		return Context{}, err
	}
	var previousVersion [4]byte
	if err := decodeFixed("previous fork version", fork.PreviousVersion, previousVersion[:]); err != nil {
		return Context{}, err
	}
	var currentVersion [4]byte
	if err := decodeFixed("current fork version", fork.CurrentVersion, currentVersion[:]); err != nil {
		return Context{}, err
	}
	slotsPerEpoch, err := source.SpecUint(ctx, "SLOTS_PER_EPOCH")
	if err != nil {
		return Context{}, err
	}
	return Context{
		SlotsPerEpoch:      slotsPerEpoch,
		genesisRoot:        genesisRoot,
		genesisForkVersion: genesisForkVersion,
		previousVersion:    previousVersion,
		currentVersion:     currentVersion,
		forkEpoch:          fork.Epoch,
	}, nil
}

func (chain Context) Epoch(slot uint64) uint64 {
	return slot / chain.SlotsPerEpoch
}

func (chain Context) Domain(domainType [4]byte, epoch uint64) ([]byte, error) {
	version := chain.currentVersion
	if epoch < chain.forkEpoch {
		version = chain.previousVersion
	}
	return signing.ComputeDomain(domainType, version[:], chain.genesisRoot[:])
}

func (chain Context) DepositDomain() ([]byte, error) {
	return signing.ComputeDomain(params.BeaconConfig().DomainDeposit, chain.genesisForkVersion[:], nil)
}

func (chain Context) GenesisForkVersion() [4]byte {
	return chain.genesisForkVersion
}

func decodeFixed(name, value string, result []byte) error {
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	if len(decoded) != len(result) {
		return fmt.Errorf("invalid %s length %d, want %d", name, len(decoded), len(result))
	}
	copy(result, decoded)
	return nil
}
