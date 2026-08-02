package validatorops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	beaconparams "github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/contracts/deposit"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
)

func Deposit(ctx context.Context, session *endtoendlive.Session, beacon *consensus.Client, key ml_dsa_87.MLDSA87Key, amountInShor uint64) (*types.Receipt, error) {
	config, err := beacon.DepositContract(ctx)
	if err != nil {
		return nil, err
	}
	address, err := common.NewAddressFromString(config.Address)
	if err != nil {
		return nil, fmt.Errorf("parse deposit contract address: %w", err)
	}
	genesis, err := beacon.Genesis(ctx)
	if err != nil {
		return nil, err
	}
	forkVersion, err := decodeHex(genesis.ForkVersion)
	if err != nil {
		return nil, fmt.Errorf("decode genesis fork version: %w", err)
	}
	data, root, err := deposit.DepositInput(key, session.Address, amountInShor, forkVersion)
	if err != nil {
		return nil, fmt.Errorf("build deposit input: %w", err)
	}
	domain, err := signing.ComputeDomain(beaconparams.BeaconConfig().DomainDeposit, forkVersion, nil)
	if err != nil {
		return nil, fmt.Errorf("build deposit signature domain: %w", err)
	}
	if err := deposit.VerifyDepositSignature(data, domain); err != nil {
		return nil, fmt.Errorf("verify generated deposit signature: %w", err)
	}
	contract, err := deposit.NewDepositContract(address, session.Execution)
	if err != nil {
		return nil, fmt.Errorf("bind deposit contract: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(session.Wallet, session.ChainID)
	if err != nil {
		return nil, err
	}
	auth.Context = ctx
	auth.Value = new(big.Int).Mul(new(big.Int).SetUint64(amountInShor), big.NewInt(params.Shor))
	transaction, err := contract.Deposit(auth, data.PublicKey, data.WithdrawalCredentials, data.Signature, root)
	if err != nil {
		return nil, fmt.Errorf("submit deposit: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, session.Execution, transaction)
	if err != nil {
		return nil, err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("deposit transaction %s failed", transaction.Hash())
	}
	foundEvent := false
	for _, log := range receipt.Logs {
		if log.Address != address {
			continue
		}
		event, err := contract.ParseDepositEvent(*log)
		if err != nil {
			return nil, fmt.Errorf("decode deposit event: %w", err)
		}
		if !bytes.Equal(event.Pubkey, data.PublicKey) ||
			!bytes.Equal(event.WithdrawalCredentials, data.WithdrawalCredentials) ||
			!bytes.Equal(event.Signature, data.Signature) {
			return nil, errors.New("deposit event does not match the signed deposit data")
		}
		foundEvent = true
	}
	if !foundEvent {
		return nil, errors.New("successful deposit receipt has no deposit event")
	}
	return receipt, nil
}
