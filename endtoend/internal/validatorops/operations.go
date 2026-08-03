package validatorops

import (
	"fmt"
	"strconv"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscontext"
	fastssz "github.com/prysmaticlabs/fastssz"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

func ProposerSlashing(key ml_dsa_87.MLDSA87Key, validatorIndex, slot uint64, chain consensuscontext.Context) (any, error) {
	epoch := chain.Epoch(slot)
	header1 := &qrysmpb.BeaconBlockHeader{
		Slot: primitives.Slot(slot), ProposerIndex: primitives.ValidatorIndex(validatorIndex),
		ParentRoot: make([]byte, 32), StateRoot: make([]byte, 32), BodyRoot: rootWithMarker(1),
	}
	header2 := &qrysmpb.BeaconBlockHeader{
		Slot: primitives.Slot(slot), ProposerIndex: primitives.ValidatorIndex(validatorIndex),
		ParentRoot: make([]byte, 32), StateRoot: make([]byte, 32), BodyRoot: rootWithMarker(2),
	}
	signature1, err := sign(key, header1, params.BeaconConfig().DomainBeaconProposer, epoch, chain)
	if err != nil {
		return nil, err
	}
	signature2, err := sign(key, header2, params.BeaconConfig().DomainBeaconProposer, epoch, chain)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"signed_header_1": signedHeaderJSON(header1, signature1),
		"signed_header_2": signedHeaderJSON(header2, signature2),
	}, nil
}

func AttesterSlashing(key ml_dsa_87.MLDSA87Key, validatorIndex, slot, finalizedEpoch uint64, chain consensuscontext.Context) (any, error) {
	epoch := chain.Epoch(slot)
	first := attestationData(slot, epoch, finalizedEpoch, 1)
	second := attestationData(slot, epoch, finalizedEpoch, 2)
	firstSignature, err := sign(key, first, params.BeaconConfig().DomainBeaconAttester, epoch, chain)
	if err != nil {
		return nil, err
	}
	secondSignature, err := sign(key, second, params.BeaconConfig().DomainBeaconAttester, epoch, chain)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"attestation_1": indexedAttestationJSON(validatorIndex, first, firstSignature),
		"attestation_2": indexedAttestationJSON(validatorIndex, second, secondSignature),
	}, nil
}

func VoluntaryExit(key ml_dsa_87.MLDSA87Key, validatorIndex, epoch uint64, chain consensuscontext.Context) (any, error) {
	exit := &qrysmpb.VoluntaryExit{Epoch: primitives.Epoch(epoch), ValidatorIndex: primitives.ValidatorIndex(validatorIndex)}
	signature, err := sign(key, exit, params.BeaconConfig().DomainVoluntaryExit, epoch, chain)
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

func sign(key ml_dsa_87.MLDSA87Key, object fastssz.HashRoot, domainType [4]byte, epoch uint64, chain consensuscontext.Context) ([]byte, error) {
	domain, err := chain.Domain(domainType, epoch)
	if err != nil {
		return nil, err
	}
	root, err := signing.ComputeSigningRoot(object, domain)
	if err != nil {
		return nil, err
	}
	signature := key.Sign(root[:]).Marshal()
	if err := signing.VerifySigningRoot(object, key.PublicKey().Marshal(), signature, domain); err != nil {
		return nil, fmt.Errorf("verify generated validator signature: %w", err)
	}
	return signature, nil
}

func signedHeaderJSON(header *qrysmpb.BeaconBlockHeader, signature []byte) map[string]any {
	return map[string]any{
		"message": map[string]string{
			"slot":           strconv.FormatUint(uint64(header.Slot), 10),
			"proposer_index": strconv.FormatUint(uint64(header.ProposerIndex), 10),
			"parent_root":    hexutil.Encode(header.ParentRoot),
			"state_root":     hexutil.Encode(header.StateRoot),
			"body_root":      hexutil.Encode(header.BodyRoot),
		},
		"signature": hexutil.Encode(signature),
	}
}

func indexedAttestationJSON(validatorIndex uint64, data *qrysmpb.AttestationData, signature []byte) map[string]any {
	return map[string]any{
		"attesting_indices": []string{strconv.FormatUint(validatorIndex, 10)},
		"data": map[string]any{
			"slot":              strconv.FormatUint(uint64(data.Slot), 10),
			"index":             strconv.FormatUint(uint64(data.CommitteeIndex), 10),
			"beacon_block_root": hexutil.Encode(data.BeaconBlockRoot),
			"source": map[string]string{
				"epoch": strconv.FormatUint(uint64(data.Source.Epoch), 10), "root": hexutil.Encode(data.Source.Root),
			},
			"target": map[string]string{
				"epoch": strconv.FormatUint(uint64(data.Target.Epoch), 10), "root": hexutil.Encode(data.Target.Root),
			},
		},
		"signatures": []string{hexutil.Encode(signature)},
	}
}

func attestationData(slot, targetEpoch, sourceEpoch uint64, marker byte) *qrysmpb.AttestationData {
	return &qrysmpb.AttestationData{
		Slot: primitives.Slot(slot), CommitteeIndex: 0, BeaconBlockRoot: rootWithMarker(marker),
		Source: &qrysmpb.Checkpoint{Epoch: primitives.Epoch(sourceEpoch), Root: make([]byte, 32)},
		Target: &qrysmpb.Checkpoint{Epoch: primitives.Epoch(targetEpoch), Root: make([]byte, 32)},
	}
}

func rootWithMarker(marker byte) []byte {
	root := make([]byte, 32)
	root[len(root)-1] = marker
	return root
}
