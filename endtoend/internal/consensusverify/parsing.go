// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensusverify

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

func committee(ctx context.Context, client consensusAPI, stateID string, data consensus.AttestationData) ([]uint64, error) {
	slot, err := decimal("attestation slot", data.Slot)
	if err != nil {
		return nil, err
	}
	index, err := decimal("attestation committee index", data.CommitteeIndex)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data []struct {
			Index      string   `json:"index"`
			Slot       string   `json:"slot"`
			Validators []string `json:"validators"`
		} `json:"data"`
	}
	path := fmt.Sprintf(
		"/qrl/v1/beacon/states/%s/committees?slot=%d&index=%d",
		url.PathEscape(stateID),
		slot,
		index,
	)
	if err := client.GetJSON(ctx, path, &response); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 {
		return nil, fmt.Errorf("expected one committee, got %d", len(response.Data))
	}
	return decimalSlice("committee validator", response.Data[0].Validators)
}

func syncCommittee(ctx context.Context, client consensusAPI, stateID string) ([]uint64, error) {
	var response struct {
		Data struct {
			Validators []string `json:"validators"`
		} `json:"data"`
	}
	if err := client.GetJSON(ctx, "/qrl/v1/beacon/states/"+url.PathEscape(stateID)+"/sync_committees", &response); err != nil {
		return nil, err
	}
	return decimalSlice("sync committee validator", response.Data.Validators)
}

func beaconBlockHeader(value consensus.BeaconBlockHeader) (*qrysmpb.BeaconBlockHeader, uint64, error) {
	slot, err := decimal("block header slot", value.Slot)
	if err != nil {
		return nil, 0, err
	}
	proposer, err := decimal("block header proposer index", value.ProposerIndex)
	if err != nil {
		return nil, 0, err
	}
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
		Slot: primitives.Slot(slot), ProposerIndex: primitives.ValidatorIndex(proposer),
		ParentRoot: parentRoot, StateRoot: stateRoot, BodyRoot: bodyRoot,
	}, proposer, nil
}

func attestationData(value consensus.AttestationData) (*qrysmpb.AttestationData, uint64, error) {
	slot, err := decimal("attestation slot", value.Slot)
	if err != nil {
		return nil, 0, err
	}
	committeeIndex, err := decimal("attestation committee index", value.CommitteeIndex)
	if err != nil {
		return nil, 0, err
	}
	targetEpoch, err := decimal("attestation target epoch", value.Target.Epoch)
	if err != nil {
		return nil, 0, err
	}
	sourceEpoch, err := decimal("attestation source epoch", value.Source.Epoch)
	if err != nil {
		return nil, 0, err
	}
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
		Slot: primitives.Slot(slot), CommitteeIndex: primitives.CommitteeIndex(committeeIndex), BeaconBlockRoot: beaconRoot,
		Source: &qrysmpb.Checkpoint{Epoch: primitives.Epoch(sourceEpoch), Root: sourceRoot},
		Target: &qrysmpb.Checkpoint{Epoch: primitives.Epoch(targetEpoch), Root: targetRoot},
	}, targetEpoch, nil
}

func depositData(value consensus.DepositData) (*qrysmpb.Deposit_Data, error) {
	publicKey, err := decodeFixed("deposit public key", value.PublicKey, fieldparams.MLDSA87PubkeyLength)
	if err != nil {
		return nil, err
	}
	withdrawalCredentials, err := decodeFixed("deposit withdrawal credentials", value.WithdrawalCredentials, fieldparams.FeeRecipientLength)
	if err != nil {
		return nil, err
	}
	amount, err := decimal("deposit amount", value.Amount)
	if err != nil {
		return nil, err
	}
	signature, err := decodeFixed("deposit signature", value.Signature, fieldparams.MLDSA87SignatureLength)
	if err != nil {
		return nil, err
	}
	return &qrysmpb.Deposit_Data{
		PublicKey: publicKey, WithdrawalCredentials: withdrawalCredentials, Amount: amount, Signature: signature,
	}, nil
}

func decimalSlice(name string, values []string) ([]uint64, error) {
	result := make([]uint64, len(values))
	for index, value := range values {
		parsed, err := decimal(name, value)
		if err != nil {
			return nil, err
		}
		result[index] = parsed
	}
	return result, nil
}

func decimal(name, value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s %q: %w", name, value, err)
	}
	return parsed, nil
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

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
