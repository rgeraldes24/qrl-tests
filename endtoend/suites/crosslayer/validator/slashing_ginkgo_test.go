//go:build e2e

package validator_test

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensuscontext"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/cyyber/qrl-tests/endtoend/internal/validatorops"
	"github.com/theQRL/go-qrl/common/hexutil"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe(
	"Validator slashings",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.Label("e2e", "live", "validator", "slashing", "mutates-chain", "scenario"),
	func() {
		var session *endtoendlive.Session
		var beacon *consensus.Client
		var chain consensuscontext.Context

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			runtime, loadErr := endtoendlive.Load(ctx)
			gomega.Expect(loadErr).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			session, err = runtime.Primary(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			beacon = session.Consensus
			chain, err = consensuscontext.Load(ctx, beacon)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("includes a proposer slashing and marks the validator slashed", func(ctx ginkgo.SpecContext) {
			assertSlashing(ctx, beacon, chain, 62, "/qrl/v1/beacon/pool/proposer_slashings", true)
		}, ginkgo.SpecTimeout(validatorTimeout), ginkgo.Label(
			"scenario:dev:validator-proposer-slashing-test",
			"scenario:dev:validator-slashing-single",
			"scenario:stable:kurtosis:validator-slashing-test",
			"scenario:dev:validator-lifecycle-test",
			"scenario:stable:validator-lifecycle-test-v2",
			"behavior:validator:proposer-slashing",
		))

		ginkgo.It("includes an attester slashing and marks the validator slashed", func(ctx ginkgo.SpecContext) {
			assertSlashing(ctx, beacon, chain, 63, "/qrl/v1/beacon/pool/attester_slashings", false)
		}, ginkgo.SpecTimeout(validatorTimeout), ginkgo.Label(
			"scenario:dev:validator-proposer-slashing-test",
			"scenario:stable:kurtosis:validator-slashing-test",
			"scenario:dev:validator-lifecycle-test",
			"scenario:stable:validator-lifecycle-test-v2",
			"behavior:validator:attester-slashing",
		))
	},
)

func assertSlashing(ctx ginkgo.SpecContext, beacon *consensus.Client, chain consensuscontext.Context, index uint64, path string, proposer bool) {
	key, err := validatorops.GenesisKey(index)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	validator, err := beacon.Validator(ctx, strconv.FormatUint(index, 10))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(validator.Slashed).To(gomega.BeFalse())
	gomega.Expect(strings.EqualFold(validator.PublicKey, hexutil.Encode(key.PublicKey().Marshal()))).To(gomega.BeTrue())
	initialBalance := validator.Balance
	initialWithdrawableEpoch := validator.WithdrawableEpoch

	head, err := beacon.Head(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	var operation any
	if proposer {
		operation, err = validatorops.ProposerSlashing(key, index, head.Slot, chain)
	} else {
		finalized, finalityErr := beacon.FinalizedEpoch(ctx)
		gomega.Expect(finalityErr).NotTo(gomega.HaveOccurred())
		operation, err = validatorops.AttesterSlashing(key, index, head.Slot, finalized, chain)
	}
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(beacon.Post(ctx, path, operation)).To(gomega.Succeed())

	lastSlot := head.Slot
	included := false
	gomega.Eventually(func(g gomega.Gomega) {
		current, err := beacon.HeadSlot(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		scannedThrough := lastSlot
		for slot := lastSlot + 1; slot <= current; slot++ {
			operations, err := beacon.BlockOperations(ctx, strconv.FormatUint(slot, 10))
			if consensus.IsNotFound(err) {
				scannedThrough = slot
				continue
			}
			if err != nil {
				g.Expect(err).NotTo(gomega.HaveOccurred())
				return
			}
			indices := operations.AttesterSlashings
			if proposer {
				indices = operations.ProposerSlashings
			}
			included = included || slices.Contains(indices, index)
			scannedThrough = slot
		}
		lastSlot = scannedThrough
		validator, err := beacon.Validator(ctx, strconv.FormatUint(index, 10))
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(included).To(gomega.BeTrue())
		g.Expect(validator.Slashed).To(gomega.BeTrue())
		g.Expect(validator.Balance).To(gomega.BeNumerically("<", initialBalance))
		g.Expect(validator.WithdrawableEpoch).NotTo(gomega.Equal(initialWithdrawableEpoch))
	}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(2 * time.Second).Should(gomega.Succeed())
}
