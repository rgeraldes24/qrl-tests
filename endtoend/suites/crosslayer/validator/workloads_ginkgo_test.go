//go:build e2e

package validator_test

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/cyyber/qrl-tests/endtoend/internal/validatorops"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func (suite *operationsSuite) runExitWorkload(ctx ginkgo.SpecContext) {
	scanner := newOperationScanner(ctx, suite.beacon)
	observedProposers := make(map[string]struct{})
	usedClients := make(map[int]struct{})
	for index := uint64(0); index < massExitCount; index++ {
		key, err := validatorops.GenesisKey(index)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		clientIndex := int(index) % len(suite.beacons)
		usedClients[clientIndex] = struct{}{}
		slot := suite.submitExit(ctx, suite.beacons[clientIndex], scanner, key, index)
		recordOperationProposer(ctx, suite.beacon, slot, observedProposers)
	}
	gomega.Expect(usedClients).To(gomega.HaveLen(len(suite.beacons)))
	gomega.Expect(containsAll(observedProposers, suite.expectedProposers)).To(gomega.BeTrue())
}

func (suite *operationsSuite) runSlashingWorkload(ctx ginkgo.SpecContext) {
	scanner := newOperationScanner(ctx, suite.beacon)
	usedClients := make(map[int]struct{})
	for _, workload := range []struct {
		offset   uint64
		proposer bool
	}{
		{offset: 0, proposer: true},
		{offset: 16, proposer: false},
	} {
		observedProposers := make(map[string]struct{})
		for offset := uint64(0); offset < matrixSlashingCount; offset++ {
			participant := offset % genesisParticipantCount
			index := participant*validatorsPerParticipant + workload.offset + offset/genesisParticipantCount
			if participant == 0 {
				index += massExitCount
			}
			key, err := validatorops.GenesisKey(index)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			clientIndex := int(offset) % len(suite.beacons)
			usedClients[clientIndex] = struct{}{}
			slot := suite.submitSlashing(
				ctx, suite.beacons[clientIndex], scanner, key, index, workload.proposer,
			)
			recordOperationProposer(ctx, suite.beacon, slot, observedProposers)
		}
		gomega.Expect(containsAll(observedProposers, suite.expectedProposers)).To(gomega.BeTrue())
	}
	gomega.Expect(usedClients).To(gomega.HaveLen(len(suite.beacons)))
}

