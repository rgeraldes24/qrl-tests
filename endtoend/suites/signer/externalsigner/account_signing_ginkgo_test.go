// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package externalsigner

import (
	"github.com/theQRL/go-qrl/accounts"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/crypto/pqcrypto"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerAccountDiscoverySpec() {
	ginkgo.It("discovers the node-managed Clef account", func(ctx ginkgo.SpecContext) {
		var managed []common.Address
		gomega.Expect(externalSignerSuite.session.Execution.Client().CallContext(
			ctx, &managed, "qrl_accounts",
		)).To(gomega.Succeed())
		gomega.Expect(managed).To(gomega.Equal([]common.Address{externalSignerSuite.account}))
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}

func registerTextSigningSpec() {
	ginkgo.It("signs text through the node", func(ctx ginkgo.SpecContext) {
		message := []byte("go-qrl external signer E2E")
		var signature hexutil.Bytes
		gomega.Expect(externalSignerSuite.session.Execution.Client().CallContext(
			ctx, &signature, "qrl_sign", externalSignerSuite.account, hexutil.Bytes(message),
		)).To(gomega.Succeed())

		valid, err := pqcrypto.MLDSA87VerifySignature(
			signature,
			accounts.TextHash(message),
			externalSignerSuite.wallet.GetPK(),
			externalSignerSuite.wallet.GetDescriptor(),
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(valid).To(gomega.BeTrue())
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}
