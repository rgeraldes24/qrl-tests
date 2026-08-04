// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package validatorops

import (
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/crypto"
	"github.com/stretchr/testify/require"
)

func TestDeterministicKeySignsConsensusRoots(t *testing.T) {
	key, err := DeterministicKey(0x42)
	require.NoError(t, err)
	require.Len(t, key.PublicKey(), consensuscrypto.PublicKeyLength)

	root := consensuscrypto.SigningRoot(
		[consensuscrypto.RootLength]byte{1},
		[consensuscrypto.RootLength]byte{2},
	)
	signature, err := key.Sign(root[:])
	require.NoError(t, err)
	require.Len(t, signature, consensuscrypto.SignatureLength)
	require.NoError(t, consensuscrypto.Verify(root, key.PublicKey(), signature))
}
