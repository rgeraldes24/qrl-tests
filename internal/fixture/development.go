// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package fixture

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/theQRL/go-qrl/common"
	qrlwallet "github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
)

//go:embed testdata/unsafe-development-wallet.seed
var developmentWalletSeed string

var DevelopmentWalletAddress = developmentWalletAddress()

func DevelopmentWallet() (qrlwallet.Wallet, error) {
	wallet, err := qrlwallet.RestoreFromSeedHex(strings.TrimSpace(developmentWalletSeed))
	if err != nil {
		return nil, fmt.Errorf("restore development wallet: %w", err)
	}
	return wallet, nil
}

func developmentWalletAddress() string {
	wallet, err := DevelopmentWallet()
	if err != nil {
		panic(err)
	}
	return common.Address(wallet.GetAddress()).Hex()
}
