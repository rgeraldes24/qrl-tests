package validatorops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscontext"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/params"
	"github.com/theQRL/qrysm/contracts/deposit"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
)

type Depositor struct {
	session     *endtoendlive.Session
	contract    *deposit.DepositContract
	address     common.Address
	forkVersion [4]byte
	domain      []byte
}

func NewDepositor(
	ctx context.Context,
	session *endtoendlive.Session,
	beacon *consensus.Client,
	chain consensuscontext.Context,
) (*Depositor, error) {
	config, err := beacon.DepositContract(ctx)
	if err != nil {
		return nil, err
	}
	address, err := common.NewAddressFromString(config.Address)
	if err != nil {
		return nil, fmt.Errorf("parse deposit contract address: %w", err)
	}
	contract, err := deposit.NewDepositContract(address, session.Execution)
	if err != nil {
		return nil, fmt.Errorf("bind deposit contract: %w", err)
	}
	domain, err := chain.DepositDomain()
	if err != nil {
		return nil, fmt.Errorf("build deposit signature domain: %w", err)
	}
	return &Depositor{
		session:     session,
		contract:    contract,
		address:     address,
		forkVersion: chain.GenesisForkVersion(),
		domain:      domain,
	}, nil
}

func (depositor *Depositor) Deposit(
	ctx context.Context,
	key ml_dsa_87.MLDSA87Key,
	amountInShor uint64,
) (*types.Receipt, error) {
	data, root, err := deposit.DepositInput(key, depositor.session.Address, amountInShor, depositor.forkVersion[:])
	if err != nil {
		return nil, fmt.Errorf("build deposit input: %w", err)
	}
	if err := deposit.VerifyDepositSignature(data, depositor.domain); err != nil {
		return nil, fmt.Errorf("verify generated deposit signature: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(depositor.session.Wallet, depositor.session.ChainID)
	if err != nil {
		return nil, err
	}
	auth.Context = ctx
	auth.Value = new(big.Int).Mul(new(big.Int).SetUint64(amountInShor), big.NewInt(params.Shor))
	transaction, err := depositor.contract.Deposit(auth, data.PublicKey, data.WithdrawalCredentials, data.Signature, root)
	if err != nil {
		return nil, fmt.Errorf("submit deposit: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, depositor.session.Execution, transaction)
	if err != nil {
		return nil, err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("deposit transaction %s failed", transaction.Hash())
	}
	foundEvent := false
	for _, log := range receipt.Logs {
		if log.Address != depositor.address {
			continue
		}
		event, err := depositor.contract.ParseDepositEvent(*log)
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
