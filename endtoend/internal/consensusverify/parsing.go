// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensusverify

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

func committee(ctx context.Context, client consensusAPI, stateID string, data consensus.AttestationData) ([]uint64, error) {
	return client.Committee(ctx, stateID, data.Slot, data.CommitteeIndex)
}

func beaconBlockHeader(value consensus.BeaconBlockHeader) (*qrysmpb.BeaconBlockHeader, uint64, error) {
	parentRoot, err := decodeFixed("block header parent root", value.ParentRoot, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	stateRoot, err := decodeFixed("block header state root", value.StateRoot, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	bodyRoot, err := decodeFixed("block header body root", value.BodyRoot, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	return &qrysmpb.BeaconBlockHeader{
		Slot: primitives.Slot(value.Slot), ProposerIndex: primitives.ValidatorIndex(value.ProposerIndex),
		ParentRoot: parentRoot, StateRoot: stateRoot, BodyRoot: bodyRoot,
	}, value.ProposerIndex, nil
}

func attestationData(value consensus.AttestationData) (*qrysmpb.AttestationData, uint64, error) {
	beaconRoot, err := decodeFixed("attestation beacon block root", value.BeaconBlockRoot, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	sourceRoot, err := decodeFixed("attestation source root", value.Source.Root, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	targetRoot, err := decodeFixed("attestation target root", value.Target.Root, fieldparams.RootLength)
	if err != nil {
		return nil, 0, err
	}
	return &qrysmpb.AttestationData{
		Slot: primitives.Slot(value.Slot), CommitteeIndex: primitives.CommitteeIndex(value.CommitteeIndex), BeaconBlockRoot: beaconRoot,
		Source: &qrysmpb.Checkpoint{Epoch: primitives.Epoch(value.Source.Epoch), Root: sourceRoot},
		Target: &qrysmpb.Checkpoint{Epoch: primitives.Epoch(value.Target.Epoch), Root: targetRoot},
	}, value.Target.Epoch, nil
}

func depositData(value consensus.Deposit) (*qrysmpb.Deposit_Data, error) {
	publicKey, err := decodeFixed("deposit public key", value.PublicKey, fieldparams.MLDSA87PubkeyLength)
	if err != nil {
		return nil, err
	}
	withdrawalCredentials, err := decodeFixed("deposit withdrawal credentials", value.WithdrawalCredentials, fieldparams.FeeRecipientLength)
	if err != nil {
		return nil, err
	}
	signature, err := decodeFixed("deposit signature", value.Signature, fieldparams.MLDSA87SignatureLength)
	if err != nil {
		return nil, err
	}
	return &qrysmpb.Deposit_Data{
		PublicKey: publicKey, WithdrawalCredentials: withdrawalCredentials, Amount: value.Amount, Signature: signature,
	}, nil
}

func decodeFixed(name, value string, length int) ([]byte, error) {
	decoded, err := decodeHex(name, value)
	if err != nil {
		return nil, err
	}
	if len(decoded) != length {
		return nil, fmt.Errorf("invalid %s length %d, want %d", name, len(decoded), length)
	}
	return decoded, nil
}

func decodeHex(name, value string) ([]byte, error) {
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", name, err)
	}
	return decoded, nil
}
