// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"math/big"

	qrlapi "github.com/cyyber/qrl-tests/endtoend/internal/rpctypes"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	qrvm "github.com/theQRL/go-qrl/core/vm"
	"github.com/theQRL/go-qrl/crypto"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerContextLogSpecs() {
	ginkgo.It("returns full-width address and block context values", func(ctx ginkgo.SpecContext) {
		header, err := vmSuite.session.Execution.HeaderByNumber(ctx, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(header.Number.Sign()).To(gomega.BeNumerically(">", 0))
		block := hexutil.EncodeBig(header.Number)
		balance, err := vmSuite.session.Execution.BalanceAt(ctx, vmSuite.session.Address, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		gomega.Expect(vmSuite.callCodeAt(ctx, opcodeCode(qrvm.ORIGIN), nil, block, nil)).To(
			gomega.Equal(vmSuite.session.Address.Bytes()),
		)
		gomega.Expect(vmSuite.callCodeAt(ctx, opcodeCode(qrvm.COINBASE), nil, block, nil)).To(
			gomega.Equal(header.Coinbase.Bytes()),
		)
		gomega.Expect(vmSuite.callCodeAt(ctx, addressOpcodeCode(qrvm.BALANCE, vmSuite.session.Address), nil, block, nil)).To(
			gomega.Equal(vmWord(balance)),
		)

		callee := patternedAddress(0xf5)
		code := returnWordCode([]byte{0x2a})
		gomega.Expect(vmSuite.callCodeAt(ctx, addressOpcodeCode(qrvm.EXTCODEHASH, callee), qrlapi.StateOverride{
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
			{"CHAINID", qrvm.CHAINID, vmSuite.session.ChainID},
			{"BASEFEE", qrvm.BASEFEE, header.BaseFee},
			{"RANDOM", qrvm.RANDOM, new(big.Int).SetBytes(header.Random.Bytes())},
		} {
			ginkgo.By(test.name)
			gomega.Expect(vmSuite.callCodeAt(ctx, opcodeCode(test.op), nil, block, nil)).To(gomega.Equal(vmWord(test.want)))
		}

		parentNumber := new(big.Int).Sub(header.Number, big.NewInt(1))
		parent, err := vmSuite.session.Execution.HeaderByNumber(ctx, parentNumber)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(vmSuite.callCodeAt(ctx, operationCode(qrvm.BLOCKHASH, parentNumber.Bytes()), nil, block, nil)).To(
			gomega.Equal(vmWord(new(big.Int).SetBytes(parent.Hash().Bytes()))),
		)

		selfBalance := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(9))
		selfBalanceCode := opcodeCode(qrvm.SELFBALANCE)
		gomega.Expect(vmSuite.callCodeAt(ctx, selfBalanceCode, qrlapi.StateOverride{
			vmSuite.target: codeAndBalanceOverride(selfBalanceCode, selfBalance),
		}, block, nil)).To(gomega.Equal(vmWord(selfBalance)))

		tip := big.NewInt(7)
		feeCap := new(big.Int).Add(header.BaseFee, big.NewInt(100))
		gomega.Expect(vmSuite.callCodeAt(ctx, opcodeCode(qrvm.GASPRICE), nil, block, map[string]any{
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
			receipt := vmSuite.mineLog(ctx, data, topics)
			gomega.Expect(receipt.Logs).To(gomega.HaveLen(1))
			gomega.Expect(receipt.Logs[0].Topics).To(gomega.Equal(topics))
			gomega.Expect(receipt.Logs[0].Data).To(gomega.Equal(data))
		}
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}
