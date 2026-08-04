// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package clef

import (
	"context"
	"fmt"
	"slices"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/rpc"
	signercore "github.com/theQRL/go-qrl/signer/core"
)

func verifyAccountListing(ctx context.Context, client *rpc.Client, account common.Address) error {
	var listedAccounts []common.Address
	if err := callRPC(ctx, client, &listedAccounts, "account_list"); err != nil {
		return err
	}
	if len(listedAccounts) != 1 || listedAccounts[0] != account {
		return fmt.Errorf("account_list returned %v, want [%s]", listedAccounts, account.Hex())
	}
	return nil
}

func verifyVersion(ctx context.Context, client *rpc.Client) error {
	var version string
	if err := callRPC(ctx, client, &version, "account_version"); err != nil {
		return err
	}
	if version != signercore.ExternalAPIVersion {
		return fmt.Errorf("account_version returned %q, want %q", version, signercore.ExternalAPIVersion)
	}
	return nil
}

func verifyNewAccount(ctx context.Context, session *clefSession) (common.Address, error) {
	var account common.Address
	if err := callRPC(ctx, session.client, &account, "account_new"); err != nil {
		return common.Address{}, err
	}
	if account == (common.Address{}) || account == session.account {
		return common.Address{}, fmt.Errorf("account_new returned invalid address %s", account.Hex())
	}
	if err := verifyAccountPresent(ctx, session.client, account); err != nil {
		return common.Address{}, err
	}
	return account, nil
}

func verifyAccountPresent(ctx context.Context, client *rpc.Client, account common.Address) error {
	var listedAccounts []common.Address
	if err := callRPC(ctx, client, &listedAccounts, "account_list"); err != nil {
		return err
	}
	if !slices.Contains(listedAccounts, account) {
		return fmt.Errorf("account_list does not contain new account %s", account.Hex())
	}
	return nil
}