func (suite *operationsSuite) submitExit(
	ctx ginkgo.SpecContext,
	submitClient *consensus.Client,
	scanner *operationScanner,
	key ml_dsa_87.MLDSA87Key,
	index uint64,
) uint64 {
	ginkgo.GinkgoHelper()

	validator, err := suite.beacon.Validator(ctx, strconv.FormatUint(index, 10))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(validator.ExitEpoch).To(gomega.Equal(^uint64(0)))
	head, err := suite.beacon.Head(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	exit, err := validatorops.VoluntaryExit(key, index, head.Slot/suite.chain.SlotsPerEpoch, suite.chain)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(submitClient.Post(ctx, "/qrl/v1/beacon/pool/voluntary_exits", exit)).To(gomega.Succeed())
	slot := scanner.await(ctx, func(operations consensus.BlockOperations) bool {
		return slices.Contains(operations.VoluntaryExits, index)
	})
	waitValidator(ctx, suite.beacon, index, func(validator consensus.Validator) bool {
		return validator.ExitEpoch != ^uint64(0)
	})
	return slot
}

func (suite *operationsSuite) submitSlashing(
	ctx ginkgo.SpecContext,
	submitClient *consensus.Client,
	scanner *operationScanner,
	key ml_dsa_87.MLDSA87Key,
	index uint64,
	proposer bool,
) uint64 {
	ginkgo.GinkgoHelper()

	validator, err := suite.beacon.Validator(ctx, strconv.FormatUint(index, 10))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(validator.Slashed).To(gomega.BeFalse())
	gomega.Expect(strings.EqualFold(validator.PublicKey, hexutil.Encode(key.PublicKey().Marshal()))).To(gomega.BeTrue())
	head, err := suite.beacon.Head(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	var operation any
	var path string
	if proposer {
		operation, err = validatorops.ProposerSlashing(key, index, head.Slot, suite.chain)
		path = "/qrl/v1/beacon/pool/proposer_slashings"
	} else {
		finalized, finalityErr := suite.beacon.FinalizedEpoch(ctx)
		gomega.Expect(finalityErr).NotTo(gomega.HaveOccurred())
		operation, err = validatorops.AttesterSlashing(key, index, head.Slot, finalized, suite.chain)
		path = "/qrl/v1/beacon/pool/attester_slashings"
	}
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(submitClient.Post(ctx, path, operation)).To(gomega.Succeed())
	slot := scanner.await(ctx, func(operations consensus.BlockOperations) bool {
		indices := operations.AttesterSlashings
		if proposer {
			indices = operations.ProposerSlashings
		}
		return slices.Contains(indices, index)
	})
	waitValidator(ctx, suite.beacon, index, func(validator consensus.Validator) bool { return validator.Slashed })
	return slot
}

type operationScanner struct {
	beacon   *consensus.Client
	nextSlot uint64
}

func newOperationScanner(ctx ginkgo.SpecContext, beacon *consensus.Client) *operationScanner {
	ginkgo.GinkgoHelper()

	head, err := beacon.HeadSlot(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return &operationScanner{beacon: beacon, nextSlot: head + 1}
}

func (scanner *operationScanner) await(
	ctx ginkgo.SpecContext,
	match func(consensus.BlockOperations) bool,
) uint64 {
	ginkgo.GinkgoHelper()

	var found uint64
	gomega.Eventually(func(g gomega.Gomega) {
		head, err := scanner.beacon.HeadSlot(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		for slot := scanner.nextSlot; slot <= head; slot++ {
			operations, err := scanner.beacon.BlockOperations(ctx, strconv.FormatUint(slot, 10))
			if consensus.IsNotFound(err) {
				scanner.nextSlot = slot + 1
				continue
			}
			g.Expect(err).NotTo(gomega.HaveOccurred())
			scanner.nextSlot = slot + 1
			if match(operations) {
				found = slot
				return
			}
		}
		g.Expect(found).NotTo(gomega.BeZero())
	}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())
	return found
}

func recordOperationProposer(
	ctx ginkgo.SpecContext,
	beacon *consensus.Client,
	slot uint64,
	observed map[string]struct{},
) {
	ginkgo.GinkgoHelper()

	graffiti, err := beacon.BlockGraffitiText(ctx, strconv.FormatUint(slot, 10))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	observed[graffiti] = struct{}{}
}

func expectedValidatorPairs(sessions []*endtoendlive.Session) map[string]struct{} {
	expected := make(map[string]struct{}, len(sessions))
	for _, session := range sessions {
		name := strings.TrimPrefix(session.Participant.Validator.Name, "vc-")
		gomega.Expect(name).NotTo(gomega.BeEmpty())
		expected[name] = struct{}{}
	}
	return expected
}

func containsAll(observed, expected map[string]struct{}) bool {
	for value := range expected {
		if _, ok := observed[value]; !ok {
			return false
		}
	}
	return true
}

func awaitEpoch(ctx ginkgo.SpecContext, beacon *consensus.Client, slotsPerEpoch, epoch uint64) {
	ginkgo.GinkgoHelper()

	gomega.Eventually(func() uint64 {
		slot, _ := beacon.HeadSlot(ctx)
		return slot / slotsPerEpoch
	}).WithContext(ctx).WithTimeout(operationsTimeout).WithPolling(validatorPollInterval).Should(
		gomega.BeNumerically(">=", epoch),
	)
}

func waitValidator(
	ctx ginkgo.SpecContext,
	beacon *consensus.Client,
	index uint64,
	match func(consensus.Validator) bool,
) {
	ginkgo.GinkgoHelper()

	gomega.Eventually(func(g gomega.Gomega) {
		validator, err := beacon.Validator(ctx, strconv.FormatUint(index, 10))
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(match(validator)).To(gomega.BeTrue(), fmt.Sprintf("validator %d did not reach the expected state", index))
	}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())
}
