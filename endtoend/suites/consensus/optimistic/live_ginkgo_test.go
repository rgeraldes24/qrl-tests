//go:build e2e

package optimistic_test

import (
	"fmt"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const optimisticTimeout = 30 * time.Minute

var _ = ginkgo.Describe(
	"Optimistic consensus synchronization",
	ginkgo.Serial,
	ginkgo.Label("e2e", "live", "consensus", "optimistic", "mutates-network", "profile-optimistic"),
	func() {
		ginkgo.It("marks unvalidated blocks optimistic and validates them after execution recovery", func(ctx ginkgo.SpecContext) {
			runtime, err := endtoendlive.Load(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			defer runtime.Close()
			if runtime.Profile != devnet.ProfileOptimistic {
				ginkgo.Skip("optimistic-sync coverage requires the optimistic profile")
			}
			sessions, err := runtime.OpenAll(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(sessions).To(gomega.HaveLen(2))
			primary, secondary := sessions[0], sessions[1]
			primaryBeacon := primary.Consensus
			secondaryBeacon := secondary.Consensus
			services := runtime.Services
			participant := secondary.Participant

			gomega.Eventually(func() error {
				status, err := secondaryBeacon.Syncing(ctx)
				if err != nil {
					return err
				}
				if status.Syncing || status.Optimistic || status.ELOffline {
					return fmt.Errorf("secondary is not ready: %+v", status)
				}
				primaryHead, err := primaryBeacon.HeadSlot(ctx)
				if err != nil {
					return err
				}
				if status.HeadSlot+1 < primaryHead {
					return fmt.Errorf("secondary head %d trails primary %d", status.HeadSlot, primaryHead)
				}
				return nil
			}).WithContext(ctx).WithTimeout(optimisticTimeout).WithPolling(time.Second).Should(gomega.Succeed())
			start, err := secondaryBeacon.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			slotsPerEpoch, err := primaryBeacon.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(services.Stop(
				ctx,
				participant.Validator.Name,
				participant.Consensus.Name,
				participant.Execution.Name,
			)).To(gomega.Succeed())
			ginkgo.DeferCleanup(func(cleanupCtx ginkgo.SpecContext) {
				for _, service := range []string{
					participant.Execution.Name,
					participant.Consensus.Name,
					participant.Validator.Name,
				} {
					_ = services.Start(cleanupCtx, service)
				}
			})

			gomega.Eventually(func() uint64 {
				head, _ := primaryBeacon.HeadSlot(ctx)
				return head
			}).WithContext(ctx).WithTimeout(optimisticTimeout).WithPolling(time.Second).Should(
				gomega.BeNumerically(">=", start+2*slotsPerEpoch),
			)
			gomega.Expect(services.Start(ctx, participant.Consensus.Name)).To(gomega.Succeed())
			gomega.Eventually(func() error {
				return runtime.RefreshEnvironment(ctx)
			}).WithContext(ctx).WithTimeout(optimisticTimeout).WithPolling(time.Second).Should(gomega.Succeed())
			secondaryBeacon, err = runtime.ConsensusClient(participant.Index)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			var optimisticHead uint64
			gomega.Eventually(func() error {
				status, err := secondaryBeacon.Syncing(ctx)
				if err != nil {
					return err
				}
				if !status.Optimistic {
					return fmt.Errorf("secondary beacon is not optimistic without execution: %+v", status)
				}
				if !status.ELOffline {
					return fmt.Errorf("secondary beacon does not report execution offline: %+v", status)
				}
				optimisticHead = status.HeadSlot
				return nil
			}).WithContext(ctx).WithTimeout(optimisticTimeout).WithPolling(100 * time.Millisecond).Should(gomega.Succeed())

			gomega.Expect(services.Start(ctx, participant.Execution.Name)).To(gomega.Succeed())

			gomega.Eventually(func() error {
				status, err := secondaryBeacon.Syncing(ctx)
				if err != nil {
					return err
				}
				if status.Syncing || status.Optimistic || status.ELOffline {
					return fmt.Errorf("secondary has not validated its head: %+v", status)
				}
				if status.HeadSlot <= optimisticHead {
					return fmt.Errorf("secondary head %d did not advance beyond optimistic head %d", status.HeadSlot, optimisticHead)
				}
				primaryHead, err := primaryBeacon.HeadSlot(ctx)
				if err != nil {
					return err
				}
				if status.HeadSlot+1 < primaryHead {
					return fmt.Errorf("secondary head %d trails primary %d", status.HeadSlot, primaryHead)
				}
				return nil
			}).WithContext(ctx).WithTimeout(optimisticTimeout).WithPolling(time.Second).Should(gomega.Succeed())
		}, ginkgo.SpecTimeout(optimisticTimeout), ginkgo.Label("behavior:consensus:optimistic-sync"))
	},
)
