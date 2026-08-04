//go:build e2e

package coldstate_test

import (
	"slices"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/behavior"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const coldStateTimeout = 30 * time.Minute

var _ = ginkgo.Describe(
	"Cold consensus state retrieval",
	ginkgo.Serial,
	ginkgo.Label("e2e", "live", "consensus", "cold-state", "profile-cold"),
	func() {
		ginkgo.It("retrieves complete genesis-era assignments after archival", func(ctx ginkgo.SpecContext) {
			runtime, err := endtoendlive.Load(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			defer runtime.Close()
			if runtime.Profile != devnet.ProfileCold {
				ginkgo.Skip("cold-state coverage requires the cold profile")
			}
			session, err := runtime.Primary(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			beacon := session.Consensus
			slotsPerEpoch, err := beacon.SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			active, err := beacon.ActiveValidatorIndices(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(func() uint64 {
				head, _ := beacon.HeadSlot(ctx)
				return head
			}).WithContext(ctx).WithTimeout(coldStateTimeout).WithPolling(time.Second).Should(
				gomega.BeNumerically(">=", 8*slotsPerEpoch),
			)

			for epoch := uint64(0); epoch < 2; epoch++ {
				assignments, err := beacon.ValidatorAssignments(ctx, epoch)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(assignments).To(gomega.HaveLen(len(active)))
				seen := make(map[uint64]struct{}, len(assignments))
				for _, assignment := range assignments {
					gomega.Expect(assignment.AttesterSlot).To(gomega.And(
						gomega.BeNumerically(">=", epoch*slotsPerEpoch),
						gomega.BeNumerically("<", (epoch+1)*slotsPerEpoch),
					))
					gomega.Expect(slices.Contains(assignment.BeaconCommittee, assignment.ValidatorIndex)).To(gomega.BeTrue())
					for _, slot := range assignment.ProposerSlots {
						gomega.Expect(slot).To(gomega.And(
							gomega.BeNumerically(">=", epoch*slotsPerEpoch),
							gomega.BeNumerically("<", (epoch+1)*slotsPerEpoch),
						))
					}
					_, duplicate := seen[assignment.ValidatorIndex]
					gomega.Expect(duplicate).To(gomega.BeFalse())
					seen[assignment.ValidatorIndex] = struct{}{}
				}
			}
		}, ginkgo.SpecTimeout(coldStateTimeout), ginkgo.Label(behavior.Name("consensus:cold-state-assignments")))
	},
)
