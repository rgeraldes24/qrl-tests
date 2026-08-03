// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensuscrypto

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	walletcommon "github.com/theQRL/go-qrllib/wallet/common"
	walletmldsa "github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
)

func TestComputeDomain(t *testing.T) {
	genesisRoot := [RootLength]byte{0x11}
	for _, test := range []struct {
		name        string
		domainType  [4]byte
		forkVersion [4]byte
		root        [RootLength]byte
		want        string
	}{
		{
			name: "previous fork", domainType: [4]byte{1, 2, 3, 4}, forkVersion: [4]byte{0, 0, 0, 2}, root: genesisRoot,
			want: "010203045f4c4b0ed11ed93379263b2e23b10940f33d3d0aee534c93105e1b58",
		},
		{
			name: "current fork", domainType: [4]byte{1, 2, 3, 4}, forkVersion: [4]byte{0, 0, 0, 3}, root: genesisRoot,
			want: "01020304b89638cec7278d3fffb7ecd79da316be4154b8886a92c206be6c8d33",
		},
		{
			name: "deposit", domainType: DomainDeposit, forkVersion: [4]byte{0, 0, 0, 1},
			want: "0300000018ae4ccbda9538839d79bb18ca09e23e24ae8c1550f56cbb3d84b053",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			domain := ComputeDomain(test.domainType, test.forkVersion, test.root)
			require.Equal(t, test.want, hex.EncodeToString(domain[:]))
		})
	}
}

func TestSigningRoot(t *testing.T) {
	object := [RootLength]byte{}
	copy(object[:], mustDecodeHex(t, strings.Repeat("22", RootLength)))
	domain := [RootLength]byte{}
	copy(domain[:], mustDecodeHex(t, "01020304"+strings.Repeat("33", RootLength-4)))
	root := SigningRoot(object, domain)
	require.Equal(t, "640f12a58208bdf155b1c0f2ab1fd2f071ac7f50897388270b0de5f5ec4ae0d6", hex.EncodeToString(root[:]))
}

func TestVerify(t *testing.T) {
	var seed walletcommon.Seed
	for index := range seed {
		seed[index] = byte(index)
	}
	wallet, err := walletmldsa.NewWalletFromSeed(seed)
	require.NoError(t, err)
	t.Cleanup(wallet.Zeroize)

	message := [RootLength]byte{0x42}
	signature, err := wallet.Sign(message[:])
	require.NoError(t, err)
	publicKey := wallet.GetPK()
	require.NoError(t, Verify(message, publicKey[:], signature[:]))

	message[0] ^= 0xff
	require.Error(t, Verify(message, publicKey[:], signature[:]))
}

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	require.NoError(t, err)
	return decoded
}
