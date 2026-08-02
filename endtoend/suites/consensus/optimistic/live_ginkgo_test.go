//go:build e2e

package optimistic_test

import (
	"fmt"
	"os"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
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
			if os.Getenv("DEVNET_PROFILE") != string(devnet.ProfileOptimistic) {
				ginkgo.Skip("optimistic-sync coverage requires DEVNET_PROFILE=optimistic")
			}
			sessions, err := endtoendlive.OpenAll(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(sessions).To(gomega.HaveLen(2))
			for _, session := range sessions {
				defer session.Close()
			}
			primary, secondary := sessions[0], sessions[1]
			primaryBeacon, err := consensus.New(primary.Participant.ConsensusURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			secondaryBeacon, err := consensus.New(secondary.Participant.ConsensusURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			services := devnet.NewServiceController(primary.Environment.EnclaveName)
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
				participant.ValidatorServiceName,
				participant.ConsensusServiceName,
				participant.ExecutionServiceName,
			)).To(gomega.Succeed())
			ginkgo.DeferCleanup(func(cleanupCtx ginkgo.SpecContext) {
				for _, service := range []string{
					participant.ExecutionServiceName,
					participant.ConsensusServiceName,
					participant.ValidatorServiceName,
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
			gomega.Expect(services.Start(ctx, participant.ConsensusServiceName)).To(gomega.Succeed())
			var consensusEndpoint string
			gomega.Eventually(func() error {
				var err error
				consensusEndpoint, err = devnet.NewManager().ConsensusEndpoint(
					ctx,
					primary.Environment.EnclaveName,
					participant.ConsensusServiceName,
				)
				return err
			}).WithContext(ctx).WithTimeout(optimisticTimeout).WithPolling(time.Second).Should(gomega.Succeed())
			secondaryBeacon, err = consensus.New(consensusEndpoint)
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

			gomega.Expect(services.Start(ctx, participant.ExecutionServiceName)).To(gomega.Succeed())

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
