// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"fmt"

	"github.com/theQRL/go-qrl/common"
	qrvm "github.com/theQRL/go-qrl/core/vm"
)

type labelFixup struct {
	label  string
	offset int
	width  int
}

// program assembles the small QRVM programs used by this package. It only
// handles byte emission, PUSH widths, and label offsets so stack order remains
// explicit in each test fixture.
type program struct {
	code   []byte
	labels map[string]int
	fixups []labelFixup
}

func newProgram() *program {
	return &program{labels: make(map[string]int)}
}

func (program *program) Op(ops ...qrvm.OpCode) *program {
	for _, op := range ops {
		program.code = append(program.code, byte(op))
	}
	return program
}

func (program *program) Push(data []byte) *program {
	if len(data) < 1 || len(data) > qrvm.WordBytes {
		panic(fmt.Sprintf("QRVM PUSH width must be between 1 and %d bytes", qrvm.WordBytes))
	}
	program.code = append(program.code, byte(qrvm.PUSH1)+byte(len(data)-1))
	program.code = append(program.code, data...)
	return program
}

func (program *program) PushByte(value byte) *program {
	return program.Push([]byte{value})
}

func (program *program) PushAddress(address common.Address) *program {
	return program.Push(address[:])
}

func (program *program) PushLabel(label string, width int) *program {
	if width < 1 || width > qrvm.WordBytes {
		panic(fmt.Sprintf("QRVM label width must be between 1 and %d bytes", qrvm.WordBytes))
	}
	program.code = append(program.code, byte(qrvm.PUSH1)+byte(width-1))
	program.fixups = append(program.fixups, labelFixup{label: label, offset: len(program.code), width: width})
	program.code = append(program.code, make([]byte, width)...)
	return program
}

func (program *program) Label(name string) *program {
	if _, exists := program.labels[name]; exists {
		panic("duplicate QRVM label " + name)
	}
	program.labels[name] = len(program.code)
	return program
}

func (program *program) Append(code []byte) *program {
	program.code = append(program.code, code...)
	return program
}

func (program *program) Bytes() []byte {
	code := append([]byte(nil), program.code...)
	for _, fixup := range program.fixups {
		target, exists := program.labels[fixup.label]
		if !exists {
			panic("undefined QRVM label " + fixup.label)
		}
		if fixup.width < 8 && uint64(target) >= uint64(1)<<(8*fixup.width) {
			panic(fmt.Sprintf("QRVM label %q does not fit in %d bytes", fixup.label, fixup.width))
		}
		for index := 0; index < fixup.width; index++ {
			code[fixup.offset+fixup.width-index-1] = byte(target >> (8 * index))
		}
	}
	return code
}
