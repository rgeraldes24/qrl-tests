//go:build e2e

package validator_test

import (
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/chaincontext"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/client"
	"github.com/cyyber/qrl-tests/endtoend/internal/consensus/validator"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

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
	chain             consensuscontext.Context
	depositor         *validatorops.Depositor
	expectedProposers map[string]struct{}
	services          *devnet.ServiceController
}

var validatorOperations operationsSuite

var _ = ginkgo.Describe(
	"Validator operation workloads",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.Label(
		"e2e", "live", "validator", "mutates-chain", "scenario", "scenario-full", "profile-operations",
	),
	func() {
		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			runtime, loadErr := endtoendlive.Load(ctx)
			gomega.Expect(loadErr).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			validatorOperations.sessions, err = runtime.OpenAll(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(validatorOperations.sessions).To(gomega.HaveLen(5))
			for _, session := range validatorOperations.sessions {
				validatorOperations.beacons = append(validatorOperations.beacons, session.Consensus)
			}
			validatorOperations.primary = validatorOperations.sessions[0]
			validatorOperations.beacon = validatorOperations.beacons[0]
			validatorOperations.chain, err = consensuscontext.Load(ctx, validatorOperations.beacon)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			validatorOperations.depositor, err = validatorops.NewDepositor(
				ctx, validatorOperations.primary, validatorOperations.beacon, validatorOperations.chain,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			active, err := validatorOperations.beacon.ActiveValidatorCount(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(active).To(gomega.BeNumerically(">=", minimumOperationsStake))
			validatorOperations.expectedProposers = expectedValidatorPairs(
				validatorOperations.sessions[:genesisParticipantCount],
			)
			validatorOperations.services = runtime.Services
		})

		registerLifecycleSpec()
		registerExitSpec()
		registerSlashingSpec()
		registerRecoverySpec()
	},
)
