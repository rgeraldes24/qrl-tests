// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"github.com/theQRL/go-qrl/common"
	qrvm "github.com/theQRL/go-qrl/core/vm"
)

func push(data []byte) []byte {
	return newProgram().Push(data).Bytes()
}

func returnTop() []byte {
	return newProgram().
		PushByte(0).
		Op(qrvm.MSTORE).
		PushByte(byte(qrvm.WordBytes)).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
}

func pushCode(width int) ([]byte, []byte) {
	value := make([]byte, width)
	for index := range value {
		value[index] = byte(index + 1)
	}
	code := newProgram().Push(value).Append(returnTop()).Bytes()
	return code, common.LeftPadBytes(value, qrvm.WordBytes)
}

func dupCode(depth int) []byte {
	program := newProgram()
	for value := 1; value <= depth; value++ {
		program.PushByte(byte(value))
	}
	return program.
		Op(qrvm.DUP1 + qrvm.OpCode(depth-1)).
		Append(returnTop()).
		Bytes()
}

func swapCode(depth int) []byte {
	program := newProgram()
	for value := 1; value <= depth+1; value++ {
		program.PushByte(byte(value))
	}
	return program.
		Op(qrvm.SWAP1 + qrvm.OpCode(depth-1)).
		Append(returnTop()).
		Bytes()
}

func memoryCode(value []byte) []byte {
	return memoryCodeAt(value, 0)
}

func memoryCodeAt(value []byte, offset byte) []byte {
	return newProgram().
		Push(value).
		PushByte(offset).
		Op(qrvm.MSTORE).
		PushByte(offset).
		Op(qrvm.MLOAD).
		Append(returnTop()).
		Bytes()
}

func returnWordCode(value []byte) []byte {
	return newProgram().Push(value).Append(returnTop()).Bytes()
}

func echoCalldataCode() []byte {
	return newProgram().
		Op(qrvm.CALLDATASIZE).
		PushByte(0).
		PushByte(0).
		Op(qrvm.CALLDATACOPY, qrvm.CALLDATASIZE).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
}

func calldataLoadCode(offset byte) []byte {
	return newProgram().
		PushByte(offset).
		Op(qrvm.CALLDATALOAD).
		Append(returnTop()).
		Bytes()
}

func codeCopyCode(data []byte) []byte {
	return newProgram().
		PushByte(byte(len(data))).
		PushLabel("data", 2).
		PushByte(0).
		Op(qrvm.CODECOPY).
		PushByte(byte(len(data))).
		PushByte(0).
		Op(qrvm.RETURN).
		Label("data").
		Append(data).
		Bytes()
}

func extCodeCopyCode(target common.Address, size byte) []byte {
	return newProgram().
		PushByte(size).
		PushByte(0).
		PushByte(0).
		PushAddress(target).
		Op(qrvm.EXTCODECOPY).
		PushByte(size).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
}

func returnDataCopyCode(target common.Address) []byte {
	return newProgram().
		PushByte(0).
		PushByte(0).
		PushByte(0).
		PushByte(0).
		PushByte(0).
		PushAddress(target).
		Op(qrvm.GAS, qrvm.CALL, qrvm.POP, qrvm.RETURNDATASIZE).
		PushByte(0).
		PushByte(0).
		Op(qrvm.RETURNDATACOPY, qrvm.RETURNDATASIZE).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
}

func keccakCalldataCode() []byte {
	return newProgram().
		Op(qrvm.CALLDATASIZE).
		PushByte(0).
		PushByte(0).
		Op(qrvm.CALLDATACOPY, qrvm.CALLDATASIZE).
		PushByte(0).
		Op(qrvm.KECCAK256).
		PushByte(0).
		Op(qrvm.MSTORE).
		PushByte(byte(qrvm.WordBytes)).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
}

func staticCallPrecompileCode(address, gas byte) []byte {
	return newProgram().
		Op(qrvm.CALLDATASIZE).
		PushByte(0).
		PushByte(0).
		Op(qrvm.CALLDATACOPY).
		PushByte(32).
		PushByte(0).
		Op(qrvm.CALLDATASIZE).
		PushByte(0).
		PushByte(address).
		PushByte(gas).
		Op(qrvm.STATICCALL).
		PushByte(byte(qrvm.WordBytes)).
		Op(qrvm.MSTORE).
		PushByte(byte(2 * qrvm.WordBytes)).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
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
	program := newProgram()
	for _, operand := range operands {
		program.Push(operand)
	}
	return program.Op(op).Append(returnTop()).Bytes()
}

func storageRoundTripCode(key, value []byte) []byte {
	return newProgram().
		Push(value).
		Push(key).
		Op(qrvm.SSTORE).
		Push(key).
		Op(qrvm.SLOAD).
		Append(returnTop()).
		Bytes()
}

func addressOpcodeCode(op qrvm.OpCode, address common.Address) []byte {
	return newProgram().PushAddress(address).Op(op).Append(returnTop()).Bytes()
}

func opcodeCode(op qrvm.OpCode) []byte {
	return newProgram().Op(op).Append(returnTop()).Bytes()
}

func jumpDestinationCode(width int, embedded bool) []byte {
	data := make([]byte, width)
	data[0] = byte(qrvm.JUMPDEST)
	target := 4 + 1 + width
	if embedded {
		target = 5
	}
	if embedded {
		return newProgram().
			Push([]byte{byte(target >> 8), byte(target)}).
			Op(qrvm.JUMP).
			Push(data).
			Op(qrvm.STOP).
			Bytes()
	}
	return newProgram().
		PushLabel("destination", 2).
		Op(qrvm.JUMP).
		Push(data).
		Label("destination").
		Op(qrvm.JUMPDEST).
		PushByte(1).
		Append(returnTop()).
		Bytes()
}
