package validatorops

import (
	"fmt"
	"strconv"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscontext"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscrypto"
	"github.com/theQRL/go-qrl/common/hexutil"
)

func ProposerSlashing(key *Key, validatorIndex, slot uint64, chain consensuscontext.Context) (any, error) {
	epoch := chain.Epoch(slot)
	header1 := consensuscrypto.BeaconBlockHeader{
		Slot: slot, ProposerIndex: validatorIndex, BodyRoot: rootWithMarker(1),
	}
	header2 := consensuscrypto.BeaconBlockHeader{
		Slot: slot, ProposerIndex: validatorIndex, BodyRoot: rootWithMarker(2),
	}
	signature1, err := sign(key, header1, consensuscrypto.DomainBeaconProposer, epoch, chain)
	if err != nil {
		return nil, err
	}
	signature2, err := sign(key, header2, consensuscrypto.DomainBeaconProposer, epoch, chain)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"signed_header_1": signedHeaderJSON(header1, signature1),
		"signed_header_2": signedHeaderJSON(header2, signature2),
	}, nil
}

func AttesterSlashing(key *Key, validatorIndex, slot, finalizedEpoch uint64, chain consensuscontext.Context) (any, error) {
	epoch := chain.Epoch(slot)
	first := attestationData(slot, epoch, finalizedEpoch, 1)
	second := attestationData(slot, epoch, finalizedEpoch, 2)
	firstSignature, err := sign(key, first, consensuscrypto.DomainBeaconAttester, epoch, chain)
	if err != nil {
		return nil, err
	}
	secondSignature, err := sign(key, second, consensuscrypto.DomainBeaconAttester, epoch, chain)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"attestation_1": indexedAttestationJSON(validatorIndex, first, firstSignature),
		"attestation_2": indexedAttestationJSON(validatorIndex, second, secondSignature),
	}, nil
}

func VoluntaryExit(key *Key, validatorIndex, epoch uint64, chain consensuscontext.Context) (any, error) {
	exit := consensuscrypto.VoluntaryExit{Epoch: epoch, ValidatorIndex: validatorIndex}
	signature, err := sign(key, exit, consensuscrypto.DomainVoluntaryExit, epoch, chain)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"message": map[string]string{
			"epoch": strconv.FormatUint(epoch, 10), "validator_index": strconv.FormatUint(validatorIndex, 10),
		},
		"signature": hexutil.Encode(signature),
	}, nil
}

func sign(
	key *Key,
	object consensuscrypto.HashRoot,
	domainType [4]byte,
	epoch uint64,
	chain consensuscontext.Context,
) ([]byte, error) {
	domain := chain.Domain(domainType, epoch)
	objectRoot, err := object.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	root := consensuscrypto.SigningRoot(objectRoot, domain)
	signature, err := key.Sign(root[:])
	if err != nil {
		return nil, err
	}
	if err := consensuscrypto.Verify(root, key.PublicKey(), signature); err != nil {
		return nil, fmt.Errorf("verify generated validator signature: %w", err)
	}
	return signature, nil
}

func signedHeaderJSON(header consensuscrypto.BeaconBlockHeader, signature []byte) map[string]any {
	return map[string]any{
		"message": map[string]string{
			"slot":           strconv.FormatUint(header.Slot, 10),
			"proposer_index": strconv.FormatUint(header.ProposerIndex, 10),
			"parent_root":    hexutil.Encode(header.ParentRoot[:]),
			"state_root":     hexutil.Encode(header.StateRoot[:]),
			"body_root":      hexutil.Encode(header.BodyRoot[:]),
		},
		"signature": hexutil.Encode(signature),
	}
}

func indexedAttestationJSON(
	validatorIndex uint64,
	data consensuscrypto.AttestationData,
	signature []byte,
) map[string]any {
	return map[string]any{
		"attesting_indices": []string{strconv.FormatUint(validatorIndex, 10)},
		"data": map[string]any{
			"slot":              strconv.FormatUint(data.Slot, 10),
			"index":             strconv.FormatUint(data.CommitteeIndex, 10),
			"beacon_block_root": hexutil.Encode(data.BeaconBlockRoot[:]),
			"source": map[string]string{
				"epoch": strconv.FormatUint(data.Source.Epoch, 10), "root": hexutil.Encode(data.Source.Root[:]),
			},
			"target": map[string]string{
				"epoch": strconv.FormatUint(data.Target.Epoch, 10), "root": hexutil.Encode(data.Target.Root[:]),
			},
		},
		"signatures": []string{hexutil.Encode(signature)},
	}
}

func attestationData(slot, targetEpoch, sourceEpoch uint64, marker byte) consensuscrypto.AttestationData {
	return consensuscrypto.AttestationData{
		Slot:            slot,
		BeaconBlockRoot: rootWithMarker(marker),
		Source:          consensuscrypto.Checkpoint{Epoch: sourceEpoch},
		Target:          consensuscrypto.Checkpoint{Epoch: targetEpoch},
	}
}

func rootWithMarker(marker byte) [consensuscrypto.RootLength]byte {
	var root [consensuscrypto.RootLength]byte
	root[len(root)-1] = marker
	return root
}
