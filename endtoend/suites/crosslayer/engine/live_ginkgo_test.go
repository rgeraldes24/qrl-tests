//go:build e2e

package engine_test

import (
	"math/big"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	engineapi "github.com/cyyber/qrl-tests/endtoend/internal/clients/engine"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	protocolengine "github.com/theQRL/go-qrl/beacon/engine"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	engineJWTSecret = "0xdc49981516e8e72b401a63e6405495a32dafc3939b5d6d83cc319ac0388bca1b"
	engineTimeout   = 5 * time.Minute
)

var _ = ginkgo.Describe(
	"Engine and cross-layer consistency",
	ginkgo.Ordered,
	ginkgo.Label("e2e", "live", "engine", "cross-layer", "scenario"),
	func() {
		var session *endtoendlive.Session
		var beacon *consensus.Client
		var engine *engineapi.Client

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			session, err = endtoendlive.Open(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(session.Close)
			beacon, err = consensus.New(session.Participant.ConsensusURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			engine, err = engineapi.New(session.Participant.EngineURL, engineJWTSecret)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("authenticates and advertises the supported Engine methods", func(ctx ginkgo.SpecContext) {
			capabilities, err := engine.ExchangeCapabilities(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(capabilities).To(gomega.ConsistOf(
				"engine_forkchoiceUpdatedV2",
				"engine_getPayloadV2",
				"engine_newPayloadV2",
				"engine_getPayloadBodiesByHashV1",
				"engine_getPayloadBodiesByRangeV1",
			))

			unauthenticated, err := engineapi.New(session.Participant.EngineURL, strings.Repeat("00", 32))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			_, err = unauthenticated.ExchangeCapabilities(ctx)
			gomega.Expect(err).To(gomega.HaveOccurred())
		}, ginkgo.SpecTimeout(engineTimeout))

		ginkgo.It("returns the same payload through consensus, execution, and Engine APIs", func(ctx ginkgo.SpecContext) {
			payload, err := beacon.BlockExecutionPayload(ctx, "head")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(payload.BlockHash).NotTo(gomega.BeEmpty())

			block, err := session.Execution.BlockByHash(ctx, common.HexToHash(payload.BlockHash))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(block).NotTo(gomega.BeNil())
			gomega.Expect(block.NumberU64()).To(gomega.Equal(payload.BlockNumber))
			gomega.Expect(block.GasLimit()).To(gomega.Equal(payload.GasLimit))
			gomega.Expect(block.GasUsed()).To(gomega.Equal(payload.GasUsed))
			gomega.Expect(block.Transactions()).To(gomega.HaveLen(len(payload.Transactions)))

			bodies, err := engine.PayloadBodiesByHash(ctx, []string{payload.BlockHash})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(bodies).To(gomega.HaveLen(1))
			gomega.Expect(bodies[0]).NotTo(gomega.BeNil())
			gomega.Expect(bodies[0].Transactions).To(gomega.Equal(payload.Transactions))
			gomega.Expect(bodies[0].Withdrawals).To(gomega.HaveLen(len(payload.Withdrawals)))
		}, ginkgo.SpecTimeout(engineTimeout))

		ginkgo.It("validates canonical forkchoice and payload data", func(ctx ginkgo.SpecContext) {
			block, err := session.Execution.BlockByNumber(ctx, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			state := protocolengine.ForkchoiceStateV1{
				HeadBlockHash: block.Hash(), SafeBlockHash: block.Hash(), FinalizedBlockHash: block.Hash(),
			}
			forkchoice, err := engine.ForkchoiceUpdatedV2(ctx, state, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(forkchoice.PayloadStatus.Status).To(gomega.Equal(protocolengine.VALID))

			payload := protocolengine.BlockToExecutableData(block, new(big.Int)).ExecutionPayload
			status, err := engine.NewPayloadV2(ctx, *payload)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(status.Status).To(gomega.Equal(protocolengine.VALID))
			gomega.Expect(status.LatestValidHash).NotTo(gomega.BeNil())
			gomega.Expect(*status.LatestValidHash).To(gomega.Equal(block.Hash()))
			invalid := *payload
			invalid.ExtraData = append(append([]byte(nil), payload.ExtraData...), 0x01)
			status, err = engine.NewPayloadV2(ctx, invalid)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(status.Status).To(gomega.Equal(protocolengine.INVALID))

			unknown := common.HexToHash("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
			state.HeadBlockHash = unknown
			forkchoice, err = engine.ForkchoiceUpdatedV2(ctx, state, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(forkchoice.PayloadStatus.Status).To(gomega.Equal(protocolengine.SYNCING))
		}, ginkgo.SpecTimeout(engineTimeout), ginkgo.Label("behavior:engine:forkchoice-payload-validation"))

		ginkgo.It("builds and returns a payload through Engine V2", func(ctx ginkgo.SpecContext) {
			block, err := session.Execution.BlockByNumber(ctx, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			state := protocolengine.ForkchoiceStateV1{
				HeadBlockHash: block.Hash(), SafeBlockHash: block.Hash(), FinalizedBlockHash: block.Hash(),
			}
			attributes := &protocolengine.PayloadAttributes{
				Timestamp: block.Time() + 1, Random: block.Random(), SuggestedFeeRecipient: block.Coinbase(),
				Withdrawals: []*types.Withdrawal{},
			}
			response, err := engine.ForkchoiceUpdatedV2(ctx, state, attributes)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(response.PayloadStatus.Status).To(gomega.Equal(protocolengine.VALID))
			gomega.Expect(response.PayloadID).NotTo(gomega.BeNil())

			var payload *protocolengine.ExecutionPayloadEnvelope
			gomega.Eventually(func() error {
				payload, err = engine.GetPayloadV2(ctx, *response.PayloadID)
				return err
			}).WithContext(ctx).WithTimeout(engineTimeout).WithPolling(250 * time.Millisecond).Should(gomega.Succeed())
			gomega.Expect(payload.ExecutionPayload.ParentHash).To(gomega.Equal(block.Hash()))
			gomega.Expect(payload.ExecutionPayload.Timestamp).To(gomega.Equal(attributes.Timestamp))
			gomega.Expect(payload.ExecutionPayload.Withdrawals).NotTo(gomega.BeNil())
		}, ginkgo.SpecTimeout(engineTimeout), ginkgo.Label("behavior:engine:payload-build"))
	},
)
