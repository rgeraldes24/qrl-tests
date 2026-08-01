//go:build e2e

package validator_test

import (
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
	validatorPollInterval = 2 * time.Second
	validatorTimeout      = 30 * time.Minute
)

type liveSuite struct {
	session   *endtoendlive.Session
	beacon    *consensus.Client
	chain     validatorops.ChainContext
	key       ml_dsa_87.MLDSA87Key
	publicKey string
	validator consensus.Validator
}

var _ = ginkgo.Describe(
	"Validator lifecycle",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.Label("e2e", "live", "validator", "mutates-chain", "assertoor"),
	func() {
		var suite liveSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			suite.session, err = endtoendlive.Open(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(suite.session.Close)
			suite.beacon, err = consensus.New(suite.session.Participant.ConsensusURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite.chain, err = validatorops.Chain(ctx, suite.beacon)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite.key, err = validatorops.DeterministicKey(0x91)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			suite.publicKey = hexutil.Encode(suite.key.PublicKey().Marshal())
		})

		ginkgo.It("submits deposits for two distinct validators", func(ctx ginkgo.SpecContext) {
			maximum, err := suite.beacon.SpecUint(ctx, "MAX_EFFECTIVE_BALANCE")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			publicKeys := make([]string, 0, 2)
			for _, marker := range []byte{0x81, 0x82} {
				key, err := validatorops.DeterministicKey(marker)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				publicKeys = append(publicKeys, hexutil.Encode(key.PublicKey().Marshal()))

				_, err = validatorops.Deposit(ctx, suite.session, suite.beacon, key, maximum)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			}
			gomega.Eventually(func(g gomega.Gomega) {
				for _, publicKey := range publicKeys {
					validator, err := suite.beacon.Validator(ctx, publicKey)
					g.Expect(err).NotTo(gomega.HaveOccurred())
					g.Expect(validator.Balance).To(gomega.BeNumerically(">=", maximum))
				}
			}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())
		}, ginkgo.SpecTimeout(validatorTimeout), ginkgo.Label("assertoor:dev:dev-deposits"))

		ginkgo.It("submits a deposit and top-up and activates the validator", func(ctx ginkgo.SpecContext) {
			maximum, err := suite.beacon.SpecUint(ctx, "MAX_EFFECTIVE_BALANCE")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			first := maximum / 2
			second := maximum - first

			_, err = validatorops.Deposit(ctx, suite.session, suite.beacon, suite.key, first)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Eventually(func() uint64 {
				validator, err := suite.beacon.Validator(ctx, suite.publicKey)
				if err != nil {
					return 0
				}
				return validator.Balance
			}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(validatorPollInterval).Should(gomega.BeNumerically(">=", first))

			_, err = validatorops.Deposit(ctx, suite.session, suite.beacon, suite.key, second)
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
			"assertoor:pectra-dev:kurtosis:topup-deposits",
			"assertoor:stable:validator-lifecycle-test-v2",
			"assertoor:dev:validator-lifecycle-test",
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

			balanceBefore, err := suite.session.Client.BalanceAt(ctx, suite.session.Address, nil)
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
					if err != nil {
						break
					}
					exitIncluded = exitIncluded || contains(operations.VoluntaryExits, suite.validator.Index)
					for _, withdrawal := range operations.Withdrawals {
						if withdrawal.ValidatorIndex == suite.validator.Index {
							g.Expect(strings.EqualFold(withdrawal.Address, suite.session.Address.Hex())).To(gomega.BeTrue())
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
				balance, err := suite.session.Client.BalanceAt(ctx, suite.session.Address, nil)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(balance.Cmp(balanceBefore)).To(gomega.BeNumerically(">", 0))
			}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(validatorPollInterval).Should(gomega.Succeed())
		}, ginkgo.SpecTimeout(validatorTimeout), ginkgo.Label(
			"assertoor:stable:kurtosis:validator-exit-test",
			"assertoor:pectra-dev:kurtosis:voluntary-exits",
			"assertoor:stable:kurtosis:validator-withdrawal-test",
			"assertoor:stable:validator-lifecycle-test-v2",
		))
	},
)

func contains(values []uint64, target uint64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
