//go:build e2e

package penalties

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/e2e/internal/beacon"
	"github.com/cyyber/qrl-tests/e2e/internal/chaininfo"
	"github.com/cyyber/qrl-tests/e2e/internal/live"
	"github.com/cyyber/qrl-tests/e2e/internal/sidecar"
	"github.com/cyyber/qrl-tests/e2e/internal/sidecar/deposit"
	"github.com/cyyber/qrl-tests/e2e/internal/sidecar/validator"
	"github.com/cyyber/qrl-tests/e2e/internal/testsuite"
	"github.com/cyyber/qrl-tests/e2e/internal/validatorops"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	// offlineEpochs is how many whole epochs the validator client stays
	// stopped while the suite checks each one's penalty.
	offlineEpochs = 3

	// attestTimeout bounds the wait for a started validator client's first
	// attested epoch: a missed first epoch and the two epochs before an
	// epoch's rewards are served. The specs add a minute or more for the
	// client's start and the checks that follow.
	attestTimeout = 5 * time.Minute
	// offlineTimeout covers the rest of the epoch the client stops in, the
	// offline epochs, the two epochs before the last one's rewards are
	// served, and reading every block of the offline epochs.
	offlineTimeout = 6 * time.Minute
)

func TestE2E(t *testing.T) {
	testsuite.Run(t, "Penalties E2E suite")
}

