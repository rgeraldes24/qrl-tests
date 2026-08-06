//go:build e2e

package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/behavior"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensusverify"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerBeaconRuntime(nodes *[]beaconNode) {
	ginkgo.It("cryptographically verifies every signature in a live block", func(ctx ginkgo.SpecContext) {
		for _, node := range *nodes {
			slotsPerEpoch, err := node.client.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Eventually(func() uint64 {
				slot, err := node.client.HeadSlot(ctx)
				if err != nil {
					return 0
				}
				return slot / slotsPerEpoch
			}).WithContext(ctx).WithTimeout(beaconAPITimeout).Should(gomega.BeNumerically(">=", 2))

			verifier, err := consensusverify.New(ctx, node.client)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			summary, err := verifier.VerifyBlock(ctx, "head")
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("participant %d", node.session.Participant.Index))
			gomega.Expect(summary.Block).To(gomega.Equal(1))
			gomega.Expect(summary.Randao).To(gomega.Equal(1))
			gomega.Expect(summary.Attestations).To(gomega.BeNumerically(">", 0))
			gomega.Expect(summary.SyncCommittee).To(gomega.BeNumerically(">", 0))
			gomega.Expect(summary.Total()).To(gomega.BeNumerically(">", 2))
		}
	}, ginkgo.SpecTimeout(beaconAPITimeout), ginkgo.Label(behavior.Name("consensus-signatures:block")))

	ginkgo.It("returns the live operation pools", func(ctx ginkgo.SpecContext) {
		for _, node := range *nodes {
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
	}, ginkgo.Label(behavior.Name("consensus-api:pools")))

	ginkgo.It("streams head, block, attestation, and finality events", func(ctx ginkgo.SpecContext) {
		events, failures, err := (*nodes)[0].client.Events(ctx, "head", "block", "attestation", "finalized_checkpoint")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		seen := make(map[string]bool)
		gomega.Eventually(func() bool {
			select {
			case event, ok := <-events:
				if !ok {
					return false
				}
				gomega.Expect(json.Valid(event.Data)).To(gomega.BeTrue())
				seen[event.Topic] = true
			case err, ok := <-failures:
				if ok {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				}
			default:
			}
			return seen["head"] && seen["block"] && seen["attestation"] && seen["finalized_checkpoint"]
		}).WithContext(ctx).WithTimeout(beaconAPITimeout).WithPolling(100 * time.Millisecond).Should(gomega.BeTrue())
	}, ginkgo.SpecTimeout(beaconAPITimeout), ginkgo.Label(behavior.Name("consensus-api:events")))
}
