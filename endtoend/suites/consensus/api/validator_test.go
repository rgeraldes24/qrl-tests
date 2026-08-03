//go:build e2e

package api

import (
	"strconv"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe(
	"Validator duties and liveness APIs",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "consensus", "validator-api"),
	func() {
		var session *endtoendlive.Session
		var client *consensus.Client

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			runtime, loadErr := endtoendlive.Load(ctx)
			gomega.Expect(loadErr).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			session, err = runtime.Primary(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			client = session.Consensus
		})

		ginkgo.It("returns coherent attester, proposer, and sync duties", func(ctx ginkgo.SpecContext) {
			head, err := client.Head(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			slotsPerEpoch, err := client.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			epoch := head.Slot / slotsPerEpoch
			indices, err := client.ActiveValidatorIndices(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(indices).NotTo(gomega.BeEmpty())
			request := make([]string, len(indices))
			for index, validatorIndex := range indices {
				request[index] = strconv.FormatUint(validatorIndex, 10)
			}

			var attester struct {
				DependentRoot string `json:"dependent_root"`
				Data          []struct {
					ValidatorIndex string `json:"validator_index"`
					Slot           string `json:"slot"`
					CommitteeIndex string `json:"committee_index"`
				} `json:"data"`
			}
			path := "/qrl/v1/validator/duties/attester/" + strconv.FormatUint(epoch, 10)
			gomega.Expect(client.PostJSON(ctx, path, request, &attester)).To(gomega.Succeed())
			gomega.Expect(attester.DependentRoot).To(gomega.MatchRegexp(`^0x[0-9a-fA-F]{64}$`))
			gomega.Expect(attester.Data).To(gomega.HaveLen(len(indices)))
			assertDutyIndices(attester.Data, indices, epoch, slotsPerEpoch)

			var proposer struct {
				DependentRoot string `json:"dependent_root"`
				Data          []struct {
					ValidatorIndex string `json:"validator_index"`
					Slot           string `json:"slot"`
				} `json:"data"`
			}
			path = "/qrl/v1/validator/duties/proposer/" + strconv.FormatUint(epoch, 10)
			gomega.Expect(client.GetJSON(ctx, path, &proposer)).To(gomega.Succeed())
			gomega.Expect(proposer.DependentRoot).To(gomega.MatchRegexp(`^0x[0-9a-fA-F]{64}$`))
			gomega.Expect(proposer.Data).NotTo(gomega.BeEmpty())
			for _, duty := range proposer.Data {
				slot, err := strconv.ParseUint(duty.Slot, 10, 64)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(slot / slotsPerEpoch).To(gomega.Equal(epoch))
				validatorIndex, err := strconv.ParseUint(duty.ValidatorIndex, 10, 64)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(indices).To(gomega.ContainElement(validatorIndex))
			}

			var syncDuties struct {
				Data []struct {
					ValidatorIndex                string   `json:"validator_index"`
					ValidatorSyncCommitteeIndices []string `json:"validator_sync_committee_indices"`
				} `json:"data"`
			}
			path = "/qrl/v1/validator/duties/sync/" + strconv.FormatUint(epoch, 10)
			gomega.Expect(client.PostJSON(ctx, path, request, &syncDuties)).To(gomega.Succeed())
			gomega.Expect(syncDuties.Data).NotTo(gomega.BeEmpty())
			for _, duty := range syncDuties.Data {
				validatorIndex, err := strconv.ParseUint(duty.ValidatorIndex, 10, 64)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(indices).To(gomega.ContainElement(validatorIndex))
				gomega.Expect(duty.ValidatorSyncCommitteeIndices).NotTo(gomega.BeEmpty())
			}
		}, ginkgo.Label("behavior:validator-api:duties"))

		ginkgo.It("returns exact liveness for every requested validator", func(ctx ginkgo.SpecContext) {
			slotsPerEpoch, err := client.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			head, err := client.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			epoch := head / slotsPerEpoch
			gomega.Expect(epoch).To(gomega.BeNumerically(">", 0))
			indices, err := client.ActiveValidatorIndices(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			liveness, err := client.Liveness(ctx, epoch-1, indices)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(liveness).To(gomega.HaveLen(len(indices)))
			seen := make(map[uint64]bool, len(liveness))
			for _, validator := range liveness {
				seen[validator.Index] = validator.IsLive
			}
			for _, index := range indices {
				gomega.Expect(seen).To(gomega.HaveKey(index))
				gomega.Expect(seen[index]).To(gomega.BeTrue())
			}
		}, ginkgo.Label("behavior:validator-api:liveness"))

		ginkgo.It("returns equivalent standard and legacy validator assignments", func(ctx ginkgo.SpecContext) {
			head, err := client.Head(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			slotsPerEpoch, err := client.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			epoch := head.Slot / slotsPerEpoch
			indices, err := client.ActiveValidatorIndices(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			request := make([]string, len(indices))
			for index, validatorIndex := range indices {
				request[index] = strconv.FormatUint(validatorIndex, 10)
			}

			var standard struct {
				Data []struct {
					ValidatorIndex string `json:"validator_index"`
					Slot           string `json:"slot"`
					CommitteeIndex string `json:"committee_index"`
				} `json:"data"`
			}
			path := "/qrl/v1/validator/duties/attester/" + strconv.FormatUint(epoch, 10)
			gomega.Expect(client.PostJSON(ctx, path, request, &standard)).To(gomega.Succeed())
			legacy, err := client.ValidatorAssignments(ctx, epoch)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			legacyByIndex := make(map[uint64]consensus.ValidatorAssignment, len(legacy))
			for _, assignment := range legacy {
				legacyByIndex[assignment.ValidatorIndex] = assignment
			}
			gomega.Expect(standard.Data).To(gomega.HaveLen(len(indices)))
			for _, duty := range standard.Data {
				validatorIndex, err := strconv.ParseUint(duty.ValidatorIndex, 10, 64)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				slot, err := strconv.ParseUint(duty.Slot, 10, 64)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				committee, err := strconv.ParseUint(duty.CommitteeIndex, 10, 64)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				assignment, found := legacyByIndex[validatorIndex]
				gomega.Expect(found).To(gomega.BeTrue())
				gomega.Expect(assignment.AttesterSlot).To(gomega.Equal(slot))
				gomega.Expect(assignment.CommitteeIndex).To(gomega.Equal(committee))
			}
		}, ginkgo.Label("behavior:validator-api:legacy-parity"))
	},
)

func assertDutyIndices(
	duties []struct {
		ValidatorIndex string `json:"validator_index"`
		Slot           string `json:"slot"`
		CommitteeIndex string `json:"committee_index"`
	},
	indices []uint64,
	epoch,
	slotsPerEpoch uint64,
) {
	ginkgo.GinkgoHelper()
	seen := make(map[uint64]struct{}, len(duties))
	for _, duty := range duties {
		validatorIndex, err := strconv.ParseUint(duty.ValidatorIndex, 10, 64)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		seen[validatorIndex] = struct{}{}
		slot, err := strconv.ParseUint(duty.Slot, 10, 64)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(slot / slotsPerEpoch).To(gomega.Equal(epoch))
		_, err = strconv.ParseUint(duty.CommitteeIndex, 10, 64)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}
	for _, index := range indices {
		gomega.Expect(seen).To(gomega.HaveKey(index))
	}
}
