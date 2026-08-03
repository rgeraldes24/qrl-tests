// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensusverify

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscrypto"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

func (verification *Verifier) verifyBlockHeader(
	ctx context.Context,
	header consensus.BeaconBlockHeader,
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
	return verification.verifyObject(ctx, message, proposer, verification.chain.Epoch(uint64(message.Slot)), consensuscrypto.DomainBeaconProposer, signatureHex)
}

func (verification *Verifier) verifySignedHeader(ctx context.Context, header consensus.SignedBeaconBlockHeader) error {
	message, proposer, err := beaconBlockHeader(header.Message)
	if err != nil {
		return err
	}
	return verification.verifyObject(ctx, message, proposer, verification.chain.Epoch(uint64(message.Slot)), consensuscrypto.DomainBeaconProposer, header.Signature)
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
	exit consensus.VoluntaryExit,
	signatureHex string,
) error {
	message := &qrysmpb.VoluntaryExit{
		Epoch: primitives.Epoch(exit.Epoch), ValidatorIndex: primitives.ValidatorIndex(exit.ValidatorIndex),
	}
	return verification.verifyObject(
		ctx, message, exit.ValidatorIndex, exit.Epoch, consensuscrypto.DomainVoluntaryExit, signatureHex,
	)
}
