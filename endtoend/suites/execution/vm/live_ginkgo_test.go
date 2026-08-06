// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package vm

import (
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/behavior"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/cyyber/qrl-tests/endtoend/internal/testsuite"
	"github.com/theQRL/go-qrl/common"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const liveSpecTimeout = 5 * time.Minute

type liveSuite struct {
	session *endtoendlive.Session
	target  common.Address
}

var vmSuite *liveSuite

var _ = ginkgo.Describe(
	"hand-written QRVM bytecode",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label(
		"e2e", "live", "vm", "mutates-chain", "scenario",
		"scenario:stable:all-opcodes-test", behavior.Name("vm:vm64-opcodes"),
	),
	func() {
		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			runtime := testsuite.LoadRuntime()
			var err error
			session, err := runtime.Primary(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			vmSuite = &liveSuite{session: session, target: patternedAddress(0xf0)}
		})

		registerWordOperationSpecs()
		registerMemoryStorageSpecs()
		registerJumpAndMinedSpecs()
		registerCopyHashSpecs()
		registerCallCreationSpecs()
		registerContextLogSpecs()
	},
)
