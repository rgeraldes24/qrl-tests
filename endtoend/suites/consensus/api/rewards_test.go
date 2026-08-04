// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package api

import (
	"math/big"
	"strconv"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

type blockRewardsResponse struct {
	Data struct {
		ProposerIndex     string `json:"proposer_index"`
		Total             string `json:"total"`
		Attestations      string `json:"attestations"`
		SyncAggregate     string `json:"sync_aggregate"`
		ProposerSlashings string `json:"proposer_slashings"`
		AttesterSlashings string `json:"attester_slashings"`
	} `json:"data"`
	ExecutionOptimistic bool `json:"execution_optimistic"`
}

type attestationRewardsResponse struct {
	Data struct {
		IdealRewards []struct {
			EffectiveBalance string `json:"effective_balance"`
			Head             string `json:"head"`
			Target           string `json:"target"`
			Source           string `json:"source"`
		} `json:"ideal_rewards"`
		TotalRewards []struct {
			ValidatorIndex string `json:"validator_index"`
			Head           string `json:"head"`
			Target         string `json:"target"`
			Source         string `json:"source"`
			InclusionDelay string `json:"inclusion_delay"`
		} `json:"total_rewards"`
	} `json:"data"`
	ExecutionOptimistic bool `json:"execution_optimistic"`
}

type syncCommitteeRewardsResponse struct {
	Data []struct {
		ValidatorIndex string `json:"validator_index"`
		Reward         string `json:"reward"`
	} `json:"data"`
	ExecutionOptimistic bool `json:"execution_optimistic"`
}

func registerBeaconRewards(nodes *[]beaconNode) {
	ginkgo.It("returns coherent block, attestation, and sync-committee rewards", func(ctx ginkgo.SpecContext) {
		for _, node := range *nodes {
			slotsPerEpoch, err := node.client.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Eventually(func() uint64 {
				slot, headErr := node.client.HeadSlot(ctx)
				if headErr != nil {
					return 0
				}
				return slot / slotsPerEpoch
			}).WithContext(ctx).WithTimeout(beaconAPITimeout).Should(gomega.BeNumerically(">=", 2))

			var block blockRewardsResponse
			gomega.Expect(node.client.GetJSON(
				ctx,
				"/qrl/v1/beacon/rewards/blocks/head",
				&block,
			)).To(gomega.Succeed())
			gomega.Expect(block.ExecutionOptimistic).To(gomega.BeFalse())
			_, err = strconv.ParseUint(block.Data.ProposerIndex, 10, 64)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			total := decimalInteger(block.Data.Total)
			parts := new(big.Int)
			for _, value := range []string{
				block.Data.Attestations,
				block.Data.SyncAggregate,
				block.Data.ProposerSlashings,
				block.Data.AttesterSlashings,
			} {
				parts.Add(parts, decimalInteger(value))
			}
			gomega.Expect(total).To(gomega.Equal(parts))

			head, err := node.client.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			epoch := head/slotsPerEpoch - 2
			var attestations attestationRewardsResponse
			gomega.Expect(node.client.PostJSON(
				ctx,
				"/qrl/v1/beacon/rewards/attestations/"+strconv.FormatUint(epoch, 10),
				[]string{},
				&attestations,
			)).To(gomega.Succeed())
			gomega.Expect(attestations.ExecutionOptimistic).To(gomega.BeFalse())
			gomega.Expect(attestations.Data.IdealRewards).NotTo(gomega.BeEmpty())
			gomega.Expect(attestations.Data.TotalRewards).NotTo(gomega.BeEmpty())
			for _, reward := range attestations.Data.IdealRewards {
				for _, value := range []string{reward.EffectiveBalance, reward.Head, reward.Target, reward.Source} {
					decimalInteger(value)
				}
			}
			seen := make(map[uint64]struct{}, len(attestations.Data.TotalRewards))
			for _, reward := range attestations.Data.TotalRewards {
				index, parseErr := strconv.ParseUint(reward.ValidatorIndex, 10, 64)
				gomega.Expect(parseErr).NotTo(gomega.HaveOccurred())
				seen[index] = struct{}{}
				for _, value := range []string{reward.Head, reward.Target, reward.Source, reward.InclusionDelay} {
					decimalInteger(value)
				}
			}
			gomega.Expect(seen).To(gomega.HaveLen(len(attestations.Data.TotalRewards)))

			var syncCommittee syncCommitteeRewardsResponse
			gomega.Expect(node.client.PostJSON(
				ctx,
				"/qrl/v1/beacon/rewards/sync_committee/head",
				[]string{},
				&syncCommittee,
			)).To(gomega.Succeed())
			gomega.Expect(syncCommittee.ExecutionOptimistic).To(gomega.BeFalse())
			gomega.Expect(syncCommittee.Data).NotTo(gomega.BeEmpty())
			for _, reward := range syncCommittee.Data {
				_, parseErr := strconv.ParseUint(reward.ValidatorIndex, 10, 64)
				gomega.Expect(parseErr).NotTo(gomega.HaveOccurred())
				decimalInteger(reward.Reward)
			}
		}
	}, ginkgo.SpecTimeout(beaconAPITimeout), ginkgo.Label("behavior:consensus-api:rewards"))
}

func decimalInteger(value string) *big.Int {
	ginkgo.GinkgoHelper()
	integer, ok := new(big.Int).SetString(value, 10)
	gomega.Expect(ok).To(gomega.BeTrue(), "invalid decimal integer %q", value)
	return integer
}
