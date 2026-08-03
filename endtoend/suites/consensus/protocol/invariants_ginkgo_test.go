//go:build e2e

package protocol_test

import (
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerProtocolInvariants(suite *protocolSuite) {
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
}
