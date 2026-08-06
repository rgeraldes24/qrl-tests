// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensusverify

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/beacon"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscontext"
)

// SignatureSummary records every consensus signature verified in a block.
type SignatureSummary struct {
	Block             int
	Randao            int
	Attestations      int
	SyncCommittee     int
	Deposits          int
	VoluntaryExits    int
	ProposerSlashings int
	AttesterSlashings int
}

func (summary SignatureSummary) Total() int {
	return summary.Block + summary.Randao + summary.Attestations + summary.SyncCommittee +
		summary.Deposits + summary.VoluntaryExits + summary.ProposerSlashings + summary.AttesterSlashings
}

type Verifier struct {
	client  consensusAPI
	chain   consensuscontext.Context
	pubkeys map[uint64][]byte
}

type consensusAPI interface {
	consensuscontext.Source
	Block(context.Context, string) (beacon.SignedBlock, error)
	BlockHeader(context.Context, string) (beacon.BlockHeader, error)
	Committee(context.Context, string, uint64, uint64) ([]uint64, error)
	SyncCommittee(context.Context, string) ([]uint64, error)
	Validator(context.Context, string) (beacon.Validator, error)
}

func New(ctx context.Context, client consensusAPI) (*Verifier, error) {
	chain, err := consensuscontext.Load(ctx, client)
	if err != nil {
		return nil, err
	}
	return &Verifier{client: client, chain: chain, pubkeys: make(map[uint64][]byte)}, nil
}

// VerifyBlock fetches blockID and verifies every consensus signature it carries.
func (verification *Verifier) VerifyBlock(ctx context.Context, blockID string) (SignatureSummary, error) {
	header, err := verification.client.BlockHeader(ctx, blockID)
	if err != nil {
		return SignatureSummary{}, err
	}
	block, err := verification.client.Block(ctx, blockID)
	if err != nil {
		return SignatureSummary{}, err
	}
	return verification.Verify(ctx, header, block)
}

// Verify verifies every consensus signature in an already fetched block.
func (verification *Verifier) Verify(
	ctx context.Context,
	header beacon.BlockHeader,
	block beacon.SignedBlock,
) (SignatureSummary, error) {
	summary := SignatureSummary{}
	if err := verification.verifyBlockHeader(ctx, header.Header.Message, header.Header.Signature, header.Root); err != nil {
		return summary, fmt.Errorf("verify block header signature: %w", err)
	}
	summary.Block++

	slot := block.Message.Slot
	proposer := block.Message.ProposerIndex
	if header.Header.Message.Slot != slot {
		return summary, fmt.Errorf("block and header slot mismatch")
	}
	if header.Header.Message.ProposerIndex != proposer {
		return summary, fmt.Errorf("block and header proposer mismatch")
	}
	if !strings.EqualFold(header.Header.Message.ParentRoot, block.Message.ParentRoot) {
		return summary, fmt.Errorf("block and header parent root mismatch")
	}
	if !strings.EqualFold(header.Header.Message.StateRoot, block.Message.StateRoot) {
		return summary, fmt.Errorf("block and header state root mismatch")
	}
	if !strings.EqualFold(header.Header.Signature, block.Signature) {
		return summary, fmt.Errorf("block and header signature mismatch")
	}
	if err := verification.verifyRandao(ctx, slot, proposer, block.Message.Body.RandaoReveal); err != nil {
		return summary, fmt.Errorf("verify RANDAO signature: %w", err)
	}
	summary.Randao++

	stateID := strconv.FormatUint(block.Message.Slot, 10)
	for index, attestation := range block.Message.Body.Attestations {
		count, err := verification.verifyAttestation(ctx, attestation)
		if err != nil {
			return summary, fmt.Errorf("verify attestation %d: %w", index, err)
		}
		summary.Attestations += count
	}
	count, err := verification.verifySyncAggregate(
		ctx,
		stateID,
		slot,
		block.Message.ParentRoot,
		block.Message.Body.SyncAggregate.Bits,
		block.Message.Body.SyncAggregate.Signatures,
	)
	if err != nil {
		return summary, fmt.Errorf("verify sync aggregate: %w", err)
	}
	summary.SyncCommittee += count

	count, err = verification.verifyDeposits(block.Message.Body.Deposits)
	if err != nil {
		return summary, err
	}
	summary.Deposits += count

	for index, item := range block.Message.Body.VoluntaryExits {
		if err := verification.verifyVoluntaryExit(ctx, item.Message, item.Signature); err != nil {
			return summary, fmt.Errorf("verify voluntary exit %d: %w", index, err)
		}
		summary.VoluntaryExits++
	}
	for index, item := range block.Message.Body.ProposerSlashings {
		if err := verification.verifySignedHeader(ctx, item.Header1); err != nil {
			return summary, fmt.Errorf("verify proposer slashing %d header 1: %w", index, err)
		}
		if err := verification.verifySignedHeader(ctx, item.Header2); err != nil {
			return summary, fmt.Errorf("verify proposer slashing %d header 2: %w", index, err)
		}
		summary.ProposerSlashings += 2
	}
	for index, item := range block.Message.Body.AttesterSlashings {
		count1, err := verification.verifyIndexedAttestation(ctx, item.Attestation1)
		if err != nil {
			return summary, fmt.Errorf("verify attester slashing %d attestation 1: %w", index, err)
		}
		count2, err := verification.verifyIndexedAttestation(ctx, item.Attestation2)
		if err != nil {
			return summary, fmt.Errorf("verify attester slashing %d attestation 2: %w", index, err)
		}
		summary.AttesterSlashings += count1 + count2
	}
	return summary, nil
}
