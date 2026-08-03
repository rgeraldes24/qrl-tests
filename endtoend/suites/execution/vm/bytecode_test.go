// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"github.com/theQRL/go-qrl/common"
	qrvm "github.com/theQRL/go-qrl/core/vm"
)

func push(data []byte) []byte {
	code := []byte{byte(qrvm.PUSH1) + byte(len(data)-1)}
	return append(code, data...)
}

func returnTop() []byte {
	return []byte{
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	}
}

func pushCode(width int) ([]byte, []byte) {
	value := make([]byte, width)
	for index := range value {
		value[index] = byte(index + 1)
	}
	code := append(push(value), returnTop()...)
	return code, common.LeftPadBytes(value, qrvm.WordBytes)
}

func dupCode(depth int) []byte {
	code := make([]byte, 0, depth*2+8)
	for value := 1; value <= depth; value++ {
		code = append(code, byte(qrvm.PUSH1), byte(value))
	}
	code = append(code, byte(qrvm.DUP1)+byte(depth-1))
	return append(code, returnTop()...)
}

func swapCode(depth int) []byte {
	code := make([]byte, 0, (depth+1)*2+8)
	for value := 1; value <= depth+1; value++ {
		code = append(code, byte(qrvm.PUSH1), byte(value))
	}
	code = append(code, byte(qrvm.SWAP1)+byte(depth-1))
	return append(code, returnTop()...)
}

func memoryCode(value []byte) []byte {
	return memoryCodeAt(value, 0)
}

func memoryCodeAt(value []byte, offset byte) []byte {
	code := append(push(value),
		byte(qrvm.PUSH1), offset,
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH1), offset,
		byte(qrvm.MLOAD),
	)
	return append(code, returnTop()...)
}

func returnWordCode(value []byte) []byte {
	return append(push(value), returnTop()...)
}

func echoCalldataCode() []byte {
	return []byte{
		byte(qrvm.CALLDATASIZE),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.CALLDATACOPY),
		byte(qrvm.CALLDATASIZE),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	}
}

func calldataLoadCode(offset byte) []byte {
	return append([]byte{
		byte(qrvm.PUSH1), offset,
		byte(qrvm.CALLDATALOAD),
	}, returnTop()...)
}

func codeCopyCode(data []byte) []byte {
	code := []byte{
		byte(qrvm.PUSH1), byte(len(data)),
		byte(qrvm.PUSH2), 0, 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.CODECOPY),
		byte(qrvm.PUSH1), byte(len(data)),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	}
	dataOffset := len(code)
	code[3] = byte(dataOffset >> 8)
	code[4] = byte(dataOffset)
	return append(code, data...)
}

func extCodeCopyCode(target common.Address, size byte) []byte {
	code := []byte{
		byte(qrvm.PUSH1), size,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH64),
	}
	code = append(code, target[:]...)
	return append(code,
		byte(qrvm.EXTCODECOPY),
		byte(qrvm.PUSH1), size,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	)
}

func returnDataCopyCode(target common.Address) []byte {
	code := []byte{
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH64),
	}
	code = append(code, target[:]...)
	return append(code,
		byte(qrvm.GAS),
		byte(qrvm.CALL),
		byte(qrvm.POP),
		byte(qrvm.RETURNDATASIZE),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURNDATACOPY),
		byte(qrvm.RETURNDATASIZE),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	)
}

func keccakCalldataCode() []byte {
	return []byte{
		byte(qrvm.CALLDATASIZE),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.CALLDATACOPY),
		byte(qrvm.CALLDATASIZE),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.KECCAK256),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	}
}

func staticCallPrecompileCode(address, gas byte) []byte {
	return []byte{
		byte(qrvm.CALLDATASIZE),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.CALLDATACOPY),
		byte(qrvm.PUSH1), 32,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.CALLDATASIZE),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), address,
		byte(qrvm.PUSH1), gas,
		byte(qrvm.STATICCALL),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH1), byte(2 * qrvm.WordBytes),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	}
}

func patternedBytes(size int) []byte {
	data := make([]byte, size)
	for index := range data {
		data[index] = byte(index + 1)
	}
	return data
}

func patternedAddress(lastByte byte) common.Address {
	address := common.BytesToAddress(patternedBytes(common.AddressLength))
	address[common.AddressLength-1] = lastByte
	return address
}

func patternedCreate2Salt() [qrvm.WordBytes]byte {
	var salt [qrvm.WordBytes]byte
	copy(salt[:], patternedBytes(len(salt)))
	return salt
}

func operationCode(op qrvm.OpCode, operands ...[]byte) []byte {
	var code []byte
	for _, operand := range operands {
		code = append(code, push(operand)...)
	}
	code = append(code, byte(op))
	return append(code, returnTop()...)
}

func storageRoundTripCode(key, value []byte) []byte {
	code := append(push(value), push(key)...)
	code = append(code, byte(qrvm.SSTORE))
	code = append(code, push(key)...)
	code = append(code, byte(qrvm.SLOAD))
	return append(code, returnTop()...)
}

func addressOpcodeCode(op qrvm.OpCode, address common.Address) []byte {
	code := []byte{byte(qrvm.PUSH64)}
	code = append(code, address[:]...)
	code = append(code, byte(op))
	return append(code, returnTop()...)
}

func opcodeCode(op qrvm.OpCode) []byte {
	return append([]byte{byte(op)}, returnTop()...)
}

func jumpDestinationCode(width int, embedded bool) []byte {
	data := make([]byte, width)
	data[0] = byte(qrvm.JUMPDEST)
	target := 4 + 1 + width
	if embedded {
		target = 5
	}
	code := []byte{
		byte(qrvm.PUSH2), byte(target >> 8), byte(target),
		byte(qrvm.JUMP),
		byte(qrvm.PUSH1) + byte(width-1),
	}
	code = append(code, data...)
	if embedded {
		return append(code, byte(qrvm.STOP))
	}
	code = append(code, byte(qrvm.JUMPDEST), byte(qrvm.PUSH1), 1)
	return append(code, returnTop()...)
}
