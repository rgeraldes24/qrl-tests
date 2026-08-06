//go:build e2e

package network

import (
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/beacon"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const (
	pollInterval    = time.Second
	progressTimeout = 10 * time.Minute
	minimumTarget   = 98.0
	minimumHead     = 80.0
)

type node struct {
	session   *endtoendlive.Session
	consensus *beacon.Client
}

type liveSuite struct {
	nodes []node
}

var networkSuite *liveSuite

var _ = ginkgo.Describe(
	"QRL network health",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "network", "scenario"),
	func() {
		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			runtime := testsuite.LoadRuntime()
			var err error
			sessions, err := runtime.OpenAll(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			networkSuite = new(liveSuite)
			for _, session := range sessions {
				networkSuite.nodes = append(networkSuite.nodes, node{
					session: session, consensus: session.Consensus,
				})
			}
		})

		registerReadinessSpecs()
		registerFinalizationSpec()
		registerParticipationSpecs()
		registerHeadConvergenceSpec()
	},
)
