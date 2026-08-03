// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"math/big"

	qrvm "github.com/theQRL/go-qrl/core/vm"
)

type operationCase struct {
	name     string
	op       qrvm.OpCode
	operands [][]byte
	want     *big.Int
}

func vm64OperationCases() []operationCase {
	modulus := new(big.Int).Lsh(big.NewInt(1), qrvm.WordBits)
	max := new(big.Int).Sub(new(big.Int).Set(modulus), big.NewInt(1))
	minSigned := new(big.Int).Lsh(big.NewInt(1), qrvm.WordBits-1)
	mulLeft := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 300), big.NewInt(1))
	mulRight := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 220), big.NewInt(1))
	dividend := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 400), big.NewInt(21))
	divisor := big.NewInt(7)
	negativeDividend := new(big.Int).Neg(
		new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 400), big.NewInt(19)),
	)
	modDivisor := big.NewInt(17)
	modLeft := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 400), big.NewInt(123))
	modRight := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 300), big.NewInt(456))
	modulus500 := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 500), big.NewInt(159))
	zero := []byte{0}
	one := []byte{1}
	shift511 := new(big.Int).SetUint64(qrvm.WordBits - 1).Bytes()

	cases := []operationCase{
		{"ADD wraps at 512 bits", qrvm.ADD, [][]byte{vmWord(max), one}, new(big.Int)},
		{"SUB wraps at 512 bits", qrvm.SUB, [][]byte{one, zero}, max},
		{
			"MUL wraps at 512 bits",
			qrvm.MUL,
			[][]byte{vmWord(mulLeft), vmWord(mulRight)},
			vmUnsigned(new(big.Int).Mul(mulLeft, mulRight), modulus),
		},
		{
			"DIV uses the upper 256 bits",
			qrvm.DIV,
			[][]byte{vmWord(divisor), vmWord(dividend)},
			new(big.Int).Div(dividend, divisor),
		},
		{
			"SDIV preserves a negative upper-half quotient",
			qrvm.SDIV,
			[][]byte{vmWord(divisor), vmWord(vmUnsigned(negativeDividend, modulus))},
			vmUnsigned(new(big.Int).Quo(negativeDividend, divisor), modulus),
		},
		{
			"SDIV preserves the signed minimum divided by minus one",
			qrvm.SDIV,
			[][]byte{vmWord(max), vmWord(minSigned)},
			minSigned,
		},
		{
			"MOD uses the upper 256 bits",
			qrvm.MOD,
			[][]byte{vmWord(modDivisor), vmWord(dividend)},
			new(big.Int).Mod(dividend, modDivisor),
		},
		{
			"SMOD preserves a negative remainder",
			qrvm.SMOD,
			[][]byte{vmWord(modDivisor), vmWord(vmUnsigned(negativeDividend, modulus))},
			vmUnsigned(new(big.Int).Rem(negativeDividend, modDivisor), modulus),
		},
		{
			"ADDMOD uses a 512-bit modulus",
			qrvm.ADDMOD,
			[][]byte{vmWord(modulus500), vmWord(modRight), vmWord(modLeft)},
			new(big.Int).Mod(new(big.Int).Add(modLeft, modRight), modulus500),
		},
		{
			"MULMOD preserves the full intermediate product",
			qrvm.MULMOD,
			[][]byte{vmWord(modulus500), vmWord(modRight), vmWord(modLeft)},
			new(big.Int).Mod(new(big.Int).Mul(modLeft, modRight), modulus500),
		},
		{
			"EXP reaches the upper 256 bits",
			qrvm.EXP,
			[][]byte{big.NewInt(400).Bytes(), []byte{2}},
			new(big.Int).Lsh(big.NewInt(1), 400),
		},
		{"SLT compares the signed minimum", qrvm.SLT, [][]byte{zero, vmWord(minSigned)}, big.NewInt(1)},
		{"SGT compares minus one", qrvm.SGT, [][]byte{vmWord(max), zero}, big.NewInt(1)},
		{"SHL reaches bit 511", qrvm.SHL, [][]byte{one, shift511}, minSigned},
		{"SHR crosses the upper 256 bits", qrvm.SHR, [][]byte{vmWord(minSigned), shift511}, big.NewInt(1)},
		{"SAR preserves the sign", qrvm.SAR, [][]byte{vmWord(minSigned), shift511}, max},
	}

	upperLeft := new(big.Int).Or(
		new(big.Int).Lsh(big.NewInt(1), 400),
		big.NewInt(0x55),
	)
	upperRight := new(big.Int).Or(
		new(big.Int).Lsh(big.NewInt(1), 300),
		big.NewInt(0xaa),
	)
	cases = append(cases,
		operationCase{"LT compares upper-half values", qrvm.LT, [][]byte{vmWord(upperLeft), vmWord(upperRight)}, big.NewInt(1)},
		operationCase{"GT compares upper-half values", qrvm.GT, [][]byte{vmWord(upperRight), vmWord(upperLeft)}, big.NewInt(1)},
		operationCase{"EQ compares complete words", qrvm.EQ, [][]byte{vmWord(upperLeft), vmWord(upperLeft)}, big.NewInt(1)},
		operationCase{"ISZERO rejects an upper-half bit", qrvm.ISZERO, [][]byte{vmWord(upperLeft)}, new(big.Int)},
		operationCase{"AND preserves upper-half bits", qrvm.AND, [][]byte{vmWord(upperLeft), vmWord(upperRight)}, new(big.Int).And(upperLeft, upperRight)},
		operationCase{"OR preserves upper-half bits", qrvm.OR, [][]byte{vmWord(upperLeft), vmWord(upperRight)}, new(big.Int).Or(upperLeft, upperRight)},
		operationCase{"XOR preserves upper-half bits", qrvm.XOR, [][]byte{vmWord(upperLeft), vmWord(upperRight)}, new(big.Int).Xor(upperLeft, upperRight)},
		operationCase{"NOT flips all 512 bits", qrvm.NOT, [][]byte{vmWord(upperLeft)}, new(big.Int).Xor(upperLeft, max)},
	)

	pattern := patternedBytes(qrvm.WordBytes)
	for _, test := range []struct {
		index byte
		want  byte
	}{{0, pattern[0]}, {qrvm.WordBytes - 1, pattern[qrvm.WordBytes-1]}, {qrvm.WordBytes, 0}} {
		cases = append(cases, operationCase{
			name:     "BYTE index " + new(big.Int).SetUint64(uint64(test.index)).String(),
			op:       qrvm.BYTE,
			operands: [][]byte{pattern, {test.index}},
			want:     new(big.Int).SetUint64(uint64(test.want)),
		})
	}

	for _, index := range []uint{31, 32} {
		signBit := index*8 + 7
		value := new(big.Int).Lsh(big.NewInt(1), signBit)
		upper := new(big.Int).Sub(
			new(big.Int).Set(modulus),
			new(big.Int).Lsh(big.NewInt(1), signBit+1),
		)
		cases = append(cases, operationCase{
			name:     "SIGNEXTEND byte " + new(big.Int).SetUint64(uint64(index)).String(),
			op:       qrvm.SIGNEXTEND,
			operands: [][]byte{vmWord(value), {byte(index)}},
			want:     new(big.Int).Or(value, upper),
		})
	}
	cases = append(cases, operationCase{
		name:     "SIGNEXTEND preserves the signed minimum",
		op:       qrvm.SIGNEXTEND,
		operands: [][]byte{vmWord(minSigned), {qrvm.WordBytes - 1}},
		want:     minSigned,
	})
	return cases
}
