// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"github.com/theQRL/go-qrl/common"
	qrvm "github.com/theQRL/go-qrl/core/vm"
)

func logInitCode(data []byte, topics []common.LogTopic) []byte {
	code := append(push(data),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
	)
	for index := len(topics) - 1; index >= 0; index-- {
		code = append(code, byte(qrvm.PUSH64))
		code = append(code, topics[index][:]...)
	}
	return append(code,
		byte(qrvm.PUSH1), byte(qrvm.WordBytes),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.LOG0)+byte(len(topics)),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.MSTORE),
		byte(qrvm.PUSH1), 1,
		byte(qrvm.PUSH1), byte(qrvm.WordBytes-1),
		byte(qrvm.RETURN),
	)
}

func runtimeInitCode(runtime []byte) []byte {
	code := []byte{
		byte(qrvm.PUSH2), byte(len(runtime) >> 8), byte(len(runtime)),
		byte(qrvm.PUSH2), 0, 0,
		byte(qrvm.PUSH1), 0,
		byte(qrvm.CODECOPY),
		byte(qrvm.PUSH2), byte(len(runtime) >> 8), byte(len(runtime)),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.RETURN),
	}
	offset := len(code)
	code[4] = byte(offset >> 8)
	code[5] = byte(offset)
	return append(code, runtime...)
}

func minedStructuralCode() ([]byte, int) {
	code := []byte{
		byte(qrvm.PUSH0),
		byte(qrvm.ISZERO),
		byte(qrvm.PUSH1), 0,
		byte(qrvm.SSTORE),
		byte(qrvm.CODESIZE),
		byte(qrvm.PUSH1), 1,
		byte(qrvm.SSTORE),
	}
	pc := len(code)
	code = append(code,
		byte(qrvm.PC),
		byte(qrvm.PUSH1), 2,
		byte(qrvm.SSTORE),
		byte(qrvm.PUSH1), 0x7f,
		byte(qrvm.PUSH1), byte(qrvm.WordBytes-1),
		byte(qrvm.MSTORE8),
		byte(qrvm.MSIZE),
		byte(qrvm.PUSH1), 3,
		byte(qrvm.SSTORE),
		byte(qrvm.PUSH1), 1,
		byte(qrvm.PUSH2), 0, 0,
		byte(qrvm.JUMPI),
		byte(qrvm.INVALID),
	)
	destination := len(code)
	code[len(code)-4] = byte(destination >> 8)
	code[len(code)-3] = byte(destination)
	code = append(code,
		byte(qrvm.JUMPDEST),
		byte(qrvm.PUSH1), 1,
		byte(qrvm.PUSH1), 4,
		byte(qrvm.SSTORE),
		byte(qrvm.STOP),
	)
	return code, pc
}
