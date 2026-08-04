package validatorops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/chaincontext"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/client"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/crypto"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/accounts/abi"
	"github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/params"
)

const depositContractABI = `[
  {"anonymous":false,"inputs":[{"indexed":false,"name":"pubkey","type":"bytes"},{"indexed":false,"name":"withdrawal_credentials","type":"bytes"},{"indexed":false,"name":"amount","type":"bytes"},{"indexed":false,"name":"signature","type":"bytes"},{"indexed":false,"name":"index","type":"bytes"}],"name":"DepositEvent","type":"event"},
  {"inputs":[{"name":"pubkey","type":"bytes"},{"name":"withdrawal_credentials","type":"bytes"},{"name":"signature","type":"bytes"},{"name":"deposit_data_root","type":"bytes32"}],"name":"deposit","outputs":[],"stateMutability":"payable","type":"function"}
]`

type depositEvent struct {
	PublicKey             []byte `abi:"pubkey"`
	WithdrawalCredentials []byte `abi:"withdrawal_credentials"`
	Amount                []byte `abi:"amount"`
	Signature             []byte `abi:"signature"`
	Index                 []byte `abi:"index"`
}

type Depositor struct {
	session  *endtoendlive.Session
	contract *bind.BoundContract
	address  common.Address
	domain   [consensuscrypto.RootLength]byte
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
	parsed, err := abi.JSON(strings.NewReader(depositContractABI))
	if err != nil {
		return nil, fmt.Errorf("parse deposit contract ABI: %w", err)
	}
	return &Depositor{
		session:  session,
		contract: bind.NewBoundContract(address, parsed, session.Execution, session.Execution, session.Execution),
		address:  address,
		domain:   chain.DepositDomain(),
	}, nil
}

func (depositor *Depositor) Deposit(
	ctx context.Context,
	key *Key,
	amountInShor uint64,
) (*types.Receipt, error) {
	data, root, err := depositInput(key, depositor.session.Address, amountInShor, depositor.domain)
	if err != nil {
		return nil, err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(depositor.session.Wallet, depositor.session.ChainID)
	if err != nil {
		return nil, err
	}
	auth.Context = ctx
	auth.Value = new(big.Int).Mul(new(big.Int).SetUint64(amountInShor), big.NewInt(params.Shor))
	transaction, err := depositor.contract.Transact(
		auth,
		"deposit",
		data.PublicKey,
		data.WithdrawalCredentials,
		data.Signature,
		root,
	)
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
		var event depositEvent
		if err := depositor.contract.UnpackLog(&event, "DepositEvent", *log); err != nil {
			return nil, fmt.Errorf("decode deposit event: %w", err)
		}
		if !bytes.Equal(event.PublicKey, data.PublicKey) ||
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

func depositInput(
	key *Key,
	withdrawalAddress common.Address,
	amount uint64,
	domain [consensuscrypto.RootLength]byte,
) (consensuscrypto.DepositData, [consensuscrypto.RootLength]byte, error) {
	message := consensuscrypto.DepositMessage{
		PublicKey:             key.PublicKey(),
		WithdrawalCredentials: withdrawalAddress.Bytes(),
		Amount:                amount,
	}
	root, err := message.HashTreeRoot()
	if err != nil {
		return consensuscrypto.DepositData{}, [consensuscrypto.RootLength]byte{}, err
	}
	signingRoot := consensuscrypto.SigningRoot(root, domain)
	signature, err := key.Sign(signingRoot[:])
	if err != nil {
		return consensuscrypto.DepositData{}, [consensuscrypto.RootLength]byte{}, err
	}
	if err := consensuscrypto.Verify(signingRoot, message.PublicKey, signature); err != nil {
		return consensuscrypto.DepositData{}, [consensuscrypto.RootLength]byte{}, fmt.Errorf(
			"verify generated deposit signature: %w",
			err,
		)
	}
	data := consensuscrypto.DepositData{
		PublicKey:             message.PublicKey,
		WithdrawalCredentials: message.WithdrawalCredentials,
		Amount:                message.Amount,
		Signature:             signature,
	}
	dataRoot, err := data.HashTreeRoot()
	return data, dataRoot, err
}
