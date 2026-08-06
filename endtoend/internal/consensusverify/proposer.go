// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensusverify

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/beacon"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscrypto"
)

func (verification *Verifier) verifyBlockHeader(
	ctx context.Context,
	header beacon.BeaconBlockHeader,
	signatureHex,
	rootHex string,
) error {
	message, proposer, err := beaconBlockHeader(header)
	if err != nil {
		return err
	}
	root, err := message.HashTreeRoot()
	if err != nil {
		return err
	}
	wantRoot, err := decodeFixed("block root", rootHex, consensuscrypto.RootLength)
	if err != nil {
		return err
	}
	if !bytes.Equal(root[:], wantRoot) {
		return fmt.Errorf("header root mismatch")
	}
	return verification.verifyObject(ctx, message, proposer, verification.chain.Epoch(message.Slot), consensuscrypto.DomainBeaconProposer, signatureHex)
}

func (verification *Verifier) verifySignedHeader(ctx context.Context, header beacon.SignedBeaconBlockHeader) error {
	message, proposer, err := beaconBlockHeader(header.Message)
	if err != nil {
		return err
	}
	return verification.verifyObject(ctx, message, proposer, verification.chain.Epoch(message.Slot), consensuscrypto.DomainBeaconProposer, header.Signature)
}

func (verification *Verifier) verifyRandao(ctx context.Context, slot, proposer uint64, signatureHex string) error {
	epoch := verification.chain.Epoch(slot)
	value := make([]byte, 32)
	binary.LittleEndian.PutUint64(value, epoch)
	object := consensuscrypto.Root(value)
	return verification.verifyObject(ctx, object, proposer, epoch, consensuscrypto.DomainRandao, signatureHex)
}

func (verification *Verifier) verifyVoluntaryExit(
	ctx context.Context,
	exit beacon.VoluntaryExit,
	signatureHex string,
) error {
	message := consensuscrypto.VoluntaryExit{Epoch: exit.Epoch, ValidatorIndex: exit.ValidatorIndex}
	return verification.verifyObject(
		ctx, message, exit.ValidatorIndex, exit.Epoch, consensuscrypto.DomainVoluntaryExit, signatureHex,
	)
}
