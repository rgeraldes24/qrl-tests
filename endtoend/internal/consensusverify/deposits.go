// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensusverify

import (
	"fmt"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscrypto"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

func (verification *Verifier) verifyDeposits(items []consensus.DepositOperation) (int, error) {
	domain, err := verification.chain.DepositDomain()
	if err != nil {
		return 0, err
	}
	for index, item := range items {
		data, err := depositData(item.Data)
		if err != nil {
			return index, fmt.Errorf("decode deposit %d: %w", index, err)
		}
		message := &qrysmpb.DepositMessage{
			PublicKey: data.PublicKey, WithdrawalCredentials: data.WithdrawalCredentials, Amount: data.Amount,
		}
		if err := consensuscrypto.VerifySigningRoot(
			message,
			data.PublicKey,
			data.Signature,
			[consensuscrypto.RootLength]byte(domain),
		); err != nil {
			return index, fmt.Errorf("verify deposit %d signature: %w", index, err)
		}
	}
	return len(items), nil
}