var _ = ginkgo.Describe(
	"A validator whose client stops and starts again",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.Label("e2e", "consensus", "penalties"),
	func() {
		var (
			node            *live.Node
			spare           spareValidator
			keystores       []sidecar.File
			validatorClient *validator.Sidecar
		)

		startValidatorClient := func(ctx ginkgo.SpecContext) {
			ginkgo.GinkgoHelper()
			validatorClient = testsuite.MustSucceed(validator.Start(ctx, validator.Config{
				Image:              node.ValidatorImage,
				BeaconURL:          node.BeaconURL,
				BeaconGRPC:         node.BeaconGRPC,
				ConsensusServiceID: node.ConsensusServiceID,
				Keystores:          keystores,
			}))
		}

		// Removing the container stops the client for good, so a client
		// started later, with a new slashing-protection database, signs only
		// for later epochs and never conflicts with what this one signed.
		stopValidatorClient := func() {
			ginkgo.GinkgoHelper()
			client := validatorClient
			validatorClient = nil
			// Printed only if the spec fails, such as when an epoch the
			// client should have missed shows its vote. Removing the
			// container loses the log, so the AfterEach has none to report.
			if logs, err := client.Logs(); err != nil {
				ginkgo.GinkgoWriter.Printf("stopped validator client logs unavailable: %v\n", err)
			} else {
				ginkgo.GinkgoWriter.Printf("stopped validator client, last log lines:\n%s\n", logs)
			}
			gomega.Expect(client.Close()).To(gomega.Succeed())
		}

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			runtime := testsuite.LoadRuntime()
			node = testsuite.MustSucceed(runtime.PrimaryNode(ctx))
			gomega.Expect(node.ValidatorImage).NotTo(gomega.BeEmpty(), "validator image is not configured")
			gomega.Expect(node.QrysmAlltoolsImage).NotTo(gomega.BeEmpty(), "Qrysm all-tools image is not configured")
			gomega.Expect(node.BeaconGRPC).NotTo(gomega.BeEmpty(), "beacon gRPC is not published")

			spares := runtime.Profile.SpareValidators()
			gomega.Expect(spares).NotTo(gomega.BeEmpty(), "profile %q has no genesis validator that no client runs", runtime.Profile)
			spare = spareValidator{
				client: node.Beacon,
				chain:  testsuite.MustSucceed(chaininfo.Load(ctx, node.Beacon)),
				spec:   testsuite.MustSucceed(loadRewardSpec(ctx, node.Beacon)),
				index:  spares[0],
			}

			derived := testsuite.MustSucceed(deposit.GenesisKey(ctx, deposit.GenesisKeyConfig{
				Image:               node.QrysmAlltoolsImage,
				Mnemonic:            devnet.GenesisMnemonic,
				Index:               spare.index,
				WithdrawalRecipient: node.Address.Hex(),
				Password:            validatorops.KeystorePassword,
			}))
			keystores = derived.Keystores

			// A network from other parameters may have no such validator, or
			// a validator client that already runs it.
			record := testsuite.MustSucceed(node.Beacon.Validator(ctx, strconv.FormatUint(spare.index, 10)))
			gomega.Expect(strings.EqualFold(record.PublicKey, derived.PublicKey)).To(gomega.BeTrue(),
				"genesis validator %d is not the key derived for its index", spare.index)
			gomega.Expect(record.Status).To(gomega.Equal("active_ongoing"))
		}, ginkgo.NodeTimeout(3*time.Minute))

		ginkgo.AfterEach(func() {
			testsuite.ReportLogsOnFailure("validator client", validatorClient)
		})

		ginkgo.AfterAll(func() {
			gomega.Expect(validatorClient.Close()).To(gomega.Succeed())
		})

		ginkgo.It("attests once its validator client starts", func(ctx ginkgo.SpecContext) {
			// No client has run the validator since genesis, so every epoch
			// before the start is a miss.
			head := testsuite.MustSucceed(node.Beacon.HeadSlot(ctx))
			startValidatorClient(ctx)
			spare.firstAttestedEpoch(ctx, spare.chain.Epoch(head), attestTimeout)
		}, ginkgo.SpecTimeout(attestTimeout+time.Minute))

		ginkgo.It("loses exactly the missed-vote penalty in every epoch its validator client is stopped", func(ctx ginkgo.SpecContext) {
			stopValidatorClient()
			// The removed client may have voted in the epoch it stopped in.
			// The host clock covers a head that lags; a head already in a
			// later epoch covers a host clock behind the node's, as Docker
			// Desktop's can be after sleep.
			head := testsuite.MustSucceed(node.Beacon.HeadSlot(ctx))
			first := max(spare.chain.EpochAt(time.Now()), spare.chain.Epoch(head)) + 1
			last := first + offlineEpochs - 1
			spare.awaitRewards(ctx, last)

			for epoch := first; epoch <= last; epoch++ {
				reward := testsuite.MustSucceed(spare.reward(ctx, epoch))
				amounts := testsuite.MustSucceed(spare.voteAmounts(ctx, epoch))
				gomega.Expect(reward).To(gomega.Equal(beacon.AttestationReward{
					ValidatorIndex: spare.index,
					Source:         -int64(amounts.missedSource),
					Target:         -int64(amounts.missedTarget),
				}), "attestation rewards of epoch %d", epoch)

				// The rewards of epoch apply when epoch+2 starts. The blocks
				// of epoch+1 add the missed sync committee duties and take
				// any withdrawal; the offline validator proposes none.
				before := testsuite.MustSucceed(spare.balanceAtEpochStart(ctx, epoch+1))
				after := testsuite.MustSucceed(spare.balanceAtEpochStart(ctx, epoch+2))
				blocks := testsuite.MustSucceed(spare.blockBalanceChanges(ctx, epoch+1))
				penalty := amounts.missedSource + amounts.missedTarget
				want := int64(before) - int64(penalty) + blocks
				gomega.Expect(int64(after)).To(gomega.Equal(want),
					"balance went from %d to %d over epoch %d: the %d shor penalty of epoch %d and %d shor from blocks make %d",
					before, after, epoch+1, penalty, epoch, blocks, want)
			}
		}, ginkgo.SpecTimeout(offlineTimeout))

		ginkgo.It("earns exactly the timely-vote rewards once its validator client starts again", func(ctx ginkgo.SpecContext) {
			head := testsuite.MustSucceed(node.Beacon.HeadSlot(ctx))
			startValidatorClient(ctx)
			resumed := spare.firstAttestedEpoch(ctx, spare.chain.Epoch(head), attestTimeout)

			// The target vote has the whole epoch, so a healthy client's is
			// timely. A timely source vote needs inclusion within two slots
			// and a timely head vote within one, which empty slots can take
			// away, so each of those earns its reward or costs its penalty.
			next := resumed + 1
			spare.awaitRewards(ctx, next)
			reward := testsuite.MustSucceed(spare.reward(ctx, next))
			amounts := testsuite.MustSucceed(spare.voteAmounts(ctx, next))
			gomega.Expect(reward.Target).To(gomega.Equal(amounts.timely.Target), "target reward of epoch %d", next)
			gomega.Expect(reward.Source).To(gomega.BeElementOf(amounts.timely.Source, -int64(amounts.missedSource)),
				"source reward of epoch %d", next)
			gomega.Expect(reward.Head).To(gomega.BeElementOf(amounts.timely.Head, int64(0)), "head reward of epoch %d", next)

			// The rewards of next apply when next+2 starts. The blocks of
			// next+1 add the validator's proposer and sync committee rewards
			// and take any withdrawal.
			before := testsuite.MustSucceed(spare.balanceAtEpochStart(ctx, next+1))
			after := testsuite.MustSucceed(spare.balanceAtEpochStart(ctx, next+2))
			blocks := testsuite.MustSucceed(spare.blockBalanceChanges(ctx, next+1))
			earned := reward.Head + reward.Source + reward.Target
			want := int64(before) + earned + blocks
			gomega.Expect(int64(after)).To(gomega.Equal(want),
				"balance went from %d to %d over epoch %d: the %d shor reward of epoch %d and %d shor from blocks make %d",
				before, after, next+1, earned, next, blocks, want)
		}, ginkgo.SpecTimeout(attestTimeout+2*time.Minute))
	},
)
