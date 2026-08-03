// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"math/big"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	qrvm "github.com/theQRL/go-qrl/core/vm"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerWordOperationSpecs() {
	ginkgo.It("executes PUSH33 through PUSH64", func(ctx ginkgo.SpecContext) {
		for width := 33; width <= qrvm.WordBytes; width++ {
			code, want := pushCode(width)
			gomega.Expect(vmSuite.callCode(ctx, code, nil)).To(gomega.Equal(want))
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout))

	ginkgo.It("executes shifted DUP and SWAP ranges", func(ctx ginkgo.SpecContext) {
		for depth := 1; depth <= 16; depth++ {
			gomega.Expect(vmSuite.callCode(ctx, dupCode(depth), nil)).To(
				gomega.Equal(common.LeftPadBytes([]byte{1}, qrvm.WordBytes)),
			)
			gomega.Expect(vmSuite.callCode(ctx, swapCode(depth), nil)).To(
				gomega.Equal(common.LeftPadBytes([]byte{1}, qrvm.WordBytes)),
			)
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout))

	for _, test := range vm64OperationCases() {
		test := test
		ginkgo.It("executes "+test.name, func(ctx ginkgo.SpecContext) {
			gomega.Expect(vmSuite.callCode(ctx, operationCode(test.op, test.operands...), nil)).To(
				gomega.Equal(vmWord(test.want)),
			)
		}, ginkgo.SpecTimeout(liveSpecTimeout))
	}
}

func registerJumpAndMinedSpecs() {
	ginkgo.It("does not treat JUMPDEST bytes inside PUSH33 through PUSH64 as code", func(ctx ginkgo.SpecContext) {
		for width := 33; width <= qrvm.WordBytes; width++ {
			_, err := vmSuite.callCodeResult(ctx, jumpDestinationCode(width, true), nil, nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(vmSuite.callCode(ctx, jumpDestinationCode(width, false), nil)).To(
				gomega.Equal(vmWord(big.NewInt(1))),
			)
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout))

	ginkgo.It("mines structural opcode state changes", func(ctx ginkgo.SpecContext) {
		runtime, expectedPC := minedStructuralCode()
		contract := vmSuite.deployRuntime(ctx, runtime)
		receipt := vmSuite.mineCall(ctx, contract)
		gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))

		for slot, want := range []*big.Int{
			big.NewInt(1),
			big.NewInt(int64(len(runtime))),
			big.NewInt(int64(expectedPC)),
			big.NewInt(qrvm.WordBytes),
			big.NewInt(1),
		} {
			key := common.Hash{}
			key[len(key)-1] = byte(slot)
			value, err := vmSuite.session.Execution.StorageAt(ctx, contract, key, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(new(big.Int).SetBytes(value)).To(gomega.Equal(want))
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout), ginkgo.Label(
		"behavior:vm:mined-opcode-path",
	))

	ginkgo.It("records mined terminal opcode status", func(ctx ginkgo.SpecContext) {
		for _, test := range []struct {
			name    string
			runtime []byte
			status  uint64
		}{
			{"STOP", []byte{byte(qrvm.STOP)}, types.ReceiptStatusSuccessful},
			{"RETURN", []byte{byte(qrvm.PUSH0), byte(qrvm.PUSH0), byte(qrvm.RETURN)}, types.ReceiptStatusSuccessful},
			{"REVERT", []byte{byte(qrvm.PUSH0), byte(qrvm.PUSH0), byte(qrvm.REVERT)}, types.ReceiptStatusFailed},
			{"INVALID", []byte{byte(qrvm.INVALID)}, types.ReceiptStatusFailed},
		} {
			ginkgo.By(test.name)
			contract := vmSuite.deployRuntime(ctx, test.runtime)
			receipt := vmSuite.mineCall(ctx, contract)
			gomega.Expect(receipt.Status).To(gomega.Equal(test.status))
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout), ginkgo.Label(
		"behavior:vm:mined-terminal-status",
	))
}
