// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package validatorops

import (
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/crypto"
	"github.com/stretchr/testify/require"
	"github.com/theQRL/go-qrl/common"
)

func TestDepositInput(t *testing.T) {
	key, err := DeterministicKey(0x24)
	require.NoError(t, err)
	domain := consensuscrypto.ComputeDomain(
		consensuscrypto.DomainDeposit,
		[4]byte{1},
		[consensuscrypto.RootLength]byte{},
	)
	data, root, err := depositInput(key, common.Address{1}, 32_000_000_000, domain)
	require.NoError(t, err)
	require.Len(t, data.PublicKey, consensuscrypto.PublicKeyLength)
	require.Len(t, data.WithdrawalCredentials, consensuscrypto.WithdrawalCredentialsLength)
	require.Len(t, data.Signature, consensuscrypto.SignatureLength)

	want, err := data.HashTreeRoot()
	require.NoError(t, err)
	require.Equal(t, want, root)
	message := consensuscrypto.DepositMessage{
		PublicKey: data.PublicKey, WithdrawalCredentials: data.WithdrawalCredentials, Amount: data.Amount,
	}
	require.NoError(t, consensuscrypto.VerifySigningRoot(message, data.PublicKey, data.Signature, domain))
}
