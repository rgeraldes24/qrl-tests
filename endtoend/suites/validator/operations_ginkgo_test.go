//go:build e2e

package validator_test

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/cyyber/qrl-tests/endtoend/internal/validatorops"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	operationsTimeout       = 3 * time.Hour
	lifecycleValidatorCount = 10
	massExitCount           = 64
	matrixSlashingCount     = 50
	minimumOperationsStake  = 512
)

type operationsSuite struct {
	sessions          []*endtoendlive.Session
	beacons           []*consensus.Client
	primary           *endtoendlive.Session
	beacon            *consensus.Client
	chain             validatorops.ChainContext
	expectedProposers map[string]struct{}
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
			gomega.Expect(suite.sessions).To(gomega.HaveLen(4))
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
			suite.expectedProposers = expectedValidatorPairs(suite.sessions)
		})

		ginkgo.It("runs the supported validator lifecycle matrix", func(ctx ginkgo.SpecContext) {
			suite.runLifecycleMatrix(ctx)
		}, ginkgo.SpecTimeout(operationsTimeout), ginkgo.Label(
			"scenario:stable:validator-lifecycle-test-v2",
			"scenario:dev:validator-lifecycle-test",
			"behavior:validator:lifecycle-matrix",
		))

		ginkgo.It("submits and includes 64 voluntary exits through every consensus client", func(ctx ginkgo.SpecContext) {
			suite.runExitWorkload(ctx)
		}, ginkgo.SpecTimeout(operationsTimeout), ginkgo.Label(
			"scenario:pectra-dev:kurtosis:voluntary-exits",
			"scenario:stable:kurtosis:validator-exit-test",
			"behavior:validator:voluntary-exit-64",
			"behavior:validator:operation-submission-matrix",
			"behavior:validator:exit-proposer-matrix",
		))

		ginkgo.It("distributes proposer and attester slashings across every client pair", func(ctx ginkgo.SpecContext) {
			suite.runSlashingWorkload(ctx)
		}, ginkgo.SpecTimeout(operationsTimeout), ginkgo.Label(
			"scenario:dev:validator-proposer-slashing-test",
			"scenario:stable:kurtosis:validator-slashing-test",
			"behavior:validator:proposer-slashing-matrix",
			"behavior:validator:attester-slashing-matrix",
			"behavior:validator:operation-submission-matrix",
			"behavior:validator:slashing-proposer-matrix",
		))
	},
)

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
		start    uint64
		proposer bool
	}{
		{start: 64, proposer: true},
		{start: 64 + matrixSlashingCount, proposer: false},
	} {
		observedProposers := make(map[string]struct{})
		for offset := uint64(0); offset < matrixSlashingCount; offset++ {
			index := workload.start + offset
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
		return contains(operations.VoluntaryExits, index)
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
		return contains(indices, index)
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

	graffiti, err := beacon.BlockGraffiti(ctx, strconv.FormatUint(slot, 10))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	decoded, err := hex.DecodeString(strings.TrimPrefix(graffiti, "0x"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	observed[strings.TrimRight(string(decoded), "\x00")] = struct{}{}
}

func expectedValidatorPairs(sessions []*endtoendlive.Session) map[string]struct{} {
	expected := make(map[string]struct{}, len(sessions))
	for _, session := range sessions {
		name := strings.TrimPrefix(session.Participant.ValidatorServiceName, "vc-")
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
