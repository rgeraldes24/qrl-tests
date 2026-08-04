//go:build e2e

package api

import (
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const beaconAPITimeout = 10 * time.Minute

type beaconNode struct {
	session *endtoendlive.Session
	client  *consensus.Client
}

var _ = ginkgo.Describe(
	"Beacon, node, and configuration APIs",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "consensus", "beacon-api"),
	func() {
		var nodes []beaconNode

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			runtime, err := endtoendlive.Load(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			ginkgo.DeferCleanup(runtime.Close)
			sessions, err := runtime.OpenAll(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			for _, session := range sessions {
				nodes = append(nodes, beaconNode{session: session, client: session.Consensus})
			}
		})

		registerBeaconMetadata(&nodes)
		registerBeaconRuntime(&nodes)
		registerBeaconRewards(&nodes)
	},
)
