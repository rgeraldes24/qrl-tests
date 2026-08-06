// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package externalsigner

import (
	"time"

	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/common"
	qrlwallet "github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const liveSpecTimeout = 5 * time.Minute

type liveSuite struct {
	session *endtoendlive.Session
	wallet  qrlwallet.Wallet
	account common.Address
}

var externalSignerSuite *liveSuite

var _ = ginkgo.Describe(
	"go-qrl configured with Clef",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.ContinueOnFailure,
	ginkgo.Label("e2e", "live", "external-signer", "mutates-chain"),
	func() {
		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			externalSignerSuite = newLiveSuite(ctx)
			gomega.Expect(externalSignerSuite).NotTo(gomega.BeNil())

			var accounts []common.Address
			err := externalSignerSuite.session.Execution.Client().CallContext(ctx, &accounts, "qrl_accounts")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(accounts).To(gomega.ContainElement(externalSignerSuite.account))
		})

		registerAccountDiscoverySpec()
		registerTextSigningSpec()
		registerSigningRejectionSpec()
		registerSignTransactionSpec()
		registerTransactionRejectionSpec()
		registerSubmitTransactionSpec()
		registerContractCreationSpec()
		registerCancellationSpec()
		registerUnavailableSignerSpec()
		registerRestartedSignerSpec()
	},
)
