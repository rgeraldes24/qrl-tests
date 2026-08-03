//go:build e2e

package transactions

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/cyyber/qrl-tests/endtoend/internal/execfixture"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/cyyber/qrl-tests/endtoend/internal/stability"
	"github.com/theQRL/go-qrl/core/types"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerTransactionWorkloads(sessions *[]*endtoendlive.Session) {
	ginkgo.It("runs the complete 1000-transaction calldata workload", func(ctx ginkgo.SpecContext) {
		nodes := *sessions
		session := nodes[0]
		beacon := session.Consensus
		startFinalized, err := beacon.FinalizedEpoch(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		data := make([]byte, fullCalldataSize)
		recipient := execfixture.PatternedAddress(0xa1)
		parameters := loadTransactionParameters(ctx, session, recipient, new(big.Int), data)
		nonce, err := session.Execution.PendingNonceAt(ctx, session.Address)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		includedByBlock := make(map[uint64]int)

		for batch := 0; batch < fullCalldataTransactionCount/fullTransactionsPerBlock; batch++ {
			transactions := make([]*types.Transaction, fullTransactionsPerBlock)
			for index := range transactions {
				sequence := batch*fullTransactionsPerBlock + index
				to := execfixture.PatternedAddress(byte(0xa1 + sequence))
				transactions[index] = signTransactionAt(
					session,
					nonce+uint64(sequence),
					to,
					new(big.Int),
					data,
					parameters,
				)
				gomega.Expect(session.Execution.SendTransaction(ctx, transactions[index])).To(gomega.Succeed())
			}
			for _, transaction := range transactions {
				receipt := waitForReceipt(ctx, session, transaction)
				gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
				gomega.Expect(receipt.BlockNumber).NotTo(gomega.BeNil())
				includedByBlock[receipt.BlockNumber.Uint64()]++
			}
		}

		total := 0
		for block, count := range includedByBlock {
			gomega.Expect(count).To(
				gomega.BeNumerically("<=", fullTransactionsPerBlock),
				fmt.Sprintf("execution block %d exceeded the workload limit", block),
			)
			total += count
		}
		gomega.Expect(total).To(gomega.Equal(fullCalldataTransactionCount))
		gomega.Expect(stability.Await(ctx, nodes, startFinalized, 2)).To(gomega.Succeed())
	}, ginkgo.SpecTimeout(fullWorkloadTimeout), ginkgo.Label(
		"scenario-full",
		"scenario:stable:big-calldata-tx-test",
		"behavior:transactions:big-calldata-1000",
		"behavior:network:post-workload-stability",
	))

	ginkgo.It("sustains ten transactions per block through every client and proposer", func(ctx ginkgo.SpecContext) {
		nodes := *sessions
		beacon := nodes[0].Consensus
		startFinalized, err := beacon.FinalizedEpoch(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		expectedProposers := make(map[string]struct{}, len(nodes))
		for _, session := range nodes {
			name := strings.TrimPrefix(session.Participant.Validator.Name, "vc-")
			gomega.Expect(name).NotTo(gomega.BeEmpty())
			expectedProposers[name] = struct{}{}
		}
		observedProposers := make(map[string]struct{}, len(expectedProposers))
		usedClients := make(map[int]struct{}, len(nodes))
		lastSlot, err := beacon.HeadSlot(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		for batch := 0; batch < 64 && len(observedProposers) < len(expectedProposers); batch++ {
			session := nodes[batch%len(nodes)]
			usedClients[session.Participant.Index] = struct{}{}
			nonce, err := session.Execution.PendingNonceAt(ctx, session.Address)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			to := execfixture.PatternedAddress(byte(0xc0 + batch))
			parameters := loadTransactionParameters(ctx, session, to, big.NewInt(1), nil)
			transactions := make([]*types.Transaction, fullTransactionsPerBlock)
			blocks := make(map[uint64]int)

			for index := range transactions {
				transactions[index] = signTransactionAt(
					session,
					nonce+uint64(index),
					execfixture.PatternedAddress(byte(0xd0+batch+index)),
					big.NewInt(int64(index+1)),
					nil,
					parameters,
				)
				gomega.Expect(session.Execution.SendTransaction(ctx, transactions[index])).To(gomega.Succeed())
			}
			for _, transaction := range transactions {
				receipt := waitForReceipt(ctx, session, transaction)
				gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
				blocks[receipt.BlockNumber.Uint64()]++
			}

			currentSlot, err := beacon.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			for slot := lastSlot + 1; slot <= currentSlot; slot++ {
				payload, err := beacon.BlockExecutionPayload(ctx, strconv.FormatUint(slot, 10))
				if consensus.IsNotFound(err) {
					continue
				}
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				if blocks[payload.BlockNumber] != fullTransactionsPerBlock {
					continue
				}
				graffiti, err := beacon.BlockGraffitiText(ctx, strconv.FormatUint(slot, 10))
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				observedProposers[graffiti] = struct{}{}
			}
			lastSlot = currentSlot
		}

		gomega.Expect(usedClients).To(gomega.HaveLen(len(nodes)))
		for proposer := range expectedProposers {
			gomega.Expect(observedProposers).To(gomega.HaveKey(proposer))
		}
		gomega.Expect(stability.Await(ctx, nodes, startFinalized, 2)).To(gomega.Succeed())
	}, ginkgo.SpecTimeout(fullWorkloadTimeout), ginkgo.Label(
		"scenario-full",
		"scenario:stable:eoa-transactions-test",
		"behavior:transactions:sustained-10-per-block",
		"behavior:transactions:proposer-inclusion-matrix",
		"behavior:network:post-workload-stability",
	))
}
