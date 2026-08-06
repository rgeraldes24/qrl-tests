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

var _ = ginkgo.Describe(
	"Validator slashings",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.Label("e2e", "live", "validator", "slashing", "mutates-chain", "scenario"),
	func() {
		var session *endtoendlive.Session
		var beaconClient *beacon.Client
		var chain consensuscontext.Context

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			runtime := testsuite.LoadRuntime()
			session, err = runtime.Primary(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			beaconClient = session.Consensus
			chain, err = consensuscontext.Load(ctx, beaconClient)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("includes a proposer slashing and marks the validator slashed", func(ctx ginkgo.SpecContext) {
			assertSlashing(ctx, beaconClient, chain, 62, "/qrl/v1/beacon/pool/proposer_slashings", true)
		}, ginkgo.SpecTimeout(validatorTimeout), ginkgo.Label(
			"scenario:dev:validator-proposer-slashing-test",
			"scenario:dev:validator-slashing-single",
			"scenario:stable:kurtosis:validator-slashing-test",
			"scenario:dev:validator-lifecycle-test",
			"scenario:stable:validator-lifecycle-test-v2",
			behavior.Name("validator:proposer-slashing"),
		))

		ginkgo.It("includes an attester slashing and marks the validator slashed", func(ctx ginkgo.SpecContext) {
			assertSlashing(ctx, beaconClient, chain, 63, "/qrl/v1/beacon/pool/attester_slashings", false)
		}, ginkgo.SpecTimeout(validatorTimeout), ginkgo.Label(
			"scenario:dev:validator-proposer-slashing-test",
			"scenario:stable:kurtosis:validator-slashing-test",
			"scenario:dev:validator-lifecycle-test",
			"scenario:stable:validator-lifecycle-test-v2",
			behavior.Name("validator:attester-slashing"),
		))
	},
)

func assertSlashing(ctx ginkgo.SpecContext, beaconClient *beacon.Client, chain consensuscontext.Context, index uint64, path string, proposer bool) {
	key, err := validatorops.GenesisKey(index)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	validator, err := beaconClient.Validator(ctx, strconv.FormatUint(index, 10))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(validator.Slashed).To(gomega.BeFalse())
	gomega.Expect(strings.EqualFold(validator.PublicKey, hexutil.Encode(key.PublicKey()))).To(gomega.BeTrue())
	initialBalance := validator.Balance
	initialWithdrawableEpoch := validator.WithdrawableEpoch

	head, err := beaconClient.Head(ctx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	var operation any
	if proposer {
		operation, err = validatorops.ProposerSlashing(key, index, head.Slot, chain)
	} else {
		finalized, finalityErr := beaconClient.FinalizedEpoch(ctx)
		gomega.Expect(finalityErr).NotTo(gomega.HaveOccurred())
		operation, err = validatorops.AttesterSlashing(key, index, head.Slot, finalized, chain)
	}
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(beaconClient.Post(ctx, path, operation)).To(gomega.Succeed())

	lastSlot := head.Slot
	included := false
	gomega.Eventually(func(g gomega.Gomega) {
		current, err := beaconClient.HeadSlot(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		scannedThrough := lastSlot
		for slot := lastSlot + 1; slot <= current; slot++ {
			operations, err := beaconClient.BlockOperations(ctx, strconv.FormatUint(slot, 10))
			if beacon.IsNotFound(err) {
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
		validator, err := beaconClient.Validator(ctx, strconv.FormatUint(index, 10))
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(included).To(gomega.BeTrue())
		g.Expect(validator.Slashed).To(gomega.BeTrue())
		g.Expect(validator.Balance).To(gomega.BeNumerically("<", initialBalance))
		g.Expect(validator.WithdrawableEpoch).NotTo(gomega.Equal(initialWithdrawableEpoch))
	}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(2 * time.Second).Should(gomega.Succeed())
}
