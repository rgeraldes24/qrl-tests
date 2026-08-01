package validatorops

import (
	"context"
	"fmt"
	"math/big"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/params"
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
	contract, err := deposit.NewDepositContract(address, session.Client)
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
	receipt, err := bind.WaitMined(ctx, session.Client, transaction)
	if err != nil {
		return nil, err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("deposit transaction %s failed", transaction.Hash())
	}
	return receipt, nil
}
