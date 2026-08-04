//go:build e2e

package protocol_test

import (
	"math/big"
	"math/bits"
	"strconv"

	"github.com/cyyber/qrl-tests/endtoend/internal/behavior"
	consensus "github.com/cyyber/qrl-tests/endtoend/internal/consensus/client"
	consensusverify "github.com/cyyber/qrl-tests/endtoend/internal/consensus/verify"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerProtocolSignatureChecks(suite *protocolSuite) {
	ginkgo.It("verifies execution-data votes, fee recipients, sync participation, and every carried signature", func(ctx ginkgo.SpecContext) {
		start, end := suite.previousEpoch(ctx)
		verifier, err := consensusverify.New(ctx, suite.beacons[0])
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		votingEpochs, err := suite.beacons[0].SpecUint(ctx, "EPOCHS_PER_EXECUTION_VOTING_PERIOD")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		votingPeriod := votingEpochs * suite.slotsPerEpoch
		votes := make(map[uint64]consensus.ExecutionDataVote)
		verified := consensusverify.SignatureSummary{}
		produced := 0
		for slot := start; slot < end; slot++ {
			blockID := strconv.FormatUint(slot, 10)
			block, err := suite.beacons[0].Block(ctx, blockID)
			if consensus.IsNotFound(err) {
				continue
			}
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			body := block.Message.Body
			data := body.ExecutionData
			produced++
			period := slot / votingPeriod
			if vote, found := votes[period]; found {
				gomega.Expect(data).To(gomega.Equal(vote))
			} else {
				gomega.Expect(data.DepositRoot).To(gomega.MatchRegexp(`^0x[0-9a-fA-F]{64}$`))
				gomega.Expect(data.BlockHash).To(gomega.MatchRegexp(`^0x[0-9a-fA-F]{64}$`))
				votes[period] = data
			}

			bitsBytes, err := hexutil.Decode(body.SyncAggregate.Bits)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			setBits := 0
			for _, value := range bitsBytes {
				setBits += bits.OnesCount8(value)
			}
			gomega.Expect(body.SyncAggregate.Signatures).To(gomega.HaveLen(setBits))

			payload := body.ExecutionPayload
			feeRecipient, err := hexutil.Decode(payload.FeeRecipient)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(feeRecipient).To(gomega.HaveLen(common.AddressLength))
			recipient := common.BytesToAddress(feeRecipient)
			gomega.Expect(recipient).To(gomega.Equal(common.MustParseAddress(expectedFeeRecipient)))
			if payload.GasUsed > 0 {
				parent, err := suite.sessions[0].Execution.BlockByHash(ctx, common.HexToHash(payload.ParentHash))
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				balanceBefore, err := suite.sessions[0].Execution.BalanceAt(ctx, recipient, parent.Number())
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				balanceAfter, err := suite.sessions[0].Execution.BalanceAt(ctx, recipient, new(big.Int).SetUint64(payload.BlockNumber))
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(balanceAfter).To(gomega.BeNumerically(">", balanceBefore))
			}

			header, err := suite.beacons[0].BlockHeader(ctx, blockID)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			summary, err := verifier.Verify(ctx, header, block)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			verified.Block += summary.Block
			verified.Randao += summary.Randao
			verified.Attestations += summary.Attestations
			verified.SyncCommittee += summary.SyncCommittee
			verified.Deposits += summary.Deposits
			verified.VoluntaryExits += summary.VoluntaryExits
			verified.ProposerSlashings += summary.ProposerSlashings
			verified.AttesterSlashings += summary.AttesterSlashings
		}
		gomega.Expect(produced).To(gomega.BeNumerically(">", 0))
		gomega.Expect(verified.Block).To(gomega.Equal(produced))
		gomega.Expect(verified.Randao).To(gomega.Equal(produced))
		gomega.Expect(verified.Attestations).To(gomega.BeNumerically(">", 0))
		gomega.Expect(verified.SyncCommittee).To(gomega.BeNumerically(">", 0))
	}, ginkgo.SpecTimeout(protocolTimeout), ginkgo.Label(
		behavior.Name("consensus:execution-data-votes"),
		behavior.Name("consensus:fee-recipients"),
		behavior.Name("consensus:sync-committee-participation"),
		behavior.Name("consensus:signature-verification"),
	))
}
