//go:build e2e

package beaconapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const beaconAPITimeout = 10 * time.Minute

type beaconNode struct {
	session *endtoendlive.Session
	client  *consensus.Client
}

var _ = ginkgo.Describe(
	"Beacon, node, and configuration APIs",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "consensus", "beacon-api"),
	func() {
		var nodes []beaconNode

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			sessions, err := endtoendlive.OpenAll(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			for _, session := range sessions {
				ginkgo.DeferCleanup(session.Close)
				client, err := consensus.New(session.Participant.ConsensusURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				nodes = append(nodes, beaconNode{session: session, client: client})
			}
		})

		ginkgo.It("returns coherent node and configuration metadata", func(ctx ginkgo.SpecContext) {
			var referenceGenesis consensus.Genesis
			var referenceDeposit consensus.DepositContract
			for index, node := range nodes {
				gomega.Expect(node.client.Health(ctx)).To(gomega.Succeed())

				var identity struct {
					Data struct {
						PeerID             string   `json:"peer_id"`
						ENR                string   `json:"enr"`
						P2PAddresses       []string `json:"p2p_addresses"`
						DiscoveryAddresses []string `json:"discovery_addresses"`
					} `json:"data"`
				}
				gomega.Expect(node.client.GetJSON(ctx, "/qrl/v1/node/identity", &identity)).To(gomega.Succeed())
				gomega.Expect(identity.Data.PeerID).NotTo(gomega.BeEmpty())
				gomega.Expect(identity.Data.P2PAddresses).NotTo(gomega.BeEmpty())

				var version struct {
					Data struct {
						Version string `json:"version"`
					} `json:"data"`
				}
				gomega.Expect(node.client.GetJSON(ctx, "/qrl/v1/node/version", &version)).To(gomega.Succeed())
				gomega.Expect(strings.ToLower(version.Data.Version)).To(gomega.HavePrefix("qrysm/"))

				var peers struct {
					Data []json.RawMessage `json:"data"`
				}
				gomega.Expect(node.client.GetJSON(ctx, "/qrl/v1/node/peers?state=connected", &peers)).To(gomega.Succeed())
				var peerCount struct {
					Data struct {
						Connected     string `json:"connected"`
						Connecting    string `json:"connecting"`
						Disconnected  string `json:"disconnected"`
						Disconnecting string `json:"disconnecting"`
					} `json:"data"`
				}
				gomega.Expect(node.client.GetJSON(ctx, "/qrl/v1/node/peer_count", &peerCount)).To(gomega.Succeed())
				connected, err := strconv.ParseUint(peerCount.Data.Connected, 10, 64)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(connected).To(gomega.Equal(uint64(len(peers.Data))))

				genesis, err := node.client.Genesis(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(genesis.Time).NotTo(gomega.BeZero())
				gomega.Expect(genesis.ValidatorsRoot).To(gomega.MatchRegexp(`^0x[0-9a-fA-F]{64}$`))
				deposit, err := node.client.DepositContract(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(deposit.ChainID).NotTo(gomega.BeZero())
				gomega.Expect(deposit.Address).To(gomega.MatchRegexp(`^Q[0-9a-fA-F]{128}$`))
				if index == 0 {
					referenceGenesis = genesis
					referenceDeposit = deposit
				} else {
					gomega.Expect(genesis).To(gomega.Equal(referenceGenesis))
					gomega.Expect(deposit).To(gomega.Equal(referenceDeposit))
				}

				var forkSchedule struct {
					Data []json.RawMessage `json:"data"`
				}
				gomega.Expect(node.client.GetJSON(ctx, "/qrl/v1/config/fork_schedule", &forkSchedule)).To(gomega.Succeed())
				gomega.Expect(forkSchedule.Data).NotTo(gomega.BeEmpty())
			}
		}, ginkgo.Label("behavior:consensus-api:metadata"))

		ginkgo.It("cross-links head block and state endpoints", func(ctx ginkgo.SpecContext) {
			for _, node := range nodes {
				head, err := node.client.Head(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				var blockRoot struct {
					Data struct {
						Root string `json:"root"`
					} `json:"data"`
				}
				gomega.Expect(node.client.GetJSON(ctx, "/qrl/v1/beacon/blocks/head/root", &blockRoot)).To(gomega.Succeed())
				gomega.Expect(blockRoot.Data.Root).To(gomega.Equal(head.Root))

				var header struct {
					Data struct {
						Header struct {
							Message struct {
								StateRoot string `json:"state_root"`
							} `json:"message"`
						} `json:"header"`
					} `json:"data"`
				}
				gomega.Expect(node.client.GetJSON(ctx, "/qrl/v1/beacon/headers/head", &header)).To(gomega.Succeed())
				var stateRoot struct {
					Data struct {
						Root string `json:"root"`
					} `json:"data"`
				}
				gomega.Expect(node.client.GetJSON(ctx, "/qrl/v1/beacon/states/head/root", &stateRoot)).To(gomega.Succeed())
				gomega.Expect(stateRoot.Data.Root).To(gomega.Equal(header.Data.Header.Message.StateRoot))

				var validators struct {
					Data []json.RawMessage `json:"data"`
				}
				gomega.Expect(node.client.GetJSON(ctx, "/qrl/v1/beacon/states/head/validators?status=active", &validators)).To(gomega.Succeed())
				active, err := node.client.ActiveValidatorCount(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(validators.Data).To(gomega.HaveLen(active))

				var balances struct {
					Data []json.RawMessage `json:"data"`
				}
				gomega.Expect(node.client.GetJSON(ctx, "/qrl/v1/beacon/states/head/validator_balances", &balances)).To(gomega.Succeed())
				gomega.Expect(balances.Data).To(gomega.HaveLen(active))

				var validatorCount struct {
					Data []struct {
						Status string `json:"status"`
						Count  string `json:"count"`
					} `json:"data"`
				}
				gomega.Expect(node.client.GetJSON(ctx, "/qrl/v1/beacon/states/head/validator_count?status=active", &validatorCount)).To(gomega.Succeed())
				count := uint64(0)
				for _, item := range validatorCount.Data {
					value, err := strconv.ParseUint(item.Count, 10, 64)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					count += value
				}
				gomega.Expect(count).To(gomega.Equal(uint64(active)))

				for _, path := range []string{
					"/qrl/v1/beacon/states/head/finality_checkpoints",
					"/qrl/v1/beacon/states/head/fork",
					"/qrl/v1/beacon/states/head/randao",
					"/qrl/v1/beacon/states/head/sync_committees",
					"/qrl/v1/beacon/states/head/committees?slot=" + strconv.FormatUint(head.Slot, 10),
				} {
					var response struct {
						Data json.RawMessage `json:"data"`
					}
					gomega.Expect(node.client.GetJSON(ctx, path, &response)).To(gomega.Succeed(), path)
					gomega.Expect(response.Data).NotTo(gomega.BeEmpty(), path)
				}
			}
		}, ginkgo.Label("behavior:consensus-api:beacon-state"))

		ginkgo.It("cryptographically verifies every signature in a live block", func(ctx ginkgo.SpecContext) {
			for _, node := range nodes {
				slotsPerEpoch, err := node.client.SpecUint(ctx, "SLOTS_PER_EPOCH")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Eventually(func() uint64 {
					slot, err := node.client.HeadSlot(ctx)
					if err != nil {
						return 0
					}
					return slot / slotsPerEpoch
				}).WithContext(ctx).WithTimeout(beaconAPITimeout).Should(gomega.BeNumerically(">=", 2))

				summary, err := node.client.VerifyBlockSignatures(ctx, "head")
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("participant %d", node.session.Participant.Index))
				gomega.Expect(summary.Block).To(gomega.Equal(1))
				gomega.Expect(summary.Randao).To(gomega.Equal(1))
				gomega.Expect(summary.Attestations).To(gomega.BeNumerically(">", 0))
				gomega.Expect(summary.SyncCommittee).To(gomega.BeNumerically(">", 0))
				gomega.Expect(summary.Total()).To(gomega.BeNumerically(">", 2))
			}
		}, ginkgo.SpecTimeout(beaconAPITimeout), ginkgo.Label("behavior:consensus-signatures:block"))

		ginkgo.It("returns the live operation pools", func(ctx ginkgo.SpecContext) {
			for _, node := range nodes {
				for _, path := range []string{
					"/qrl/v1/beacon/pool/attestations",
					"/qrl/v1/beacon/pool/voluntary_exits",
					"/qrl/v1/beacon/pool/proposer_slashings",
					"/qrl/v1/beacon/pool/attester_slashings",
				} {
					var response struct {
						Data []json.RawMessage `json:"data"`
					}
					gomega.Expect(node.client.GetJSON(ctx, path, &response)).To(gomega.Succeed(), path)
					gomega.Expect(response.Data).NotTo(gomega.BeNil(), path)
				}
			}
		}, ginkgo.Label("behavior:consensus-api:pools"))
	},
)
