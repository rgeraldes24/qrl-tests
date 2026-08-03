// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	qrlapi "github.com/cyyber/qrl-tests/endtoend/internal/rpctypes"
	"github.com/theQRL/go-qrl/common"
	qrvm "github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/crypto"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerMemoryStorageSpecs() {
	ginkgo.It("loads and stores full 64-byte memory words", func(ctx ginkgo.SpecContext) {
		value := make([]byte, qrvm.WordBytes)
		for index := range value {
			value[index] = byte(index + 1)
		}
		gomega.Expect(vmSuite.callCode(ctx, memoryCode(value), nil)).To(gomega.Equal(value))
		for _, offset := range []byte{1, qrvm.WordBytes - 1} {
			gomega.Expect(vmSuite.callCode(ctx, memoryCodeAt(value, offset), nil)).To(gomega.Equal(value))
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout))

	ginkgo.It("stores and loads a full-width storage value", func(ctx ginkgo.SpecContext) {
		key := patternedBytes(common.HashLength)
		value := patternedBytes(qrvm.WordBytes)
		gomega.Expect(vmSuite.callCode(ctx, storageRoundTripCode(key, value), nil)).To(
			gomega.Equal(value),
		)
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}

func registerCopyHashSpecs() {
	ginkgo.It("copies calldata across 64-byte word boundaries", func(ctx ginkgo.SpecContext) {
		for _, size := range []int{63, 64, 65} {
			input := patternedBytes(size)
			gomega.Expect(vmSuite.callCodeWithInput(ctx, echoCalldataCode(), input, nil)).To(
				gomega.Equal(input),
			)
			for _, offset := range []int{0, 1} {
				want := make([]byte, qrvm.WordBytes)
				if offset < len(input) {
					copy(want, input[offset:])
				}
				gomega.Expect(vmSuite.callCodeWithInput(
					ctx, calldataLoadCode(byte(offset)), input, nil,
				)).To(gomega.Equal(want))
			}
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout))

	ginkgo.It("copies code and return data across 64-byte word boundaries", func(ctx ginkgo.SpecContext) {
		callee := patternedAddress(0xf2)
		for _, size := range []int{63, 64, 65} {
			input := patternedBytes(size)
			gomega.Expect(vmSuite.callCode(ctx, codeCopyCode(input), nil)).To(gomega.Equal(input))

			overrides := qrlapi.StateOverride{callee: codeOverride(input)}
			gomega.Expect(vmSuite.callCode(ctx, extCodeCopyCode(callee, byte(size)), overrides)).To(
				gomega.Equal(input),
			)

			overrides[callee] = codeOverride(codeCopyCode(input))
			gomega.Expect(vmSuite.callCode(ctx, returnDataCopyCode(callee), overrides)).To(
				gomega.Equal(input),
			)
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout))

	ginkgo.It("hashes memory across 64-byte word boundaries", func(ctx ginkgo.SpecContext) {
		for _, size := range []int{63, 64, 65} {
			input := patternedBytes(size)
			want := common.LeftPadBytes(crypto.Keccak256(input), qrvm.WordBytes)
			gomega.Expect(vmSuite.callCodeWithInput(ctx, keccakCalldataCode(), input, nil)).To(
				gomega.Equal(want),
			)
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}
