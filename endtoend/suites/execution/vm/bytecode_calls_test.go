// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"github.com/theQRL/go-qrl/common"
	qrvm "github.com/theQRL/go-qrl/core/vm"
)

func revertingStorageCode() []byte {
	return newProgram().
		PushByte(1).
		PushByte(0).
		Op(qrvm.SSTORE).
		PushByte(0).
		PushByte(0).
		Op(qrvm.REVERT).
		Bytes()
}

func failingCallCode(op qrvm.OpCode, target common.Address) []byte {
	program := newProgram().
		PushByte(0).
		PushByte(0).
		PushByte(0).
		PushByte(0)
	if op == qrvm.CALL {
		program.PushByte(7)
	}
	program.
		PushAddress(target).
		Op(qrvm.GAS, op).
		PushByte(0).
		Op(qrvm.MSTORE)
	switch op {
	case qrvm.CALL:
		program.PushAddress(target).Op(qrvm.BALANCE)
	case qrvm.DELEGATECALL:
		program.PushByte(0).Op(qrvm.SLOAD)
	default:
		program.Op(qrvm.RETURNDATASIZE)
	}
	return program.
		PushByte(byte(qrvm.WordBytes)).
		Op(qrvm.MSTORE, qrvm.SELFBALANCE).
		PushByte(byte(2 * qrvm.WordBytes)).
		Op(qrvm.MSTORE).
		PushByte(byte(3 * qrvm.WordBytes)).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
}

func failingCreateCode(op qrvm.OpCode, child common.Address, salt [qrvm.WordBytes]byte) ([]byte, []byte) {
	initCode := revertingStorageCode()
	program := newProgram().
		Push(initCode).
		PushByte(0).
		Op(qrvm.MSTORE)
	if op == qrvm.CREATE2 {
		program.Push(salt[:])
	}
	code := program.
		PushByte(byte(len(initCode))).
		PushByte(byte(qrvm.WordBytes-len(initCode))).
		PushByte(7).
		Op(op).
		PushByte(0).
		Op(qrvm.MSTORE).
		PushAddress(child).
		Op(qrvm.EXTCODESIZE).
		PushByte(byte(qrvm.WordBytes)).
		Op(qrvm.MSTORE).
		PushAddress(child).
		Op(qrvm.BALANCE).
		PushByte(byte(2*qrvm.WordBytes)).
		Op(qrvm.MSTORE, qrvm.SELFBALANCE).
		PushByte(byte(3 * qrvm.WordBytes)).
		Op(qrvm.MSTORE).
		Push([]byte{1, 0}).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
	return code, initCode
}

func callCode(op qrvm.OpCode, target common.Address) []byte {
	return callCodeWithValue(op, target, 0)
}

func callCodeWithValue(op qrvm.OpCode, target common.Address, value byte) []byte {
	program := newProgram().
		PushByte(byte(3 * qrvm.WordBytes)).
		PushByte(0).
		PushByte(0).
		PushByte(0)
	if op == qrvm.CALL {
		program.PushByte(value)
	}
	return program.
		PushAddress(target).
		Op(qrvm.GAS, op).
		PushByte(byte(3 * qrvm.WordBytes)).
		Op(qrvm.MSTORE).
		Push([]byte{1, 0}).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
}

func callContextCode() []byte {
	return newProgram().
		Op(qrvm.ADDRESS).
		PushByte(0).
		Op(qrvm.MSTORE, qrvm.CALLER).
		PushByte(byte(qrvm.WordBytes)).
		Op(qrvm.MSTORE, qrvm.CALLVALUE).
		PushByte(byte(2 * qrvm.WordBytes)).
		Op(qrvm.MSTORE).
		PushByte(byte(3 * qrvm.WordBytes)).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
}

func callValueContextCode() []byte {
	return newProgram().
		Op(qrvm.CALLVALUE).
		PushByte(0).
		Op(qrvm.MSTORE, qrvm.SELFBALANCE).
		PushByte(byte(qrvm.WordBytes)).
		Op(qrvm.MSTORE).
		PushByte(byte(2 * qrvm.WordBytes)).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
}

func createCode(op qrvm.OpCode, value byte, salt [qrvm.WordBytes]byte) ([]byte, []byte) {
	childRuntime := returnWordCode([]byte{0x2a})
	childInit := newProgram().
		Push(childRuntime).
		PushByte(0).
		Op(qrvm.MSTORE).
		PushByte(byte(len(childRuntime))).
		PushByte(byte(qrvm.WordBytes - len(childRuntime))).
		Op(qrvm.RETURN).
		Bytes()
	program := newProgram().
		Push(childInit).
		PushByte(0).
		Op(qrvm.MSTORE)
	if op == qrvm.CREATE2 {
		program.Push(salt[:])
	}
	code := program.
		PushByte(byte(len(childInit))).
		PushByte(byte(qrvm.WordBytes-len(childInit))).
		PushByte(value).
		Op(op, qrvm.DUP1).
		PushByte(byte(qrvm.WordBytes)).
		Op(qrvm.MSTORE, qrvm.DUP1, qrvm.EXTCODESIZE).
		PushByte(0).
		Op(qrvm.MSTORE, qrvm.POP).
		PushByte(byte(qrvm.WordBytes)).
		Op(qrvm.MLOAD, qrvm.BALANCE).
		Push([]byte{1, 0}).
		Op(qrvm.MSTORE).
		PushByte(byte(qrvm.WordBytes)).
		PushByte(byte(2*qrvm.WordBytes)).
		PushByte(0).
		PushByte(0).
		PushByte(0).
		PushByte(byte(qrvm.WordBytes)).
		Op(qrvm.MLOAD, qrvm.GAS, qrvm.CALL).
		PushByte(byte(3 * qrvm.WordBytes)).
		Op(qrvm.MSTORE).
		Push([]byte{1, 64}).
		PushByte(0).
		Op(qrvm.RETURN).
		Bytes()
	return code, childInit
}
