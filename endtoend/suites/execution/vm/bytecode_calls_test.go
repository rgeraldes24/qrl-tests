// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"github.com/theQRL/go-qrl/common"
	qrvm "github.com/theQRL/go-qrl/core/vm"
)

func revertingStorageCode() []byte {
	return []byte{
		byte(qrvm.PUSH1), 1,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.SSTORE),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.REVERT),
	}
}

func failingCallCode(op qrvm.OpCode, target common.Address) []byte {
	code := []byte{
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
	}
	if op == qrvm.CALL {
		code = append(code, byte(qrvm.PUSH1), 7)
	}
	code = append(code, byte(qrvm.PUSH64))
	code = append(code, target[:]...)
	code = append(code,
		byte(qrvm.GAS),
		byte(op),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
	)
	switch op {
	case qrvm.CALL:
		code = append(code, byte(qrvm.PUSH64))
		code = append(code, target[:]...)
		code = append(code, byte(qrvm.BALANCE))
	case qrvm.DELEGATECALL:
		code = append(code, byte(qrvm.PUSH1), 0, byte(qrvm.SLOAD))
	default:
		code = append(code, byte(qrvm.RETURNDATASIZE))
	}
	return append(code,
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.SELFBALANCE),
		byte(qrvm.PUSH1), byte(2*qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH1), byte(3*qrvm.WordBytes),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	)
}

func failingCreateCode(op qrvm.OpCode, child common.Address, salt [qrvm.WordBytes]byte) ([]byte, []byte) {
	initCode := revertingStorageCode()
	code := append(push(initCode),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
	)
	if op == qrvm.CREATE2 {
		code = append(code, push(salt[:])...)
	}
	code = append(code,
		byte(qrvm.PUSH1), byte(len(initCode)),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes-len(initCode)),
		byte(qrvm.PUSH1), 7,
		byte(op),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH64),
	)
	code = append(code, child[:]...)
	code = append(code,
		byte(qrvm.EXTCODESIZE),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH64),
	)
	code = append(code, child[:]...)
	return append(code,
		byte(qrvm.BALANCE),
		byte(qrvm.PUSH1), byte(2*qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.SELFBALANCE),
		byte(qrvm.PUSH1), byte(3*qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH2), 1, 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	), initCode
}

func callCode(op qrvm.OpCode, target common.Address) []byte {
	return callCodeWithValue(op, target, 0)
}

func callCodeWithValue(op qrvm.OpCode, target common.Address, value byte) []byte {
	code := []byte{
		byte(qrvm.PUSH1), byte(3 * qrvm.WordBytes),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
	}
	if op == qrvm.CALL {
		code = append(code, byte(qrvm.PUSH1), value)
	}
	code = append(code, byte(qrvm.PUSH64))
	code = append(code, target[:]...)
	return append(code,
		byte(qrvm.GAS),
		byte(op),
		byte(qrvm.PUSH1), byte(3*qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH2), 1, 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	)
}

func callContextCode() []byte {
	return []byte{
		byte(qrvm.ADDRESS),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
		byte(qrvm.CALLER),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.CALLVALUE),
		byte(qrvm.PUSH1), byte(2 * qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH1), byte(3 * qrvm.WordBytes),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	}
}

func callValueContextCode() []byte {
	return []byte{
		byte(qrvm.CALLVALUE),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
		byte(qrvm.SELFBALANCE),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH1), byte(2 * qrvm.WordBytes),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	}
}

func createCode(op qrvm.OpCode, value byte, salt [qrvm.WordBytes]byte) ([]byte, []byte) {
	childRuntime := returnWordCode([]byte{0x2a})
	childInit := append(push(childRuntime),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH1), byte(len(childRuntime)),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes-len(childRuntime)),
		byte(qrvm.RETURN),
	)
	code := append(push(childInit),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
	)
	if op == qrvm.CREATE2 {
		code = append(code, push(salt[:])...)
	}
	code = append(code,
		byte(qrvm.PUSH1), byte(len(childInit)),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes-len(childInit)),
		byte(qrvm.PUSH1), value,
		byte(op),
		byte(qrvm.DUP1),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.DUP1),
		byte(qrvm.EXTCODESIZE),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
		byte(qrvm.POP),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.MLOAD),
		byte(qrvm.BALANCE),
		byte(qrvm.PUSH2), 1, 0,
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.PUSH1), byte(2*qrvm.WordBytes),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.MLOAD),
		byte(qrvm.GAS),
		byte(qrvm.CALL),
		byte(qrvm.PUSH1), byte(3*qrvm.WordBytes),
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH2), 1, 64,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	)
	return code, childInit
}
