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

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe(
	"Validator slashings",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.Label("e2e", "live", "validator", "slashing", "mutates-chain", "assertoor"),
	func() {
		var session *endtoendlive.Session
		var beacon *consensus.Client
		var chain validatorops.ChainContext

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			session, err = endtoendlive.Open(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(session.Close)
			beacon, err = consensus.New(session.Participant.ConsensusURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			chain, err = validatorops.Chain(ctx, beacon)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("includes a proposer slashing and marks the validator slashed", func(ctx ginkgo.SpecContext) {
			assertSlashing(ctx, beacon, chain, 62, "/qrl/v1/beacon/pool/proposer_slashings", true)
		}, ginkgo.SpecTimeout(validatorTimeout), ginkgo.Label(
			"assertoor:dev:validator-proposer-slashing-test",
			"assertoor:dev:validator-slashing-single",
			"assertoor:stable:kurtosis:validator-slashing-test",
			"assertoor:dev:validator-lifecycle-test",
			"assertoor:stable:validator-lifecycle-test-v2",
		))

		ginkgo.It("includes an attester slashing and marks the validator slashed", func(ctx ginkgo.SpecContext) {
			assertSlashing(ctx, beacon, chain, 63, "/qrl/v1/beacon/pool/attester_slashings", false)
		}, ginkgo.SpecTimeout(validatorTimeout), ginkgo.Label(
			"assertoor:dev:validator-proposer-slashing-test",
			"assertoor:stable:kurtosis:validator-slashing-test",
			"assertoor:dev:validator-lifecycle-test",
			"assertoor:stable:validator-lifecycle-test-v2",
		))
	},
)

func assertSlashing(ctx ginkgo.SpecContext, beacon *consensus.Client, chain validatorops.ChainContext, index uint64, path string, proposer bool) {
	key, err := validatorops.GenesisKey(index)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	validator, err := beacon.Validator(ctx, strconv.FormatUint(index, 10))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(validator.Slashed).To(gomega.BeFalse())
	gomega.Expect(strings.EqualFold(validator.PublicKey, hexutil.Encode(key.PublicKey().Marshal()))).To(gomega.BeTrue())

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
			included = included || contains(indices, index)
			scannedThrough = slot
		}
		lastSlot = scannedThrough
		validator, err := beacon.Validator(ctx, strconv.FormatUint(index, 10))
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(included).To(gomega.BeTrue())
		g.Expect(validator.Slashed).To(gomega.BeTrue())
	}).WithContext(ctx).WithTimeout(validatorTimeout).WithPolling(2 * time.Second).Should(gomega.Succeed())
}
