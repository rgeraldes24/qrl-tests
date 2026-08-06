// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensuscrypto

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

const (
	PublicKeyLength             = 2592
	SignatureLength             = 4627
	WithdrawalCredentialsLength = 64
)

type HashRoot interface {
	HashTreeRoot() ([RootLength]byte, error)
}

type BeaconBlockHeader struct {
	Slot          uint64
	ProposerIndex uint64
	ParentRoot    [RootLength]byte
	StateRoot     [RootLength]byte
	BodyRoot      [RootLength]byte
}

func (header BeaconBlockHeader) HashTreeRoot() ([RootLength]byte, error) {
	return containerRoot(
		uint64Root(header.Slot),
		uint64Root(header.ProposerIndex),
		header.ParentRoot,
		header.StateRoot,
		header.BodyRoot,
	), nil
}

type Checkpoint struct {
	Epoch uint64
	Root  [RootLength]byte
}

func (checkpoint Checkpoint) HashTreeRoot() ([RootLength]byte, error) {
	return containerRoot(uint64Root(checkpoint.Epoch), checkpoint.Root), nil
}

type AttestationData struct {
	Slot            uint64
	CommitteeIndex  uint64
	BeaconBlockRoot [RootLength]byte
	Source          Checkpoint
	Target          Checkpoint
}

func (data AttestationData) HashTreeRoot() ([RootLength]byte, error) {
	source, _ := data.Source.HashTreeRoot()
	target, _ := data.Target.HashTreeRoot()
	return containerRoot(
		uint64Root(data.Slot),
		uint64Root(data.CommitteeIndex),
		data.BeaconBlockRoot,
		source,
		target,
	), nil
}

type VoluntaryExit struct {
	Epoch          uint64
	ValidatorIndex uint64
}

func (exit VoluntaryExit) HashTreeRoot() ([RootLength]byte, error) {
	return containerRoot(uint64Root(exit.Epoch), uint64Root(exit.ValidatorIndex)), nil
}

type DepositMessage struct {
	PublicKey             []byte
	WithdrawalCredentials []byte
	Amount                uint64
}

func (message DepositMessage) HashTreeRoot() ([RootLength]byte, error) {
	publicKey, err := fixedBytesRoot("deposit public key", message.PublicKey, PublicKeyLength)
	if err != nil {
		return [RootLength]byte{}, err
	}
	withdrawal, err := fixedBytesRoot(
		"withdrawal credentials",
		message.WithdrawalCredentials,
		WithdrawalCredentialsLength,
	)
	if err != nil {
		return [RootLength]byte{}, err
	}
	return containerRoot(publicKey, withdrawal, uint64Root(message.Amount)), nil
}

type DepositData struct {
	PublicKey             []byte
	WithdrawalCredentials []byte
	Amount                uint64
	Signature             []byte
}

func (data DepositData) HashTreeRoot() ([RootLength]byte, error) {
	publicKey, err := fixedBytesRoot("deposit public key", data.PublicKey, PublicKeyLength)
	if err != nil {
		return [RootLength]byte{}, err
	}
	withdrawal, err := fixedBytesRoot(
		"withdrawal credentials",
		data.WithdrawalCredentials,
		WithdrawalCredentialsLength,
	)
	if err != nil {
		return [RootLength]byte{}, err
	}
	signature, err := fixedBytesRoot("deposit signature", data.Signature, SignatureLength)
	if err != nil {
		return [RootLength]byte{}, err
	}
	return containerRoot(publicKey, withdrawal, uint64Root(data.Amount), signature), nil
}

func uint64Root(value uint64) [RootLength]byte {
	var root [RootLength]byte
	binary.LittleEndian.PutUint64(root[:8], value)
	return root
}

func fixedBytesRoot(name string, value []byte, length int) ([RootLength]byte, error) {
	if len(value) != length {
		return [RootLength]byte{}, fmt.Errorf("%s must be %d bytes, got %d", name, length, len(value))
	}
	chunks := make([][RootLength]byte, (length+RootLength-1)/RootLength)
	for index := range chunks {
		copy(chunks[index][:], value[index*RootLength:])
	}
	return merkleize(chunks), nil
}

func containerRoot(fields ...[RootLength]byte) [RootLength]byte {
	return merkleize(fields)
}

func merkleize(chunks [][RootLength]byte) [RootLength]byte {
	width := 1
	for width < len(chunks) {
		width *= 2
	}
	level := make([][RootLength]byte, width)
	copy(level, chunks)
	for width > 1 {
		for index := 0; index < width; index += 2 {
			var pair [RootLength * 2]byte
			copy(pair[:RootLength], level[index][:])
			copy(pair[RootLength:], level[index+1][:])
			level[index/2] = sha256.Sum256(pair[:])
		}
		width /= 2
		level = level[:width]
	}
	return level[0]
}
