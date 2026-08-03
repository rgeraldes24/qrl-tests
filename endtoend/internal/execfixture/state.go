package execfixture

import (
	"context"
	"fmt"

	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/accounts/abi"
	"github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/core/vm"
)

type StateContract struct {
	Address common.Address
	Topic   common.LogTopic
}

func DeployStateContract(ctx context.Context, session *endtoendlive.Session, topic common.LogTopic) (StateContract, error) {
	auth, err := bind.NewKeyedTransactorWithChainID(session.Wallet, session.ChainID)
	if err != nil {
		return StateContract{}, err
	}
	auth.Context = ctx
	auth.NoSend = true
	auth.GasLimit = 1_000_000
	address, tx, _, err := bind.DeployContract(auth, abi.ABI{}, stateInitCode(topic), session.Execution)
	if err != nil {
		return StateContract{}, err
	}
	receipt, err := SendAndWait(ctx, session.Execution, tx)
	if err != nil {
		return StateContract{}, err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return StateContract{}, fmt.Errorf("state contract deployment failed with status %d", receipt.Status)
	}
	return StateContract{Address: address, Topic: topic}, nil
}

func FullWord(seed byte) common.StorageValue64 {
	var value common.StorageValue64
	for index := range value {
		value[index] = seed + byte(index)
	}
	return value
}

func FullTopic(seed byte) common.LogTopic {
	var topic common.LogTopic
	for index := range topic {
		topic[index] = seed + byte(index)
	}
	return topic
}

func PatternedAddress(seed byte) common.Address {
	var address common.Address
	for index := range address {
		address[index] = seed + byte(index)
	}
	return address
}

func stateInitCode(topic common.LogTopic) []byte {
	runtime := []byte{byte(vm.PUSH1), 0, byte(vm.CALLDATALOAD), byte(vm.DUP1), byte(vm.PUSH1), 0, byte(vm.SSTORE), byte(vm.PUSH1), 0, byte(vm.MSTORE), byte(vm.PUSH64)}
	runtime = append(runtime, topic[:]...)
	runtime = append(runtime,
		byte(vm.PUSH1), byte(vm.WordBytes), byte(vm.PUSH1), 0, byte(vm.LOG1),
		byte(vm.PUSH1), byte(vm.WordBytes), byte(vm.PUSH1), 0, byte(vm.RETURN),
	)

	const headerLength = 12
	code := []byte{
		byte(vm.PUSH1), byte(len(runtime)),
		byte(vm.PUSH1), headerLength,
		byte(vm.PUSH1), 0,
		byte(vm.CODECOPY),
		byte(vm.PUSH1), byte(len(runtime)),
		byte(vm.PUSH1), 0,
		byte(vm.RETURN),
	}
	return append(code, runtime...)
}
