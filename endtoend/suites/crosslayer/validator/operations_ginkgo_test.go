//go:build e2e

package validator_test

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/cyyber/qrl-tests/endtoend/internal/stability"
	"github.com/cyyber/qrl-tests/endtoend/internal/validatorops"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	operationsTimeout        = 3 * time.Hour
	lifecycleValidatorCount  = 10
	massExitCount            = 64
	matrixSlashingCount      = 50
	minimumOperationsStake   = 512
	genesisParticipantCount  = 4
	validatorsPerParticipant = minimumOperationsStake / genesisParticipantCount
	massDepositCount         = 300
)

type operationsSuite struct {
	sessions          []*endtoendlive.Session
	beacons           []*consensus.Client
	primary           *endtoendlive.Session
	beacon            *consensus.Client
	chain             validatorops.ChainContext
	expectedProposers map[string]struct{}
	services          *devnet.ServiceController
}

var _ = ginkgo.Describe(
	"Validator operation workloads",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.Label(
		"e2e", "live", "validator", "mutates-chain", "scenario", "scenario-full", "profile-operations",
	),
	func() {
		var suite operationsSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			suite.sessions, err = endtoendlive.OpenAll(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(suite.sessions).To(gomega.HaveLen(5))
			for _, session := range suite.sessions {
				ginkgo.DeferCleanup(session.Close)
				beacon, err := consensus.New(session.Participant.ConsensusURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				suite.beacons = append(suite.beacons, beacon)
			}
			suite.primary = suite.sessions[0]
			suite.beacon = suite.beacons[0]
			suite.chain, err = validatorops.Chain(ctx, suite.beacon)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			active, err := suite.beacon.ActiveValidatorCount(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(active).To(gomega.BeNumerically(">=", minimumOperationsStake))
			suite.expectedProposers = expectedValidatorPairs(suite.sessions[:genesisParticipantCount])
			suite.services = devnet.NewServiceController(suite.primary.Environment.EnclaveName)
		})

		ginkgo.It("runs the supported validator lifecycle matrix", func(ctx ginkgo.SpecContext) {
			startFinalized, err := suite.beacon.FinalizedEpoch(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite.runLifecycleMatrix(ctx)
			gomega.Expect(stability.Await(ctx, suite.sessions, startFinalized, 2)).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(operationsTimeout), ginkgo.Label(
			"scenario:stable:validator-lifecycle-test-v2",
			"scenario:dev:validator-lifecycle-test",
			"behavior:validator:lifecycle-matrix",
			"behavior:validator:deposit-event-signature",
			"behavior:validator:partial-withdrawal",
			"behavior:network:post-workload-stability",
		))

		ginkgo.It("submits and includes 64 voluntary exits through every consensus client", func(ctx ginkgo.SpecContext) {
			startFinalized, err := suite.beacon.FinalizedEpoch(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite.runExitWorkload(ctx)
			gomega.Expect(stability.Await(ctx, suite.sessions, startFinalized, 2)).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(operationsTimeout), ginkgo.Label(
			"scenario:pectra-dev:kurtosis:voluntary-exits",
			"scenario:stable:kurtosis:validator-exit-test",
			"behavior:validator:voluntary-exit-64",
			"behavior:validator:operation-submission-matrix",
			"behavior:validator:exit-proposer-matrix",
			"behavior:network:post-workload-stability",
		))

		ginkgo.It("distributes proposer and attester slashings across every client pair", func(ctx ginkgo.SpecContext) {
			startFinalized, err := suite.beacon.FinalizedEpoch(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite.runSlashingWorkload(ctx)
			gomega.Expect(stability.Await(ctx, suite.sessions, startFinalized, 2)).To(gomega.Succeed())
		}, ginkgo.SpecTimeout(operationsTimeout), ginkgo.Label(
			"scenario:dev:validator-proposer-slashing-test",
			"scenario:stable:kurtosis:validator-slashing-test",
			"behavior:validator:proposer-slashing-matrix",
			"behavior:validator:attester-slashing-matrix",
			"behavior:validator:operation-submission-matrix",
			"behavior:validator:slashing-proposer-matrix",
			"behavior:network:post-workload-stability",
		))

		ginkgo.It("recovers finality after activating 300 initially offline validators", func(ctx ginkgo.SpecContext) {
			suite.runMassDepositChurn(ctx)
		}, ginkgo.SpecTimeout(operationsTimeout), ginkgo.Label(
			"scenario-full",
			"scenario:dev:validator-lifecycle-test",
			"behavior:validator:mass-deposit-churn",
			"behavior:validator:deposit-proposer-matrix",
			"behavior:network:post-workload-stability",
		))
	},
)

func (suite *operationsSuite) runMassDepositChurn(ctx ginkgo.SpecContext) {
	offline := suite.sessions[genesisParticipantCount]
	service := offline.Participant.ValidatorServiceName
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
		_, err = validatorops.Deposit(ctx, suite.primary, suite.beacon, key, maximum)
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
			graffiti, err := suite.beacon.BlockGraffiti(ctx, strconv.FormatUint(slot, 10))
			if consensus.IsNotFound(err) {
				continue
			}
			g.Expect(err).NotTo(gomega.HaveOccurred())
			if decoded, err := hex.DecodeString(strings.TrimPrefix(graffiti, "0x")); err == nil &&
				strings.TrimRight(string(decoded), "\x00") == wantedProposer {
				proposerSlot = slot
				return
			}
		}
		lastSlot = head
		g.Expect(proposerSlot).NotTo(gomega.BeZero())
	}).WithContext(ctx).WithTimeout(operationsTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())

	summary, err := suite.beacon.VerifyBlockSignatures(ctx, strconv.FormatUint(proposerSlot, 10))
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

func (suite *operationsSuite) runLifecycleMatrix(ctx ginkgo.SpecContext) {
	maximum, err := suite.beacon.SpecUint(ctx, "MAX_EFFECTIVE_BALANCE")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	keys := make([]ml_dsa_87.MLDSA87Key, lifecycleValidatorCount)
	publicKeys := make([]string, lifecycleValidatorCount)
	for index := range keys {
		keys[index], err = validatorops.DeterministicKey(0xa0 + byte(index))
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		publicKeys[index] = hexutil.Encode(keys[index].PublicKey().Marshal())
		amount := maximum
		if index == 0 || index == 5 {
			amount = maximum / 2
		}
		_, err = validatorops.Deposit(ctx, suite.primary, suite.beacon, keys[index], amount)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}
	for _, index := range []int{0, 5} {
		_, err = validatorops.Deposit(ctx, suite.primary, suite.beacon, keys[index], maximum/2)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}

	validators := make([]consensus.Validator, len(publicKeys))
	gomega.Eventually(func(g gomega.Gomega) {
		for index, publicKey := range publicKeys {
			validator, err := suite.beacon.Validator(ctx, publicKey)
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(validator.Balance).To(gomega.BeNumerically(">=", maximum))
			g.Expect(validator.EffectiveBalance).To(gomega.Equal(maximum))
			g.Expect(validator.Status).To(gomega.Equal("active_ongoing"))
			validators[index] = validator
		}
	}).WithContext(ctx).WithTimeout(operationsTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())

	partialValidator := 1
	partialScanner := newOperationScanner(ctx, suite.beacon)
	_, err = validatorops.Deposit(ctx, suite.primary, suite.beacon, keys[partialValidator], maximum/2)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	balanceAfterTopUp, err := suite.primary.Execution.BalanceAt(ctx, suite.primary.Address, nil)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	partialSlot := partialScanner.await(ctx, func(operations consensus.BlockOperations) bool {
		for _, withdrawal := range operations.Withdrawals {
			if withdrawal.ValidatorIndex == validators[partialValidator].Index {
				expected := "0x" + suite.primary.Address.Hex()[1:]
				gomega.Expect(strings.EqualFold(withdrawal.Address, expected)).To(gomega.BeTrue())
				gomega.Expect(withdrawal.Amount).To(gomega.BeNumerically(">", 0))
				return true
			}
		}
		return false
	})
	partialPayload, err := suite.beacon.BlockExecutionPayload(ctx, strconv.FormatUint(partialSlot, 10))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	balanceAfterWithdrawal, err := suite.primary.Execution.BalanceAt(
		ctx,
		suite.primary.Address,
		new(big.Int).SetUint64(partialPayload.BlockNumber),
	)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(balanceAfterWithdrawal).To(gomega.BeNumerically(">", balanceAfterTopUp))
	validatorAfterPartialWithdrawal, err := suite.beacon.Validator(
		ctx,
		strconv.FormatUint(validators[partialValidator].Index, 10),
	)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(validatorAfterPartialWithdrawal.Status).To(gomega.Equal("active_ongoing"))
	gomega.Expect(validatorAfterPartialWithdrawal.ExitEpoch).To(gomega.Equal(^uint64(0)))

	operationScanner := newOperationScanner(ctx, suite.beacon)
	suite.submitSlashing(ctx, suite.beacon, operationScanner, keys[2], validators[2].Index, false)
	suite.submitSlashing(ctx, suite.beacon, operationScanner, keys[7], validators[7].Index, true)

	committeePeriod, err := suite.beacon.SpecUint(ctx, "SHARD_COMMITTEE_PERIOD")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	exitableEpoch := uint64(0)
	for _, validator := range validators {
		if candidate := validator.ActivationEpoch + committeePeriod; candidate > exitableEpoch {
			exitableEpoch = candidate
		}
	}
	awaitEpoch(ctx, suite.beacon, suite.chain.SlotsPerEpoch, exitableEpoch)

	exitIndices := []int{0, 1, 4, 5, 8, 9}
	head, err := suite.beacon.Head(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	lifecycleStart := head.Slot
	for _, keyIndex := range exitIndices {
		suite.submitExit(ctx, suite.beacon, operationScanner, keys[keyIndex], validators[keyIndex].Index)
	}
	suite.submitSlashing(ctx, suite.beacon, operationScanner, keys[3], validators[3].Index, false)
	suite.submitSlashing(ctx, suite.beacon, operationScanner, keys[6], validators[6].Index, true)

	wantedWithdrawals := make(map[uint64]struct{}, len(exitIndices))
	for _, keyIndex := range exitIndices {
		wantedWithdrawals[validators[keyIndex].Index] = struct{}{}
	}
	observedWithdrawals := make(map[uint64]struct{}, len(wantedWithdrawals))
	lastSlot := lifecycleStart
	gomega.Eventually(func(g gomega.Gomega) {
		current, err := suite.beacon.HeadSlot(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		for slot := lastSlot + 1; slot <= current; slot++ {
			operations, err := suite.beacon.BlockOperations(ctx, strconv.FormatUint(slot, 10))
			if consensus.IsNotFound(err) {
				continue
			}
			g.Expect(err).NotTo(gomega.HaveOccurred())
			for _, withdrawal := range operations.Withdrawals {
				if _, ok := wantedWithdrawals[withdrawal.ValidatorIndex]; ok {
					expected := "0x" + suite.primary.Address.Hex()[1:]
					g.Expect(strings.EqualFold(withdrawal.Address, expected)).To(gomega.BeTrue())
					g.Expect(withdrawal.Amount).To(gomega.BeNumerically(">", 0))
					observedWithdrawals[withdrawal.ValidatorIndex] = struct{}{}
				}
			}
		}
		lastSlot = current
		g.Expect(observedWithdrawals).To(gomega.HaveLen(len(wantedWithdrawals)))
	}).WithContext(ctx).WithTimeout(operationsTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())
}
