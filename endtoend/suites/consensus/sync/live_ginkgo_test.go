//go:build e2e

package sync

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const syncTimeout = 15 * time.Minute

var _ = ginkgo.Describe(
	"Fresh consensus sync and doppelganger protection",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "consensus", "sync", "mutates-network", "profile-sync"),
	func() {
		var primary, secondary *endtoendlive.Session
		var primaryBeacon *consensus.Client
		var services *devnet.ServiceController

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			if os.Getenv("DEVNET_PROFILE") != string(devnet.ProfileSync) {
				ginkgo.Skip("fresh sync and doppelganger coverage requires DEVNET_PROFILE=sync")
			}
			sessions, err := endtoendlive.OpenAll(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(sessions).To(gomega.HaveLen(2))
			for _, session := range sessions {
				ginkgo.DeferCleanup(session.Close)
			}
			primary, secondary = sessions[0], sessions[1]
			primaryBeacon, err = consensus.New(primary.Participant.ConsensusURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			services = devnet.NewServiceController(primary.Environment.EnclaveName)
		})

		ginkgo.It("refuses recently active validator keys after local history is cleared", func(ctx ginkgo.SpecContext) {
			slotsPerEpoch, err := primaryBeacon.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			indices := validatorIndices(32, 64)

			gomega.Eventually(func() uint64 {
				head, _ := primaryBeacon.HeadSlot(ctx)
				return head / slotsPerEpoch
			}).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(time.Second).Should(
				gomega.BeNumerically(">=", 3),
			)
			head, err := primaryBeacon.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			baselineEpoch := head/slotsPerEpoch - 1
			liveness, err := primaryBeacon.Liveness(ctx, baselineEpoch, indices)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(liveness).To(gomega.HaveLen(len(indices)))
			for _, validator := range liveness {
				gomega.Expect(validator.IsLive).To(gomega.BeTrue(), "validator %d was not active before restart", validator.Index)
			}

			metricsURL := secondary.Participant.ConsensusMetricsURL
			checksBefore, err := doppelgangerChecks(ctx, metricsURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			service := secondary.Participant.ValidatorServiceName
			gomega.Expect(services.Stop(ctx, service)).To(gomega.Succeed())
			gomega.Expect(services.Start(ctx, service)).To(gomega.Succeed())

			gomega.Eventually(func() float64 {
				checks, err := doppelgangerChecks(ctx, metricsURL)
				if err != nil {
					return checksBefore
				}
				return checks
			}).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(2 * time.Second).Should(
				gomega.BeNumerically(">", checksBefore),
			)
			restartHead, err := primaryBeacon.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nonSigningEpoch := restartHead/slotsPerEpoch + 1
			gomega.Eventually(func() uint64 {
				head, _ := primaryBeacon.HeadSlot(ctx)
				return head / slotsPerEpoch
			}).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(time.Second).Should(
				gomega.BeNumerically(">", nonSigningEpoch),
			)
			liveness, err = primaryBeacon.Liveness(ctx, nonSigningEpoch, indices)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(liveness).To(gomega.HaveLen(len(indices)))
			for _, validator := range liveness {
				gomega.Expect(validator.IsLive).To(gomega.BeFalse(), "validator %d signed after doppelganger detection", validator.Index)
			}
			_ = services.Stop(ctx, service)
		}, ginkgo.SpecTimeout(syncTimeout), ginkgo.Label(
			"behavior:consensus-startup:doppelganger-rpc",
			"behavior:consensus-startup:doppelganger-no-signing",
		))

		ginkgo.It("syncs a secondary beacon node after clearing its database", func(ctx ginkgo.SpecContext) {
			slotsPerEpoch, err := primaryBeacon.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			start, err := primaryBeacon.HeadSlot(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			participant := secondary.Participant
			gomega.Expect(services.Stop(
				ctx,
				participant.ConsensusServiceName,
				participant.ExecutionServiceName,
			)).To(gomega.Succeed())
			ginkgo.DeferCleanup(func(cleanupCtx ginkgo.SpecContext) {
				_ = services.Start(cleanupCtx, participant.ExecutionServiceName, participant.ConsensusServiceName)
			})

			gomega.Eventually(func() uint64 {
				slot, _ := primaryBeacon.HeadSlot(ctx)
				return slot
			}).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(time.Second).Should(
				gomega.BeNumerically(">=", start+2*slotsPerEpoch),
			)

			gomega.Expect(services.Start(ctx, participant.ExecutionServiceName, participant.ConsensusServiceName)).To(gomega.Succeed())
			var refreshed *endtoendlive.Session
			gomega.Eventually(func() error {
				current, err := endtoendlive.OpenParticipant(ctx, participant.Index, false)
				if err != nil {
					return err
				}
				refreshed = current
				return nil
			}).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(time.Second).Should(gomega.Succeed())
			ginkgo.DeferCleanup(refreshed.Close)

			secondaryBeacon, err := consensus.New(refreshed.Participant.ConsensusURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Eventually(func() error {
				progress, err := refreshed.Execution.SyncProgress(ctx)
				if err != nil {
					return err
				}
				if progress != nil {
					return fmt.Errorf("execution client is still syncing")
				}
				status, err := secondaryBeacon.Syncing(ctx)
				if err != nil {
					return err
				}
				if status.Syncing || status.Optimistic || status.ELOffline {
					return fmt.Errorf("consensus client is not ready: %+v", status)
				}
				primarySlot, err := primaryBeacon.HeadSlot(ctx)
				if err != nil {
					return err
				}
				if status.HeadSlot+1 < primarySlot {
					return fmt.Errorf("secondary head %d trails primary %d", status.HeadSlot, primarySlot)
				}
				return nil
			}).WithContext(ctx).WithTimeout(syncTimeout).WithPolling(time.Second).Should(gomega.Succeed())
		}, ginkgo.SpecTimeout(syncTimeout), ginkgo.Label("behavior:consensus-sync:fresh-database"))
	},
)

func validatorIndices(first, end uint64) []uint64 {
	indices := make([]uint64, end-first)
	for index := range indices {
		indices[index] = first + uint64(index)
	}
	return indices
}

func doppelgangerChecks(ctx context.Context, metricsURL string) (float64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(metricsURL, "/")+"/metrics", nil)
	if err != nil {
		return 0, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("metrics endpoint returned %s", response.Status)
	}
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, err
	}
	var total float64
	for _, line := range strings.Split(string(payload), "\n") {
		if !strings.HasPrefix(line, "grpc_server_handled_total{") ||
			!strings.Contains(line, `grpc_method="CheckDoppelGanger"`) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return 0, fmt.Errorf("invalid doppelganger metric line %q", line)
		}
		value, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return 0, fmt.Errorf("parse doppelganger metric: %w", err)
		}
		total += value
	}
	return total, nil
}
