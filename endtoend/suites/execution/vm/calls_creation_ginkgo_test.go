// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"math/big"

	qrlapi "github.com/cyyber/qrl-tests/endtoend/internal/rpctypes"
	"github.com/theQRL/go-qrl/common"
	qrvm "github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/crypto"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerCallCreationSpecs() {
	ginkgo.It("executes CALL, STATICCALL, and DELEGATECALL", func(ctx ginkgo.SpecContext) {
		callee := patternedAddress(0xf1)
		overrides := qrlapi.StateOverride{callee: codeOverride(callContextCode())}
		for _, op := range []qrvm.OpCode{qrvm.CALL, qrvm.STATICCALL, qrvm.DELEGATECALL} {
			output := vmSuite.callCode(ctx, callCode(op, callee), overrides)
			gomega.Expect(output).To(gomega.HaveLen(4 * qrvm.WordBytes))

			wantAddress, wantCaller := callee, vmSuite.target
			if op == qrvm.DELEGATECALL {
				wantAddress = vmSuite.target
				wantCaller = vmSuite.session.Address
			}
			gomega.Expect(common.BytesToAddress(output[:qrvm.WordBytes])).To(gomega.Equal(wantAddress))
			gomega.Expect(common.BytesToAddress(output[qrvm.WordBytes : 2*qrvm.WordBytes])).To(gomega.Equal(wantCaller))
			gomega.Expect(new(big.Int).SetBytes(output[2*qrvm.WordBytes : 3*qrvm.WordBytes]).Sign()).To(gomega.BeZero())
			gomega.Expect(new(big.Int).SetBytes(output[3*qrvm.WordBytes:]).Uint64()).To(gomega.Equal(uint64(1)))
		}

		valueCode := callCodeWithValue(qrvm.CALL, callee, 7)
		valueOutput := vmSuite.callCode(ctx, valueCode, qrlapi.StateOverride{
			vmSuite.target: codeAndBalanceOverride(valueCode, big.NewInt(10)),
			callee:         codeAndBalanceOverride(callValueContextCode(), new(big.Int)),
		})
		gomega.Expect(valueOutput).To(gomega.HaveLen(4 * qrvm.WordBytes))
		gomega.Expect(new(big.Int).SetBytes(valueOutput[:qrvm.WordBytes])).To(gomega.Equal(big.NewInt(7)))
		gomega.Expect(new(big.Int).SetBytes(valueOutput[qrvm.WordBytes : 2*qrvm.WordBytes])).To(gomega.Equal(big.NewInt(7)))
		gomega.Expect(new(big.Int).SetBytes(valueOutput[3*qrvm.WordBytes:])).To(gomega.Equal(big.NewInt(1)))
	}, ginkgo.SpecTimeout(liveSpecTimeout))

	ginkgo.It("reverts failed CALL, STATICCALL, and DELEGATECALL effects", func(ctx ginkgo.SpecContext) {
		callee := patternedAddress(0xf4)
		callerBalance := big.NewInt(100)
		calleeBalance := big.NewInt(5)
		for _, op := range []qrvm.OpCode{qrvm.CALL, qrvm.STATICCALL, qrvm.DELEGATECALL} {
			code := failingCallCode(op, callee)
			overrides := qrlapi.StateOverride{
				vmSuite.target: codeAndBalanceOverride(code, callerBalance),
				callee:         codeAndBalanceOverride(revertingStorageCode(), calleeBalance),
			}
			output := vmSuite.callCode(ctx, code, overrides)
			gomega.Expect(output).To(gomega.HaveLen(3 * qrvm.WordBytes))
			gomega.Expect(new(big.Int).SetBytes(output[:qrvm.WordBytes]).Sign()).To(gomega.BeZero())
			if op == qrvm.CALL {
				gomega.Expect(new(big.Int).SetBytes(output[qrvm.WordBytes : 2*qrvm.WordBytes])).To(
					gomega.Equal(calleeBalance),
				)
			} else {
				gomega.Expect(new(big.Int).SetBytes(output[qrvm.WordBytes : 2*qrvm.WordBytes]).Sign()).To(gomega.BeZero())
			}
			gomega.Expect(new(big.Int).SetBytes(output[2*qrvm.WordBytes:])).To(gomega.Equal(callerBalance))
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout))

	ginkgo.It("executes CREATE and CREATE2", func(ctx ginkgo.SpecContext) {
		salt := patternedCreate2Salt()
		for _, op := range []qrvm.OpCode{qrvm.CREATE, qrvm.CREATE2} {
			for _, value := range []byte{0, 7} {
				code, childInit := createCode(op, value, salt)
				var overrides qrlapi.StateOverride
				if value > 0 {
					overrides = qrlapi.StateOverride{
						vmSuite.target: codeAndBalanceOverride(code, big.NewInt(100)),
					}
				}
				output := vmSuite.callCode(ctx, code, overrides)
				gomega.Expect(output).To(gomega.HaveLen(5 * qrvm.WordBytes))
				gomega.Expect(new(big.Int).SetBytes(output[:qrvm.WordBytes]).Uint64()).To(
					gomega.Equal(uint64(len(returnWordCode([]byte{0x2a})))),
				)

				var wantAddress common.Address
				if op == qrvm.CREATE {
					wantAddress = crypto.CreateAddress(vmSuite.target, 0)
				} else {
					initHash := crypto.Keccak256Hash(childInit)
					wantAddress = crypto.CreateAddress2(vmSuite.target, salt, initHash[:])
				}
				gomega.Expect(common.BytesToAddress(output[qrvm.WordBytes : 2*qrvm.WordBytes])).To(gomega.Equal(wantAddress))
				gomega.Expect(new(big.Int).SetBytes(output[2*qrvm.WordBytes : 3*qrvm.WordBytes]).Uint64()).To(gomega.Equal(uint64(0x2a)))
				gomega.Expect(new(big.Int).SetBytes(output[3*qrvm.WordBytes : 4*qrvm.WordBytes]).Uint64()).To(gomega.Equal(uint64(1)))
				gomega.Expect(new(big.Int).SetBytes(output[4*qrvm.WordBytes:]).Uint64()).To(gomega.Equal(uint64(value)))
			}
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout))

	ginkgo.It("reverts failed CREATE and CREATE2 state and value", func(ctx ginkgo.SpecContext) {
		callerBalance := big.NewInt(100)
		initCode := revertingStorageCode()
		salt := patternedCreate2Salt()
		for _, op := range []qrvm.OpCode{qrvm.CREATE, qrvm.CREATE2} {
			var child common.Address
			if op == qrvm.CREATE {
				child = crypto.CreateAddress(vmSuite.target, 0)
			} else {
				hash := crypto.Keccak256Hash(initCode)
				child = crypto.CreateAddress2(vmSuite.target, salt, hash[:])
			}
			code, encodedInit := failingCreateCode(op, child, salt)
			gomega.Expect(encodedInit).To(gomega.Equal(initCode))
			output := vmSuite.callCode(ctx, code, qrlapi.StateOverride{
				vmSuite.target: codeAndBalanceOverride(code, callerBalance),
			})
			gomega.Expect(output).To(gomega.HaveLen(4 * qrvm.WordBytes))
			for index := 0; index < 3; index++ {
				gomega.Expect(new(big.Int).SetBytes(output[index*qrvm.WordBytes : (index+1)*qrvm.WordBytes]).Sign()).To(gomega.BeZero())
			}
			gomega.Expect(new(big.Int).SetBytes(output[3*qrvm.WordBytes:])).To(gomega.Equal(callerBalance))
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}
