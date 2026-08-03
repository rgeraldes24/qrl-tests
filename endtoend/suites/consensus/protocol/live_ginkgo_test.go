//go:build e2e

package protocol_test

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"math/bits"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensusverify"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	protocolTimeout      = 30 * time.Minute
	protocolPollInterval = 2 * time.Second
	maxMetricsMemory     = 2_000_000_000
	expectedFeeRecipient = "Q0838a121a6e4dd8a51e7437b152fabbc76a173f077132f2c2ed021c7b0991e70da4dba44e9ec00984a90f28dfb0aabbda1ddc9e98a76ab0acb6644c5e76fbbe8"
)

type protocolSuite struct {
	sessions      []*endtoendlive.Session
	beacons       []*consensus.Client
	slotsPerEpoch uint64
}

var _ = ginkgo.Describe(
	"Consensus protocol invariants",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.Label("e2e", "live", "consensus", "protocol"),
	func() {
		var suite protocolSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			runtime, loadErr := endtoendlive.Load(ctx)
			gomega.Expect(loadErr).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			suite.sessions, err = runtime.OpenAll(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			for _, session := range suite.sessions {
				suite.beacons = append(suite.beacons, session.Consensus)
			}
			suite.slotsPerEpoch, err = suite.beacons[0].SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Eventually(func(g gomega.Gomega) {
				head, err := suite.beacons[0].HeadSlot(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(head / suite.slotsPerEpoch).To(gomega.BeNumerically(">=", 3))
			}).WithContext(ctx).WithTimeout(protocolTimeout).WithPolling(protocolPollInterval).Should(gomega.Succeed())
		})

		ginkgo.It("preserves genesis validator invariants on every client", func(ctx ginkgo.SpecContext) {
			firstGenesis, err := suite.beacons[0].Genesis(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			for _, beacon := range suite.beacons[1:] {
				genesis, err := beacon.Genesis(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(genesis).To(gomega.Equal(firstGenesis))
			}

			maximum, err := suite.beacons[0].SpecUint(ctx, "MAX_EFFECTIVE_BALANCE")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			farFuture := ^uint64(0)
			validators, err := suite.beacons[0].Validators(ctx, "active")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(len(validators)).To(gomega.BeNumerically(">=", 64))
			for _, validator := range validators {
				gomega.Expect(validator.EffectiveBalance).To(gomega.Equal(maximum))
				gomega.Expect(validator.ExitEpoch).To(gomega.Equal(farFuture))
				gomega.Expect(validator.WithdrawableEpoch).To(gomega.Equal(farFuture))
				gomega.Expect(validator.Slashed).To(gomega.BeFalse())
				publicKey, err := hexutil.Decode(validator.PublicKey)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(publicKey).To(gomega.HaveLen(2592))
				withdrawal, err := hexutil.Decode(validator.Withdrawal)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(withdrawal).To(gomega.HaveLen(common.AddressLength))
				if validator.Index < 64 {
					gomega.Expect(validator.ActivationEpoch).To(gomega.BeZero())
				}
			}
		}, ginkgo.SpecTimeout(protocolTimeout), ginkgo.Label("behavior:consensus:genesis-invariants"))

		ginkgo.It("connects every participant and exposes healthy client metrics", func(ctx ginkgo.SpecContext) {
			for index, beacon := range suite.beacons {
				var peers struct {
					Data []struct {
						PeerID string `json:"peer_id"`
						State  string `json:"state"`
					} `json:"data"`
				}
				gomega.Expect(beacon.GetJSON(ctx, "/qrl/v1/node/peers?state=connected", &peers)).To(gomega.Succeed())
				if len(suite.beacons) > 1 {
					gomega.Expect(len(peers.Data)).To(gomega.BeNumerically(">=", len(suite.beacons)-1))
				}
				seen := make(map[string]struct{}, len(peers.Data))
				for _, peer := range peers.Data {
					gomega.Expect(peer.State).To(gomega.Equal("connected"))
					gomega.Expect(peer.PeerID).NotTo(gomega.BeEmpty())
					_, duplicate := seen[peer.PeerID]
					gomega.Expect(duplicate).To(gomega.BeFalse())
					seen[peer.PeerID] = struct{}{}
				}

				beaconMetrics := readMetrics(ctx, suite.sessions[index].Participant.Consensus.MetricsURL)
				head, err := beacon.HeadSlot(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				metricHead := singleMetric(beaconMetrics, "beacon_head_slot")
				gomega.Expect(metricHead).To(gomega.BeNumerically("<=", float64(head)))
				gomega.Expect(float64(head) - metricHead).To(gomega.BeNumerically("<=", 2))
				gomega.Expect(singleMetric(beaconMetrics, "go_memstats_alloc_bytes")).To(gomega.BeNumerically("<", maxMetricsMemory))

				validatorMetrics := readMetrics(ctx, suite.sessions[index].Participant.Validator.MetricsURL)
				gomega.Expect(metricCount(validatorMetrics, "validator_statuses")).To(gomega.BeNumerically(">", 0))
				gomega.Expect(metricSum(validatorMetrics, "validator_successful_attestations")).To(gomega.BeNumerically(">", 0))
			}
		}, ginkgo.SpecTimeout(protocolTimeout), ginkgo.Label(
			"behavior:consensus:peer-topology",
			"behavior:consensus:metrics",
		))

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
			"behavior:consensus:execution-data-votes",
			"behavior:consensus:fee-recipients",
			"behavior:consensus:sync-committee-participation",
			"behavior:consensus:signature-verification",
		))
	},
)

func (suite *protocolSuite) previousEpoch(ctx context.Context) (uint64, uint64) {
	ginkgo.GinkgoHelper()

	head, err := suite.beacons[0].HeadSlot(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	epoch := head / suite.slotsPerEpoch
	gomega.Expect(epoch).To(gomega.BeNumerically(">", 0))
	start := (epoch - 1) * suite.slotsPerEpoch
	return start, start + suite.slotsPerEpoch
}

func readMetrics(ctx context.Context, endpoint string) map[string]*dto.MetricFamily {
	ginkgo.GinkgoHelper()
	gomega.Expect(endpoint).NotTo(gomega.BeEmpty())
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+"/metrics", nil)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	response, err := http.DefaultClient.Do(request)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	defer response.Body.Close()
	gomega.Expect(response.StatusCode).To(gomega.Equal(http.StatusOK))
	parser := expfmt.TextParser{}
	families, err := parser.TextToMetricFamilies(io.LimitReader(response.Body, 32<<20))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return families
}

func singleMetric(families map[string]*dto.MetricFamily, name string) float64 {
	ginkgo.GinkgoHelper()
	values := metricValues(families, name)
	gomega.Expect(values).To(gomega.HaveLen(1), fmt.Sprintf("metric %s", name))
	return values[0]
}

func metricCount(families map[string]*dto.MetricFamily, name string) int {
	return len(metricValues(families, name))
}

func metricSum(families map[string]*dto.MetricFamily, name string) float64 {
	values := metricValues(families, name)
	gomega.Expect(values).NotTo(gomega.BeEmpty(), fmt.Sprintf("metric %s", name))
	var total float64
	for _, value := range values {
		total += value
	}
	return total
}

func metricValues(families map[string]*dto.MetricFamily, name string) []float64 {
	family := families[name]
	if family == nil {
		return nil
	}
	values := make([]float64, 0, len(family.Metric))
	for _, metric := range family.Metric {
		switch family.GetType() {
		case dto.MetricType_COUNTER:
			values = append(values, metric.GetCounter().GetValue())
		case dto.MetricType_GAUGE:
			values = append(values, metric.GetGauge().GetValue())
		case dto.MetricType_UNTYPED:
			values = append(values, metric.GetUntyped().GetValue())
		}
	}
	return values
}
