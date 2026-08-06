// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensusverify

import (
	"fmt"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/beacon"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscrypto"
)

func (verification *Verifier) verifyDeposits(items []beacon.DepositOperation) (int, error) {
	domain := verification.chain.DepositDomain()
	for index, item := range items {
		data, err := depositData(item.Data)
		if err != nil {
			return index, fmt.Errorf("decode deposit %d: %w", index, err)
		}
		message := consensuscrypto.DepositMessage{
			PublicKey: data.PublicKey, WithdrawalCredentials: data.WithdrawalCredentials, Amount: data.Amount,
		}
		if err := consensuscrypto.VerifySigningRoot(
			message,
			data.PublicKey,
			data.Signature,
			domain,
		); err != nil {
			return index, fmt.Errorf("verify deposit %d signature: %w", index, err)
		}
	}
	return len(items), nil
}
