//go:build e2e

package validator_test

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/cyyber/qrl-tests/endtoend/internal/stability"
	"github.com/cyyber/qrl-tests/endtoend/internal/validatorops"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerLifecycleSpec() {
	ginkgo.It("runs the supported validator lifecycle matrix", func(ctx ginkgo.SpecContext) {
		startFinalized, err := validatorOperations.beacon.FinalizedEpoch(ctx)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		validatorOperations.runLifecycleMatrix(ctx)
		gomega.Expect(stability.Await(
			ctx, validatorOperations.sessions, startFinalized, 2,
		)).To(gomega.Succeed())
	}, ginkgo.SpecTimeout(operationsTimeout), ginkgo.Label(
		"scenario:stable:validator-lifecycle-test-v2",
		"scenario:dev:validator-lifecycle-test",
		"behavior:validator:lifecycle-matrix",
		"behavior:validator:deposit-event-signature",
		"behavior:validator:partial-withdrawal",
		"behavior:network:post-workload-stability",
	))
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
		_, err = suite.depositor.Deposit(ctx, keys[index], amount)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}
	for _, index := range []int{0, 5} {
		_, err = suite.depositor.Deposit(ctx, keys[index], maximum/2)
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
	_, err = suite.depositor.Deposit(ctx, keys[partialValidator], maximum/2)
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
		ctx, suite.primary.Address, new(big.Int).SetUint64(partialPayload.BlockNumber),
	)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(balanceAfterWithdrawal).To(gomega.BeNumerically(">", balanceAfterTopUp))
	validatorAfterPartialWithdrawal, err := suite.beacon.Validator(
		ctx, strconv.FormatUint(validators[partialValidator].Index, 10),
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
