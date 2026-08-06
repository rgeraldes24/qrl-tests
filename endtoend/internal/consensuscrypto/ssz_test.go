// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensuscrypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsensusObjectRoots(t *testing.T) {
	for _, test := range []struct {
		name string
		item HashRoot
		want string
	}{
		{
			name: "block header",
			item: BeaconBlockHeader{
				Slot: 17, ProposerIndex: 9,
				ParentRoot: array32(1), StateRoot: array32(2), BodyRoot: array32(3),
			},
			want: "c3edabb321e528bd7b0f7298e598426bfe07d78cb58d9767aa56b9b16acf2ed8",
		},
		{
			name: "attestation data",
			item: AttestationData{
				Slot: 17, CommitteeIndex: 2, BeaconBlockRoot: array32(4),
				Source: Checkpoint{Epoch: 1, Root: array32(5)},
				Target: Checkpoint{Epoch: 2, Root: array32(6)},
			},
			want: "a68feecf7f23cd791e5efe05f52974f07e77152d84b0976c1d8e0667953f99be",
		},
		{
			name: "voluntary exit",
			item: VoluntaryExit{Epoch: 7, ValidatorIndex: 11},
			want: "1d1caf48c76264b53d945a6cf0224747c7216d2676bcec350a293a8decea3cab",
		},
		{
			name: "deposit message",
			item: DepositMessage{
				PublicKey:             sequence(PublicKeyLength, 7),
				WithdrawalCredentials: sequence(WithdrawalCredentialsLength, 8),
				Amount:                32_000_000_000,
			},
			want: "b355666a37b1fca0d247ed048e61d4213bbb56425e58f46004e1a46aa61d7e6d",
		},
		{
			name: "deposit data",
			item: DepositData{
				PublicKey:             sequence(PublicKeyLength, 7),
				WithdrawalCredentials: sequence(WithdrawalCredentialsLength, 8),
				Amount:                32_000_000_000,
				Signature:             sequence(SignatureLength, 9),
			},
			want: "8bf200ce2405c62d50f450d4bed8d0fdb6e885b5087db254c7bcbb6c88a5febd",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, err := test.item.HashTreeRoot()
			require.NoError(t, err)
			require.Equal(t, test.want, hex.EncodeToString(root[:]))
		})
	}
}

func array32(marker byte) [RootLength]byte {
	var result [RootLength]byte
	copy(result[:], sequence(RootLength, marker))
	return result
}

func sequence(length int, marker byte) []byte {
	result := make([]byte, length)
	for index := range result {
		result[index] = marker + byte(index)
	}
	return result
}
