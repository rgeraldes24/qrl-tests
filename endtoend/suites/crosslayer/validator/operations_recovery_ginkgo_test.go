//go:build e2e

package validator_test

import (
	"strconv"
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensusverify"
	"github.com/cyyber/qrl-tests/endtoend/internal/stability"
	"github.com/cyyber/qrl-tests/endtoend/internal/validatorops"
	"github.com/theQRL/go-qrl/common/hexutil"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerRecoverySpec() {
	ginkgo.It("recovers finality after activating 300 initially offline validators", func(ctx ginkgo.SpecContext) {
		validatorOperations.runMassDepositChurn(ctx)
	}, ginkgo.SpecTimeout(operationsTimeout), ginkgo.Label(
		"scenario-full",
		"scenario:dev:validator-lifecycle-test",
		"behavior:validator:mass-deposit-churn",
		"behavior:validator:deposit-proposer-matrix",
		"behavior:network:post-workload-stability",
	))
}

func (suite *operationsSuite) runMassDepositChurn(ctx ginkgo.SpecContext) {
	offline := suite.sessions[genesisParticipantCount]
	service := offline.Participant.Validator.Name
	gomega.Expect(suite.services.Stop(ctx, service)).To(gomega.Succeed())
	ginkgo.DeferCleanup(func(cleanupCtx ginkgo.SpecContext) {
		_ = suite.services.Start(cleanupCtx, service)
	})

	publicKeys := make([]string, massDepositCount)
	validatorIndices := make([]uint64, massDepositCount)
	maximum := suite.maximumBalance(ctx)
	for offset := 0; offset < massDepositCount; offset++ {
		keyIndex := uint64(minimumOperationsStake + offset)
		key, err := validatorops.GenesisKey(keyIndex)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		publicKeys[offset] = hexutil.Encode(key.PublicKey().Marshal())
		_, err = suite.depositor.Deposit(ctx, key, maximum)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}

	gomega.Eventually(func(g gomega.Gomega) {
		for index, publicKey := range publicKeys {
			validator, err := suite.beacon.Validator(ctx, publicKey)
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(validator.Status).To(gomega.Equal("active_ongoing"))
			validatorIndices[index] = validator.Index
		}
	}).WithContext(ctx).WithTimeout(operationsTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())

	stalled := suite.awaitFinalityStall(ctx)
	gomega.Expect(suite.services.Start(ctx, service)).To(gomega.Succeed())

	gomega.Eventually(func(g gomega.Gomega) {
		head, err := suite.beacon.HeadSlot(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		epoch := head / suite.chain.SlotsPerEpoch
		g.Expect(epoch).To(gomega.BeNumerically(">", 0))
		liveness, err := suite.beacon.Liveness(ctx, epoch-1, validatorIndices)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(liveness).To(gomega.HaveLen(len(validatorIndices)))
		for _, item := range liveness {
			g.Expect(item.IsLive).To(gomega.BeTrue())
		}
	}).WithContext(ctx).WithTimeout(operationsTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())

	wantedProposer := strings.TrimPrefix(service, "vc-")
	lastSlot, err := suite.beacon.HeadSlot(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	var proposerSlot uint64
	gomega.Eventually(func(g gomega.Gomega) {
		head, err := suite.beacon.HeadSlot(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		for slot := lastSlot + 1; slot <= head; slot++ {
			graffiti, err := suite.beacon.BlockGraffitiText(ctx, strconv.FormatUint(slot, 10))
			if consensus.IsNotFound(err) {
				continue
			}
			g.Expect(err).NotTo(gomega.HaveOccurred())
			if graffiti == wantedProposer {
				proposerSlot = slot
				return
			}
		}
		lastSlot = head
		g.Expect(proposerSlot).NotTo(gomega.BeZero())
	}).WithContext(ctx).WithTimeout(operationsTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())

	verifier, err := consensusverify.New(ctx, suite.beacon)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	summary, err := verifier.VerifyBlock(ctx, strconv.FormatUint(proposerSlot, 10))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(summary.Block).To(gomega.Equal(1))
	gomega.Expect(summary.Randao).To(gomega.Equal(1))
	gomega.Expect(stability.Await(ctx, suite.sessions, stalled.Epoch, 2)).To(gomega.Succeed())
}

func (suite *operationsSuite) maximumBalance(ctx ginkgo.SpecContext) uint64 {
	ginkgo.GinkgoHelper()
	maximum, err := suite.beacon.SpecUint(ctx, "MAX_EFFECTIVE_BALANCE")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return maximum
}

func (suite *operationsSuite) awaitFinalityStall(ctx ginkgo.SpecContext) consensus.Checkpoint {
	ginkgo.GinkgoHelper()

	last, err := suite.beacon.FinalizedCheckpoint(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	lastChangeSlot, err := suite.beacon.HeadSlot(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Eventually(func(g gomega.Gomega) {
		checkpoint, err := suite.beacon.FinalizedCheckpoint(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		head, err := suite.beacon.HeadSlot(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		if checkpoint != last {
			last = checkpoint
			lastChangeSlot = head
		}
		g.Expect(head).To(gomega.BeNumerically(">=", lastChangeSlot+3*suite.chain.SlotsPerEpoch))
	}).WithContext(ctx).WithTimeout(operationsTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())
	return last
}
