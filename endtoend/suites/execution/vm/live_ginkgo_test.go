// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"math/big"
	"time"

	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	qrlapi "github.com/cyyber/qrl-tests/endtoend/internal/rpctypes"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/core/types"
	qrvm "github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/crypto"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const liveSpecTimeout = 5 * time.Minute

type liveSuite struct {
	session *endtoendlive.Session
	target  common.Address
}

var _ = ginkgo.Describe(
	"hand-written QRVM bytecode",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label(
		"e2e", "live", "vm", "mutates-chain", "scenario",
		"scenario:stable:all-opcodes-test", "behavior:vm:vm64-opcodes",
	),
	func() {
		var suite *liveSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			runtime, err := endtoendlive.Load(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			session, err := runtime.Primary(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite = &liveSuite{
				session: session,
				target:  patternedAddress(0xf0),
			}
		})

		ginkgo.It("executes PUSH33 through PUSH64", func(ctx ginkgo.SpecContext) {
			for width := 33; width <= qrvm.WordBytes; width++ {
				code, want := pushCode(width)
				gomega.Expect(suite.callCode(ctx, code, nil)).To(gomega.Equal(want))
			}
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("executes shifted DUP and SWAP ranges", func(ctx ginkgo.SpecContext) {
			for depth := 1; depth <= 16; depth++ {
				gomega.Expect(suite.callCode(ctx, dupCode(depth), nil)).To(
					gomega.Equal(common.LeftPadBytes([]byte{1}, qrvm.WordBytes)),
				)
				gomega.Expect(suite.callCode(ctx, swapCode(depth), nil)).To(
					gomega.Equal(common.LeftPadBytes([]byte{1}, qrvm.WordBytes)),
				)
			}
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("executes 512-bit arithmetic, signed, shift, byte, and sign-extension boundaries", func(ctx ginkgo.SpecContext) {
			for _, test := range vm64OperationCases() {
				ginkgo.By(test.name)
				gomega.Expect(suite.callCode(ctx, operationCode(test.op, test.operands...), nil)).To(
					gomega.Equal(vmWord(test.want)),
				)
			}
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("loads and stores full 64-byte memory words", func(ctx ginkgo.SpecContext) {
			value := make([]byte, qrvm.WordBytes)
			for index := range value {
				value[index] = byte(index + 1)
			}
			gomega.Expect(suite.callCode(ctx, memoryCode(value), nil)).To(gomega.Equal(value))
			for _, offset := range []byte{1, qrvm.WordBytes - 1} {
				gomega.Expect(suite.callCode(ctx, memoryCodeAt(value, offset), nil)).To(gomega.Equal(value))
			}
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("stores and loads a full-width storage value", func(ctx ginkgo.SpecContext) {
			key := patternedBytes(common.HashLength)
			value := patternedBytes(qrvm.WordBytes)
			gomega.Expect(suite.callCode(ctx, storageRoundTripCode(key, value), nil)).To(
				gomega.Equal(value),
			)
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("does not treat JUMPDEST bytes inside PUSH33 through PUSH64 as code", func(ctx ginkgo.SpecContext) {
			for width := 33; width <= qrvm.WordBytes; width++ {
				_, err := suite.callCodeResult(ctx, jumpDestinationCode(width, true), nil, nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(suite.callCode(ctx, jumpDestinationCode(width, false), nil)).To(
					gomega.Equal(vmWord(big.NewInt(1))),
				)
			}
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("mines structural opcode state changes", func(ctx ginkgo.SpecContext) {
			runtime, expectedPC := minedStructuralCode()
			contract := suite.deployRuntime(ctx, runtime)
			receipt := suite.mineCall(ctx, contract)
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
				value, err := suite.session.Execution.StorageAt(ctx, contract, key, nil)
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
				contract := suite.deployRuntime(ctx, test.runtime)
				receipt := suite.mineCall(ctx, contract)
				gomega.Expect(receipt.Status).To(gomega.Equal(test.status))
			}
		}, ginkgo.SpecTimeout(liveSpecTimeout), ginkgo.Label(
			"behavior:vm:mined-terminal-status",
		))

		ginkgo.It("copies calldata across 64-byte word boundaries", func(ctx ginkgo.SpecContext) {
			for _, size := range []int{63, 64, 65} {
				input := patternedBytes(size)
				gomega.Expect(suite.callCodeWithInput(ctx, echoCalldataCode(), input, nil)).To(
					gomega.Equal(input),
				)
				for _, offset := range []int{0, 1} {
					want := make([]byte, qrvm.WordBytes)
					if offset < len(input) {
						copy(want, input[offset:])
					}
					gomega.Expect(suite.callCodeWithInput(
						ctx,
						calldataLoadCode(byte(offset)),
						input,
						nil,
					)).To(gomega.Equal(want))
				}
			}
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("copies code and return data across 64-byte word boundaries", func(ctx ginkgo.SpecContext) {
			callee := patternedAddress(0xf2)
			for _, size := range []int{63, 64, 65} {
				input := patternedBytes(size)
				gomega.Expect(suite.callCode(ctx, codeCopyCode(input), nil)).To(gomega.Equal(input))

				overrides := qrlapi.StateOverride{callee: codeOverride(input)}
				gomega.Expect(suite.callCode(
					ctx,
					extCodeCopyCode(callee, byte(size)),
					overrides,
				)).To(gomega.Equal(input))

				overrides[callee] = codeOverride(codeCopyCode(input))
				gomega.Expect(suite.callCode(ctx, returnDataCopyCode(callee), overrides)).To(
					gomega.Equal(input),
				)
			}
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("hashes memory across 64-byte word boundaries", func(ctx ginkgo.SpecContext) {
			for _, size := range []int{63, 64, 65} {
				input := patternedBytes(size)
				want := common.LeftPadBytes(crypto.Keccak256(input), qrvm.WordBytes)
				gomega.Expect(suite.callCodeWithInput(ctx, keccakCalldataCode(), input, nil)).To(
					gomega.Equal(want),
				)
			}
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("executes CALL, STATICCALL, and DELEGATECALL", func(ctx ginkgo.SpecContext) {
			callee := patternedAddress(0xf1)
			overrides := qrlapi.StateOverride{callee: codeOverride(callContextCode())}
			for _, op := range []qrvm.OpCode{qrvm.CALL, qrvm.STATICCALL, qrvm.DELEGATECALL} {
				output := suite.callCode(ctx, callCode(op, callee), overrides)
				gomega.Expect(output).To(gomega.HaveLen(4 * qrvm.WordBytes))

				wantAddress, wantCaller := callee, suite.target
				if op == qrvm.DELEGATECALL {
					wantAddress = suite.target
					wantCaller = suite.session.Address
				}
				gomega.Expect(common.BytesToAddress(output[:qrvm.WordBytes])).To(gomega.Equal(wantAddress))
				gomega.Expect(common.BytesToAddress(output[qrvm.WordBytes : 2*qrvm.WordBytes])).To(gomega.Equal(wantCaller))
				gomega.Expect(new(big.Int).SetBytes(output[2*qrvm.WordBytes : 3*qrvm.WordBytes]).Sign()).To(gomega.BeZero())
				gomega.Expect(new(big.Int).SetBytes(output[3*qrvm.WordBytes:]).Uint64()).To(gomega.Equal(uint64(1)))
			}

			valueCode := callCodeWithValue(qrvm.CALL, callee, 7)
			valueOutput := suite.callCode(ctx, valueCode, qrlapi.StateOverride{
				suite.target: codeAndBalanceOverride(valueCode, big.NewInt(10)),
				callee:       codeAndBalanceOverride(callValueContextCode(), new(big.Int)),
			})
			gomega.Expect(valueOutput).To(gomega.HaveLen(4 * qrvm.WordBytes))
			gomega.Expect(new(big.Int).SetBytes(valueOutput[:qrvm.WordBytes])).To(
				gomega.Equal(big.NewInt(7)),
			)
			gomega.Expect(new(big.Int).SetBytes(valueOutput[qrvm.WordBytes : 2*qrvm.WordBytes])).To(
				gomega.Equal(big.NewInt(7)),
			)
			gomega.Expect(new(big.Int).SetBytes(valueOutput[3*qrvm.WordBytes:])).To(
				gomega.Equal(big.NewInt(1)),
			)
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("reverts failed CALL, STATICCALL, and DELEGATECALL effects", func(ctx ginkgo.SpecContext) {
			callee := patternedAddress(0xf4)
			callerBalance := big.NewInt(100)
			calleeBalance := big.NewInt(5)
			for _, op := range []qrvm.OpCode{qrvm.CALL, qrvm.STATICCALL, qrvm.DELEGATECALL} {
				code := failingCallCode(op, callee)
				overrides := qrlapi.StateOverride{
					suite.target: codeAndBalanceOverride(code, callerBalance),
					callee:       codeAndBalanceOverride(revertingStorageCode(), calleeBalance),
				}
				output := suite.callCode(ctx, code, overrides)
				gomega.Expect(output).To(gomega.HaveLen(3 * qrvm.WordBytes))
				gomega.Expect(new(big.Int).SetBytes(output[:qrvm.WordBytes]).Sign()).To(gomega.BeZero())
				if op == qrvm.CALL {
					gomega.Expect(new(big.Int).SetBytes(output[qrvm.WordBytes : 2*qrvm.WordBytes])).To(
						gomega.Equal(calleeBalance),
					)
				} else {
					gomega.Expect(new(big.Int).SetBytes(output[qrvm.WordBytes : 2*qrvm.WordBytes]).Sign()).To(gomega.BeZero())
				}
				gomega.Expect(new(big.Int).SetBytes(output[2*qrvm.WordBytes:])).To(
					gomega.Equal(callerBalance),
				)
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
							suite.target: codeAndBalanceOverride(code, big.NewInt(100)),
						}
					}
					output := suite.callCode(ctx, code, overrides)
					gomega.Expect(output).To(gomega.HaveLen(5 * qrvm.WordBytes))
					gomega.Expect(new(big.Int).SetBytes(output[:qrvm.WordBytes]).Uint64()).To(gomega.Equal(uint64(len(returnWordCode([]byte{0x2a})))))

					var wantAddress common.Address
					if op == qrvm.CREATE {
						wantAddress = crypto.CreateAddress(suite.target, 0)
					} else {
						initHash := crypto.Keccak256Hash(childInit)
						wantAddress = crypto.CreateAddress2(suite.target, salt, initHash[:])
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
					child = crypto.CreateAddress(suite.target, 0)
				} else {
					hash := crypto.Keccak256Hash(initCode)
					child = crypto.CreateAddress2(suite.target, salt, hash[:])
				}
				code, encodedInit := failingCreateCode(op, child, salt)
				gomega.Expect(encodedInit).To(gomega.Equal(initCode))
				output := suite.callCode(ctx, code, qrlapi.StateOverride{
					suite.target: codeAndBalanceOverride(code, callerBalance),
				})
				gomega.Expect(output).To(gomega.HaveLen(4 * qrvm.WordBytes))
				for index := 0; index < 3; index++ {
					gomega.Expect(new(big.Int).SetBytes(output[index*qrvm.WordBytes : (index+1)*qrvm.WordBytes]).Sign()).To(gomega.BeZero())
				}
				gomega.Expect(new(big.Int).SetBytes(output[3*qrvm.WordBytes:])).To(
					gomega.Equal(callerBalance),
				)
			}
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("returns full-width address and block context values", func(ctx ginkgo.SpecContext) {
			header, err := suite.session.Execution.HeaderByNumber(ctx, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(header.Number.Sign()).To(gomega.BeNumerically(">", 0))
			block := hexutil.EncodeBig(header.Number)
			balance, err := suite.session.Execution.BalanceAt(ctx, suite.session.Address, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(suite.callCodeAt(ctx, opcodeCode(qrvm.ORIGIN), nil, block, nil)).To(
				gomega.Equal(suite.session.Address.Bytes()),
			)
			gomega.Expect(suite.callCodeAt(ctx, opcodeCode(qrvm.COINBASE), nil, block, nil)).To(
				gomega.Equal(header.Coinbase.Bytes()),
			)
			gomega.Expect(suite.callCodeAt(ctx, addressOpcodeCode(qrvm.BALANCE, suite.session.Address), nil, block, nil)).To(
				gomega.Equal(vmWord(balance)),
			)

			callee := patternedAddress(0xf5)
			code := returnWordCode([]byte{0x2a})
			gomega.Expect(suite.callCodeAt(ctx, addressOpcodeCode(qrvm.EXTCODEHASH, callee), qrlapi.StateOverride{
				callee: codeOverride(code),
			}, block, nil)).To(gomega.Equal(vmWord(new(big.Int).SetBytes(crypto.Keccak256(code)))))

			for _, test := range []struct {
				name string
				op   qrvm.OpCode
				want *big.Int
			}{
				{"NUMBER", qrvm.NUMBER, header.Number},
				{"TIMESTAMP", qrvm.TIMESTAMP, new(big.Int).SetUint64(header.Time)},
				{"GASLIMIT", qrvm.GASLIMIT, new(big.Int).SetUint64(header.GasLimit)},
				{"CHAINID", qrvm.CHAINID, suite.session.ChainID},
				{"BASEFEE", qrvm.BASEFEE, header.BaseFee},
				{"RANDOM", qrvm.RANDOM, new(big.Int).SetBytes(header.Random.Bytes())},
			} {
				ginkgo.By(test.name)
				gomega.Expect(suite.callCodeAt(ctx, opcodeCode(test.op), nil, block, nil)).To(
					gomega.Equal(vmWord(test.want)),
				)
			}

			parentNumber := new(big.Int).Sub(header.Number, big.NewInt(1))
			parent, err := suite.session.Execution.HeaderByNumber(ctx, parentNumber)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(suite.callCodeAt(ctx, operationCode(
				qrvm.BLOCKHASH,
				parentNumber.Bytes(),
			), nil, block, nil)).To(
				gomega.Equal(vmWord(new(big.Int).SetBytes(parent.Hash().Bytes()))),
			)

			selfBalance := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(9))
			selfBalanceCode := opcodeCode(qrvm.SELFBALANCE)
			gomega.Expect(suite.callCodeAt(ctx, selfBalanceCode, qrlapi.StateOverride{
				suite.target: codeAndBalanceOverride(selfBalanceCode, selfBalance),
			}, block, nil)).To(gomega.Equal(vmWord(selfBalance)))

			tip := big.NewInt(7)
			feeCap := new(big.Int).Add(header.BaseFee, big.NewInt(100))
			gomega.Expect(suite.callCodeAt(ctx, opcodeCode(qrvm.GASPRICE), nil, block, map[string]any{
				"maxFeePerGas":         (*hexutil.Big)(feeCap),
				"maxPriorityFeePerGas": (*hexutil.Big)(tip),
			})).To(gomega.Equal(vmWord(new(big.Int).Add(header.BaseFee, tip))))
		}, ginkgo.SpecTimeout(liveSpecTimeout))

		ginkgo.It("mines LOG0 through LOG4 with full-width values", func(ctx ginkgo.SpecContext) {
			data := make([]byte, qrvm.WordBytes)
			for index := range data {
				data[index] = byte(index + 1)
			}

			for count := 0; count <= 4; count++ {
				topics := make([]common.LogTopic, count)
				for topicIndex := range topics {
					for byteIndex := range topics[topicIndex] {
						topics[topicIndex][byteIndex] = byte((topicIndex+1)*17 + byteIndex)
					}
				}
				receipt := suite.mineLog(ctx, data, topics)
				gomega.Expect(receipt.Logs).To(gomega.HaveLen(1))
				gomega.Expect(receipt.Logs[0].Topics).To(gomega.Equal(topics))
				gomega.Expect(receipt.Logs[0].Data).To(gomega.Equal(data))
			}
		}, ginkgo.SpecTimeout(liveSpecTimeout))
	},
)
