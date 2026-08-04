package validatorops

import (
	"fmt"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/chaincontext"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/client"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/crypto"
	"github.com/theQRL/go-qrl/common/hexutil"
)

func ProposerSlashing(key *Key, validatorIndex, slot uint64, chain consensuscontext.Context) (consensus.ProposerSlashing, error) {
	epoch := chain.Epoch(slot)
	header1 := consensuscrypto.BeaconBlockHeader{
		Slot: slot, ProposerIndex: validatorIndex, BodyRoot: rootWithMarker(1),
	}
	header2 := consensuscrypto.BeaconBlockHeader{
		Slot: slot, ProposerIndex: validatorIndex, BodyRoot: rootWithMarker(2),
	}
	signature1, err := sign(key, header1, consensuscrypto.DomainBeaconProposer, epoch, chain)
	if err != nil {
		return consensus.ProposerSlashing{}, err
	}
	signature2, err := sign(key, header2, consensuscrypto.DomainBeaconProposer, epoch, chain)
	if err != nil {
		return consensus.ProposerSlashing{}, err
	}
	return consensus.ProposerSlashing{
		Header1: signedHeader(header1, signature1),
		Header2: signedHeader(header2, signature2),
	}, nil
}

func AttesterSlashing(key *Key, validatorIndex, slot, finalizedEpoch uint64, chain consensuscontext.Context) (consensus.AttesterSlashing, error) {
	epoch := chain.Epoch(slot)
	first := attestationData(slot, epoch, finalizedEpoch, 1)
	second := attestationData(slot, epoch, finalizedEpoch, 2)
	firstSignature, err := sign(key, first, consensuscrypto.DomainBeaconAttester, epoch, chain)
	if err != nil {
		return consensus.AttesterSlashing{}, err
	}
	secondSignature, err := sign(key, second, consensuscrypto.DomainBeaconAttester, epoch, chain)
	if err != nil {
		return consensus.AttesterSlashing{}, err
	}
	return consensus.AttesterSlashing{
		Attestation1: indexedAttestation(validatorIndex, first, firstSignature),
		Attestation2: indexedAttestation(validatorIndex, second, secondSignature),
	}, nil
}

func VoluntaryExit(key *Key, validatorIndex, epoch uint64, chain consensuscontext.Context) (consensus.SignedVoluntaryExit, error) {
	exit := consensuscrypto.VoluntaryExit{Epoch: epoch, ValidatorIndex: validatorIndex}
	signature, err := sign(key, exit, consensuscrypto.DomainVoluntaryExit, epoch, chain)
	if err != nil {
		return consensus.SignedVoluntaryExit{}, err
	}
	return consensus.SignedVoluntaryExit{
		Message:   consensus.VoluntaryExit{Epoch: epoch, ValidatorIndex: validatorIndex},
		Signature: hexutil.Encode(signature),
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

func signedHeader(header consensuscrypto.BeaconBlockHeader, signature []byte) consensus.SignedBeaconBlockHeader {
	return consensus.SignedBeaconBlockHeader{
		Message: consensus.BeaconBlockHeader{
			Slot:          header.Slot,
			ProposerIndex: header.ProposerIndex,
			ParentRoot:    hexutil.Encode(header.ParentRoot[:]),
			StateRoot:     hexutil.Encode(header.StateRoot[:]),
			BodyRoot:      hexutil.Encode(header.BodyRoot[:]),
		},
		Signature: hexutil.Encode(signature),
	}
}

func indexedAttestation(
	validatorIndex uint64,
	data consensuscrypto.AttestationData,
	signature []byte,
) consensus.IndexedAttestation {
	return consensus.IndexedAttestation{
		AttestingIndices: []uint64{validatorIndex},
		Data: consensus.AttestationData{
			Slot:            data.Slot,
			CommitteeIndex:  data.CommitteeIndex,
			BeaconBlockRoot: hexutil.Encode(data.BeaconBlockRoot[:]),
			Source: consensus.Checkpoint{
				Epoch: data.Source.Epoch,
				Root:  hexutil.Encode(data.Source.Root[:]),
			},
			Target: consensus.Checkpoint{
				Epoch: data.Target.Epoch,
				Root:  hexutil.Encode(data.Target.Root[:]),
			},
		},
		Signatures: []string{hexutil.Encode(signature)},
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
