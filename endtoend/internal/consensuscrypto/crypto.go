// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

// Package consensuscrypto implements the protocol-level domain and signing
// root formulas used to independently verify QRL consensus signatures.
package consensuscrypto

import (
	"crypto/sha256"
	"errors"
	"fmt"

	ssz "github.com/prysmaticlabs/fastssz"
	walletcommon "github.com/theQRL/go-qrllib/wallet/common"
	walletmldsa "github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
)

const (
	RootLength       = sha256.Size
	PublicKeyLength  = walletmldsa.PKSize
	SignatureLength  = walletmldsa.SigSize
	FeeRecipientSize = walletcommon.AddressSize
)

var (
	DomainBeaconProposer = [4]byte{0x00, 0x00, 0x00, 0x00}
	DomainBeaconAttester = [4]byte{0x01, 0x00, 0x00, 0x00}
	DomainRandao         = [4]byte{0x02, 0x00, 0x00, 0x00}
	DomainDeposit        = [4]byte{0x03, 0x00, 0x00, 0x00}
	DomainVoluntaryExit  = [4]byte{0x04, 0x00, 0x00, 0x00}
	DomainSyncCommittee  = [4]byte{0x07, 0x00, 0x00, 0x00}
)

type Root [RootLength]byte

func (root Root) HashTreeRoot() ([RootLength]byte, error) {
	return root, nil
}

func (root Root) HashTreeRootWith(hasher *ssz.Hasher) error {
	index := hasher.Index()
	hasher.PutBytes(root[:])
	hasher.Merkleize(index)
	return nil
}

func ComputeDomain(domainType [4]byte, forkVersion [4]byte, genesisValidatorsRoot [RootLength]byte) [RootLength]byte {
	var forkData [RootLength * 2]byte
	copy(forkData[:4], forkVersion[:])
	copy(forkData[RootLength:], genesisValidatorsRoot[:])
	forkDataRoot := sha256.Sum256(forkData[:])

	var domain [RootLength]byte
	copy(domain[:4], domainType[:])
	copy(domain[4:], forkDataRoot[:RootLength-4])
	return domain
}

func SigningRoot(objectRoot, domain [RootLength]byte) [RootLength]byte {
	var signingData [RootLength * 2]byte
	copy(signingData[:RootLength], objectRoot[:])
	copy(signingData[RootLength:], domain[:])
	return sha256.Sum256(signingData[:])
}

func VerifySigningRoot(object ssz.HashRoot, publicKey, signature []byte, domain [RootLength]byte) error {
	objectRoot, err := object.HashTreeRoot()
	if err != nil {
		return fmt.Errorf("compute object root: %w", err)
	}
	return Verify(SigningRoot(objectRoot, domain), publicKey, signature)
}

func Verify(message [RootLength]byte, publicKey, signature []byte) error {
	key, err := walletmldsa.BytesToPK(publicKey)
	if err != nil {
		return err
	}
	if len(signature) != SignatureLength {
		return fmt.Errorf("signature must be %d bytes", SignatureLength)
	}
	descriptor, err := walletmldsa.NewMLDSA87Descriptor()
	if err != nil {
		return err
	}
	if !walletmldsa.Verify(message[:], signature, &key, descriptor.ToDescriptor()) {
		return errors.New("signature did not verify")
	}
	return nil
}
