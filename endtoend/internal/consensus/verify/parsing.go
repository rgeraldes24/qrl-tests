// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensusverify

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/client"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/crypto"
)

func committee(ctx context.Context, client consensusAPI, stateID string, data consensus.AttestationData) ([]uint64, error) {
	return client.Committee(ctx, stateID, data.Slot, data.CommitteeIndex)
}

func beaconBlockHeader(value consensus.BeaconBlockHeader) (consensuscrypto.BeaconBlockHeader, uint64, error) {
	parentRoot, err := decodeFixed("block header parent root", value.ParentRoot, consensuscrypto.RootLength)
	if err != nil {
		return consensuscrypto.BeaconBlockHeader{}, 0, err
	}
	stateRoot, err := decodeFixed("block header state root", value.StateRoot, consensuscrypto.RootLength)
	if err != nil {
		return consensuscrypto.BeaconBlockHeader{}, 0, err
	}
	bodyRoot, err := decodeFixed("block header body root", value.BodyRoot, consensuscrypto.RootLength)
	if err != nil {
		return consensuscrypto.BeaconBlockHeader{}, 0, err
	}
	return consensuscrypto.BeaconBlockHeader{
		Slot: value.Slot, ProposerIndex: value.ProposerIndex,
		ParentRoot: [32]byte(parentRoot), StateRoot: [32]byte(stateRoot), BodyRoot: [32]byte(bodyRoot),
	}, value.ProposerIndex, nil
}

func attestationData(value consensus.AttestationData) (consensuscrypto.AttestationData, uint64, error) {
	beaconRoot, err := decodeFixed("attestation beacon block root", value.BeaconBlockRoot, consensuscrypto.RootLength)
	if err != nil {
		return consensuscrypto.AttestationData{}, 0, err
	}
	sourceRoot, err := decodeFixed("attestation source root", value.Source.Root, consensuscrypto.RootLength)
	if err != nil {
		return consensuscrypto.AttestationData{}, 0, err
	}
	targetRoot, err := decodeFixed("attestation target root", value.Target.Root, consensuscrypto.RootLength)
	if err != nil {
		return consensuscrypto.AttestationData{}, 0, err
	}
	return consensuscrypto.AttestationData{
		Slot: value.Slot, CommitteeIndex: value.CommitteeIndex, BeaconBlockRoot: [32]byte(beaconRoot),
		Source: consensuscrypto.Checkpoint{Epoch: value.Source.Epoch, Root: [32]byte(sourceRoot)},
		Target: consensuscrypto.Checkpoint{Epoch: value.Target.Epoch, Root: [32]byte(targetRoot)},
	}, value.Target.Epoch, nil
}

func depositData(value consensus.Deposit) (consensuscrypto.DepositData, error) {
	publicKey, err := decodeFixed("deposit public key", value.PublicKey, consensuscrypto.PublicKeyLength)
	if err != nil {
		return consensuscrypto.DepositData{}, err
	}
	withdrawalCredentials, err := decodeFixed("deposit withdrawal credentials", value.WithdrawalCredentials, consensuscrypto.FeeRecipientSize)
	if err != nil {
		return consensuscrypto.DepositData{}, err
	}
	signature, err := decodeFixed("deposit signature", value.Signature, consensuscrypto.SignatureLength)
	if err != nil {
		return consensuscrypto.DepositData{}, err
	}
	return consensuscrypto.DepositData{
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
