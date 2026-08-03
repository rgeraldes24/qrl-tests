//go:build e2e

package protocol_test

import (
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	protocolTimeout      = 30 * time.Minute
	protocolPollInterval = 2 * time.Second
	maxMetricsMemory     = 2_000_000_000
	expectedFeeRecipient = "Q0838a121a6e4dd8a51e7437b152fabbc76a173f077132f2c2ed021c7b0991e70da4dba44e9ec00984a90f28dfb0aabbda1ddc9e98a76ab0acb6644c5e76fbbe8"
)

type protocolSuite struct {
	sessions      []*endtoendlive.Session
	beacons       []*consensus.Client
	slotsPerEpoch uint64
}

var _ = ginkgo.Describe(
	"Consensus protocol invariants",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.Label("e2e", "live", "consensus", "protocol"),
	func() {
		var suite protocolSuite

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			var err error
			runtime, loadErr := endtoendlive.Load(ctx)
			gomega.Expect(loadErr).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			suite.sessions, err = runtime.OpenAll(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			for _, session := range suite.sessions {
				suite.beacons = append(suite.beacons, session.Consensus)
			}
			suite.slotsPerEpoch, err = suite.beacons[0].SpecUint(ctx, "SLOTS_PER_EPOCH")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Eventually(func(g gomega.Gomega) {
				head, err := suite.beacons[0].HeadSlot(ctx)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				g.Expect(head / suite.slotsPerEpoch).To(gomega.BeNumerically(">=", 3))
			}).WithContext(ctx).WithTimeout(protocolTimeout).WithPolling(protocolPollInterval).Should(gomega.Succeed())
		})

		registerProtocolInvariants(&suite)
		registerProtocolSignatureChecks(&suite)
	},
)
