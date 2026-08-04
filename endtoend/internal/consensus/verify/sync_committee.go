// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensusverify

import (
	"context"
	"fmt"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/crypto"
)

func (verification *Verifier) verifySyncAggregate(
	ctx context.Context,
	stateID string,
	slot uint64,
	parentRootHex,
	bitsHex string,
	signatures []string,
) (int, error) {
	bits, err := decodeHex("sync committee bits", bitsHex)
	if err != nil {
		return 0, err
	}
	committee, err := verification.client.SyncCommittee(ctx, stateID)
	if err != nil {
		return 0, err
	}
	participants := make([]uint64, 0, len(committee))
	for index, validatorIndex := range committee {
		if index/8 < len(bits) && bits[index/8]&(1<<uint(index%8)) != 0 {
			participants = append(participants, validatorIndex)
		}
	}
	if len(participants) != len(signatures) {
		return 0, fmt.Errorf("sync aggregate has %d participants and %d signatures", len(participants), len(signatures))
	}
	parentRoot, err := decodeFixed("parent block root", parentRootHex, consensuscrypto.RootLength)
	if err != nil {
		return 0, err
	}
	object := consensuscrypto.Root(parentRoot)
	epoch := uint64(0)
	if slot > 0 {
		epoch = verification.chain.Epoch(slot - 1)
	}
	for index, validatorIndex := range participants {
		if err := verification.verifyObject(
			ctx,
			object,
			validatorIndex,
			epoch,
			consensuscrypto.DomainSyncCommittee,
			signatures[index],
		); err != nil {
			return index, fmt.Errorf("participant %d: %w", validatorIndex, err)
		}
	}
	return len(signatures), nil
}
