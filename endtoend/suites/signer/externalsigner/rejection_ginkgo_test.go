// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package externalsigner

import (
	"context"
	"math/big"
	"time"

	qrlapi "github.com/cyyber/qrl-tests/endtoend/internal/rpctypes"
	"github.com/cyyber/qrl-tests/internal/fixture"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerSigningRejectionSpec() {
	ginkgo.It("propagates Clef signing rejection through the node", func(ctx ginkgo.SpecContext) {
		var signature hexutil.Bytes
		err := externalSignerSuite.session.Execution.Client().CallContext(
			ctx,
			&signature,
			"qrl_sign",
			externalSignerSuite.account,
			hexutil.Bytes(fixture.RemoteSignerRejectedText),
		)
		gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("request denied")))
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}

func registerTransactionRejectionSpec() {
	ginkgo.It("propagates Clef transaction rejection through the node", func(ctx ginkgo.SpecContext) {
		before, err := externalSignerSuite.session.Execution.PendingNonceAt(ctx, externalSignerSuite.account)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		args := externalSignerSuite.transactionArgs(ctx)
		args.Value = (*hexutil.Big)(big.NewInt(fixture.RemoteSignerRejectedTransaction))

		var signed qrlapi.SignTransactionResult
		err = externalSignerSuite.session.Execution.Client().CallContext(ctx, &signed, "qrl_signTransaction", args)
		gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("request denied")))

		var hash common.Hash
		err = externalSignerSuite.session.Execution.Client().CallContext(ctx, &hash, "qrl_sendTransaction", args)
		gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("request denied")))

		expectNoTransactionAtNonce(ctx, externalSignerSuite, before)
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}

func registerCancellationSpec() {
	ginkgo.It("does not submit a transaction after signing is canceled", func(ctx ginkgo.SpecContext) {
		before, err := externalSignerSuite.session.Execution.PendingNonceAt(ctx, externalSignerSuite.account)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		args := externalSignerSuite.transactionArgs(ctx)
		args.Value = (*hexutil.Big)(big.NewInt(fixture.RemoteSignerDelayedTransaction))
		requestCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		var hash common.Hash
		err = externalSignerSuite.session.Execution.Client().CallContext(
			requestCtx, &hash, "qrl_sendTransaction", args,
		)
		gomega.Expect(err).To(gomega.MatchError(context.DeadlineExceeded))

		expectNoTransactionAtNonce(ctx, externalSignerSuite, before)
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}
