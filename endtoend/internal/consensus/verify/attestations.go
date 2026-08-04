// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensusverify

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/client"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/crypto"
	"github.com/theQRL/go-bitfield"
)

func (verification *Verifier) verifyAttestation(
	ctx context.Context,
	attestation consensus.Attestation,
) (int, error) {
	bits, err := decodeHex("attestation aggregation bits", attestation.AggregationBits)
	if err != nil {
		return 0, err
	}
	positions := bitfield.Bitlist(bits).BitIndices()
	stateID := strconv.FormatUint(attestation.Data.Slot, 10)
	committee, err := committee(ctx, verification.client, stateID, attestation.Data)
	if err != nil {
		return 0, err
	}
	indices := make([]uint64, len(positions))
	for index, position := range positions {
		if int(position) >= len(committee) {
			return 0, fmt.Errorf("aggregation bit %d exceeds committee length %d", position, len(committee))
		}
		indices[index] = committee[position]
	}
	return verification.verifyAttestationSignatures(ctx, attestation.Data, indices, attestation.Signatures)
}

func (verification *Verifier) verifyIndexedAttestation(
	ctx context.Context,
	attestation consensus.IndexedAttestation,
) (int, error) {
	return verification.verifyAttestationSignatures(
		ctx, attestation.Data, attestation.AttestingIndices, attestation.Signatures,
	)
}

func (verification *Verifier) verifyAttestationSignatures(
	ctx context.Context,
	data consensus.AttestationData,
	indices []uint64,
	signatures []string,
) (int, error) {
	if len(indices) != len(signatures) {
		return 0, fmt.Errorf("attestation has %d participants and %d signatures", len(indices), len(signatures))
	}
	message, targetEpoch, err := attestationData(data)
	if err != nil {
		return 0, err
	}
	for index, validatorIndex := range indices {
		if err := verification.verifyObject(
			ctx,
			message,
			validatorIndex,
			targetEpoch,
			consensuscrypto.DomainBeaconAttester,
			signatures[index],
		); err != nil {
			return index, fmt.Errorf("participant %d: %w", validatorIndex, err)
		}
	}
	return len(signatures), nil
}
