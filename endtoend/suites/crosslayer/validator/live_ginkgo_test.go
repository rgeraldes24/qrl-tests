//go:build e2e

package validator_test

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/behavior"
	"github.com/cyyber/qrl-tests/endtoend/internal/clients/beacon"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscontext"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
	"github.com/cyyber/qrl-tests/endtoend/internal/validatorops"
	"github.com/theQRL/go-qrl/common/hexutil"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	validatorPollInterval = 2 * time.Second
	validatorTimeout      = 30 * time.Minute
)

type liveSuite struct {
	session   *endtoendlive.Session
	beacon    *beacon.Client
	chain     consensuscontext.Context
	depositor *validatorops.Depositor
	key       *validatorops.Key
	publicKey string
	validator beacon.Validator
}

var _ = ginkgo.Describe(
	"Validator lifecycle",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.Label("e2e", "live", "validator", "mutates-chain", "scenario"),
	func() {
		var suite liveSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			runtime := testsuite.LoadRuntime()
			suite.session, err = runtime.Primary(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite.beacon = suite.session.Consensus
			suite.chain, err = consensuscontext.Load(ctx, suite.beacon)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite.depositor, err = validatorops.NewDepositor(ctx, suite.session, suite.beacon, suite.chain)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite.key, err = validatorops.DeterministicKey(0x91)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite.publicKey = hexutil.Encode(suite.key.PublicKey())
		})

		ginkgo.It("submits deposits for two distinct validators", func(ctx ginkgo.SpecContext) {
			maximum, err := suite.beacon.SpecUint(ctx, "MAX_EFFECTIVE_BALANCE")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			publicKeys := make([]string, 0, 2)
			for _, marker := range []byte{0x81, 0x82} {
				key, err := validatorops.DeterministicKey(marker)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				publicKeys = append(publicKeys, hexutil.Encode(key.PublicKey()))

				_, err = suite.depositor.Deposit(ctx, key, maximum)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			}
			gomega.Eventually(func(g gomega.Gomega) {
				for _, publicKey := range publicKeys {
					validator, err := suite.beacon.Validator(ctx, publicKey)
					g.Expect(err).NotTo(gomega.HaveOccurred())
					g.Expect(validator.Balance).To(gomega.BeNumerically(">=", maximum))
				}
			}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())
		}, ginkgo.SpecTimeout(validatorTimeout), ginkgo.Label(
			"scenario:dev:dev-deposits",
			behavior.Name("validator:deposit-distinct"),
		))

		ginkgo.It("submits a deposit and top-up and activates the validator", func(ctx ginkgo.SpecContext) {
			maximum, err := suite.beacon.SpecUint(ctx, "MAX_EFFECTIVE_BALANCE")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			first := maximum / 2
			second := maximum - first

			_, err = suite.depositor.Deposit(ctx, suite.key, first)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			_, err = suite.depositor.Deposit(ctx, suite.key, second)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Eventually(func(g gomega.Gomega) {
				validator, err := suite.beacon.Validator(ctx, suite.publicKey)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(validator.Balance).To(gomega.BeNumerically(">=", maximum))
				g.Expect(validator.EffectiveBalance).To(gomega.Equal(maximum))
				g.Expect(validator.Status).To(gomega.Equal("active_ongoing"))
				suite.validator = validator
			}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())
		}, ginkgo.SpecTimeout(validatorTimeout), ginkgo.Label(
			"scenario:pectra-dev:kurtosis:topup-deposits",
			"scenario:stable:validator-lifecycle-test-v2",
			"scenario:dev:validator-lifecycle-test",
			behavior.Name("validator:deposit-topup-activate"),
		))

		ginkgo.It("exits the validator and transfers its withdrawal to execution", func(ctx ginkgo.SpecContext) {
			gomega.Expect(suite.validator.PublicKey).NotTo(gomega.BeEmpty())
			committeePeriod, err := suite.beacon.SpecUint(ctx, "SHARD_COMMITTEE_PERIOD")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Eventually(func() uint64 {
				head, err := suite.beacon.HeadSlot(ctx)
				if err != nil {
					return 0
				}
				return head / suite.chain.SlotsPerEpoch
			}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(validatorPollInterval).Should(
				gomega.BeNumerically(">=", suite.validator.ActivationEpoch+committeePeriod),
			)

			head, err := suite.beacon.Head(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			epoch := head.Slot / suite.chain.SlotsPerEpoch
			exit, err := validatorops.VoluntaryExit(suite.key, suite.validator.Index, epoch, suite.chain)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(suite.beacon.Post(ctx, "/qrl/v1/beacon/pool/voluntary_exits", exit)).To(gomega.Succeed())

			balanceBefore, err := suite.session.Execution.BalanceAt(ctx, suite.session.Address, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			lastSlot := head.Slot
			exitIncluded := false
			withdrawalIncluded := false
			gomega.Eventually(func(g gomega.Gomega) {
				current, err := suite.beacon.HeadSlot(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				scannedThrough := lastSlot
				for slot := lastSlot + 1; slot <= current; slot++ {
					operations, err := suite.beacon.BlockOperations(ctx, strconv.FormatUint(slot, 10))
					if beacon.IsNotFound(err) {
						scannedThrough = slot
						continue
					}
					if err != nil {
						g.Expect(err).NotTo(gomega.HaveOccurred())
						return
					}
					exitIncluded = exitIncluded || slices.Contains(operations.VoluntaryExits, suite.validator.Index)
					for _, withdrawal := range operations.Withdrawals {
						if withdrawal.ValidatorIndex == suite.validator.Index {
							expectedAddress := "0x" + suite.session.Address.Hex()[1:]
							g.Expect(strings.EqualFold(withdrawal.Address, expectedAddress)).To(gomega.BeTrue())
							withdrawalIncluded = true
						}
					}
					scannedThrough = slot
				}
				lastSlot = scannedThrough
				validator, err := suite.beacon.Validator(ctx, suite.publicKey)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(validator.ExitEpoch).NotTo(gomega.Equal(^uint64(0)))
				g.Expect(exitIncluded).To(gomega.BeTrue())
				g.Expect(withdrawalIncluded).To(gomega.BeTrue())
				balance, err := suite.session.Execution.BalanceAt(ctx, suite.session.Address, nil)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(balance.Cmp(balanceBefore)).To(gomega.BeNumerically(">", 0))
			}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())
		}, ginkgo.SpecTimeout(validatorTimeout), ginkgo.Label(
			"scenario:stable:kurtosis:validator-exit-test",
			"scenario:pectra-dev:kurtosis:voluntary-exits",
			"scenario:stable:kurtosis:validator-withdrawal-test",
			"scenario:stable:validator-lifecycle-test-v2",
			behavior.Name("validator:voluntary-exit"),
			behavior.Name("validator:exit-withdraw"),
		))
	},
)
