//go:build e2e

package soak_test

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/execfixture"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/cyyber/qrl-tests/endtoend/internal/stability"
	"github.com/theQRL/go-qrl/core/types"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	defaultSoakDuration = 30 * time.Minute
	soakTimeout         = 4 * time.Hour
)

var _ = ginkgo.Describe(
	"repeated workload and disruption recovery",
	ginkgo.Serial,
	ginkgo.Label("e2e", "live", "system", "soak", "scenario-full", "mutates-network", "mutates-chain"),
	func() {
		ginkgo.It("keeps finalizing through repeated restarts and partitions", func(ctx ginkgo.SpecContext) {
			runtime, err := endtoendlive.Load(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			defer runtime.Close()
			sessions, err := runtime.OpenAll(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			if len(sessions) < 4 {
				ginkgo.Skip("the soak lane requires the four-participant chaos profile")
			}
			beacon := sessions[0].Consensus
			contract, err := execfixture.DeployStateContract(ctx, sessions[0], execfixture.FullTopic(0xd0))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			services := runtime.Services
			partition, err := devnet.NewNetworkPartition(sessions[0].Environment.Backend)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(func() {
				cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				gomega.Expect(partition.Clear(cleanup)).To(gomega.Succeed())
			})

			duration := defaultSoakDuration
			if configured := os.Getenv("E2E_SOAK_DURATION"); configured != "" {
				duration, err = time.ParseDuration(configured)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(duration).To(gomega.BeNumerically(">", 0))
			}
			deadline := time.Now().Add(duration)
			for cycle := 0; cycle == 0 || time.Now().Before(deadline); cycle++ {
				startFinalized, err := beacon.FinalizedEpoch(ctx)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				for transactionIndex := 0; transactionIndex < len(sessions); transactionIndex++ {
					nonce, err := sessions[0].Execution.PendingNonceAt(ctx, sessions[0].Address)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					word := execfixture.FullWord(byte(cycle*len(sessions) + transactionIndex + 1))
					tx, err := execfixture.SignCall(ctx, sessions[0], nonce, contract.Address, big.NewInt(int64(cycle+1)), word[:])
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(sessions[0].Execution.SendTransaction(ctx, tx)).To(gomega.Succeed())
					awaitReceipt(ctx, sessions[0], tx)
				}

				if cycle%2 == 0 {
					participant := sessions[1].Participant
					gomega.Expect(services.Restart(ctx,
						participant.Execution.Name,
						participant.Consensus.Name,
						participant.Validator.Name,
					)).To(gomega.Succeed())
					var replacement *endtoendlive.Session
					gomega.Eventually(func() error {
						var err error
						if err := runtime.RefreshEnvironment(ctx); err != nil {
							return err
						}
						replacement, err = runtime.OpenParticipant(ctx, participant.Index)
						return err
					}).WithContext(ctx).WithTimeout(5 * time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
					sessions[1] = replacement
				} else {
					participants := sessions[0].Environment.Participants
					middle := len(participants) / 2
					gomega.Expect(partition.Apply(ctx, participants[:middle], participants[middle:])).To(gomega.Succeed())
					startSlot, err := beacon.HeadSlot(ctx)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Eventually(func() uint64 {
						slot, _ := beacon.HeadSlot(ctx)
						return slot
					}).WithContext(ctx).WithTimeout(3 * time.Minute).WithPolling(time.Second).Should(gomega.BeNumerically(">", startSlot))
					gomega.Expect(partition.Clear(ctx)).To(gomega.Succeed())
				}

				stabilityCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
				err = stability.Await(stabilityCtx, sessions, startFinalized, 1)
				cancel()
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("cycle %d", cycle))
			}
		}, ginkgo.SpecTimeout(soakTimeout), ginkgo.Label("behavior:system:soak-recovery"))
	},
)

func awaitReceipt(ctx context.Context, session *endtoendlive.Session, tx *types.Transaction) {
	ginkgo.GinkgoHelper()
	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	_, err := execfixture.WaitReceipt(waitCtx, session.Execution, tx.Hash())
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}
