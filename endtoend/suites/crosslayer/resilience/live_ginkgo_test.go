//go:build e2e

package resilience

import (
	"context"
	"fmt"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const recoveryTimeout = 10 * time.Minute

type liveSuite struct {
	primary, secondary *endtoendlive.Session
	primaryBeacon      *consensus.Client
	services           *devnet.ServiceController
	stopped            map[string]bool
}

var _ = ginkgo.Describe(
	"native client outage and recovery",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "resilience", "multi-node", "mutates-network", "scenario"),
	func() {
		var suite *liveSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			runtime, err := endtoendlive.Load(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			sessions, err := runtime.OpenAll(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			if len(sessions) < 2 {
				ginkgo.Skip("resilience scenarios require the sync or multi profile")
			}
			suite = &liveSuite{
				primary: sessions[0], secondary: sessions[1],
				primaryBeacon: sessions[0].Consensus,
				services:      runtime.Services,
				stopped:       make(map[string]bool),
			}
		})

		ginkgo.AfterEach(func() {
			if suite == nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			for service := range suite.stopped {
				gomega.Expect(suite.start(ctx, service)).To(gomega.Succeed())
			}
		})

		ginkgo.It("recovers a stopped execution client and Engine connection", func(ctx ginkgo.SpecContext) {
			service := suite.secondary.Participant.Execution.Name
			gomega.Expect(suite.stop(ctx, service)).To(gomega.Succeed())

			gomega.Eventually(func() bool {
				_, err := suite.secondary.Execution.BlockNumber(ctx)
				return err != nil
			}).WithContext(ctx).WithTimeout(2 * time.Minute).WithPolling(time.Second).Should(gomega.BeTrue())

			gomega.Expect(suite.start(ctx, service)).To(gomega.Succeed())
			suite.awaitSecondaryReady(ctx)
		}, ginkgo.SpecTimeout(recoveryTimeout))

		ginkgo.It("recovers a stopped consensus client", func(ctx ginkgo.SpecContext) {
			service := suite.secondary.Participant.Consensus.Name
			start, err := suite.primaryBeacon.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(suite.stop(ctx, service)).To(gomega.Succeed())
			gomega.Eventually(func() uint64 {
				head, _ := suite.primaryBeacon.HeadSlot(ctx)
				return head
			}).WithContext(ctx).WithTimeout(2 * time.Minute).Should(gomega.BeNumerically(">", start))

			gomega.Expect(suite.start(ctx, service)).To(gomega.Succeed())
			suite.awaitSecondaryReady(ctx)
		}, ginkgo.SpecTimeout(recoveryTimeout))

		ginkgo.It("catches up an execution-consensus pair after an extended outage", func(ctx ginkgo.SpecContext) {
			participant := suite.secondary.Participant
			services := []string{
				participant.Validator.Name,
				participant.Consensus.Name,
				participant.Execution.Name,
			}
			start, err := suite.primaryBeacon.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			slotsPerEpoch, err := suite.primaryBeacon.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(suite.stop(ctx, services...)).To(gomega.Succeed())
			gomega.Eventually(func() uint64 {
				head, _ := suite.primaryBeacon.HeadSlot(ctx)
				return head
			}).WithContext(ctx).WithTimeout(4 * time.Minute).Should(
				gomega.BeNumerically(">=", start+2*slotsPerEpoch),
			)

			gomega.Expect(suite.start(ctx, services...)).To(gomega.Succeed())
			suite.awaitSecondaryReady(ctx)
		}, ginkgo.SpecTimeout(recoveryTimeout))
	},
)

func (suite *liveSuite) awaitSecondaryReady(ctx context.Context) {
	ginkgo.GinkgoHelper()
	gomega.Eventually(func() error {
		if err := suite.primary.Runtime.RefreshEnvironment(ctx); err != nil {
			return err
		}
		secondary, err := suite.primary.Runtime.OpenParticipant(ctx, suite.secondary.Participant.Index, false)
		if err != nil {
			return err
		}
		defer secondary.Close()
		secondaryBeacon := secondary.Consensus
		progress, err := secondary.Execution.SyncProgress(ctx)
		if err != nil {
			return err
		}
		if progress != nil {
			return fmt.Errorf("execution client still syncing: %+v", progress)
		}
		status, err := secondaryBeacon.Syncing(ctx)
		if err != nil {
			return err
		}
		if status.Syncing || status.Optimistic || status.ELOffline {
			return fmt.Errorf("consensus client not ready: %+v", status)
		}
		primary, err := suite.primaryBeacon.HeadSlot(ctx)
		if err != nil {
			return err
		}
		if status.HeadSlot+1 < primary {
			return fmt.Errorf("secondary head %d trails primary %d", status.HeadSlot, primary)
		}
		return nil
	}).WithContext(ctx).WithTimeout(recoveryTimeout).WithPolling(time.Second).Should(gomega.Succeed())
}

func (suite *liveSuite) stop(ctx context.Context, services ...string) error {
	if err := suite.services.Stop(ctx, services...); err != nil {
		return err
	}
	for _, service := range services {
		suite.stopped[service] = true
	}
	return nil
}

func (suite *liveSuite) start(ctx context.Context, services ...string) error {
	if err := suite.services.Start(ctx, services...); err != nil {
		return err
	}
	for _, service := range services {
		delete(suite.stopped, service)
	}
	return nil
}
