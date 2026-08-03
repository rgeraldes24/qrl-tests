// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

//go:build e2e

package externalsigner

import (
	"context"
	"fmt"
	"time"

	qrlapi "github.com/cyyber/qrl-tests/endtoend/internal/rpctypes"
	"github.com/theQRL/go-qrl/accounts"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/crypto/pqcrypto"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func registerUnavailableSignerSpec() {
	ginkgo.It("fails while Clef is unavailable and recovers after restart", func(ctx ginkgo.SpecContext) {
		gomega.Expect(externalSignerSuite.session.Services.Stop(ctx, "signer-clef")).To(gomega.Succeed())
		stopped := true
		defer func() {
			if stopped {
				_ = externalSignerSuite.session.Services.Start(context.Background(), "signer-clef")
			}
		}()

		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		var signature hexutil.Bytes
		err := externalSignerSuite.session.Execution.Client().CallContext(
			requestCtx, &signature, "qrl_sign", externalSignerSuite.account, hexutil.Bytes("Clef unavailable"),
		)
		gomega.Expect(err).To(gomega.HaveOccurred())

		gomega.Expect(externalSignerSuite.session.Services.Start(ctx, "signer-clef")).To(gomega.Succeed())
		stopped = false
		gomega.Eventually(func() error {
			var managed []common.Address
			if err := externalSignerSuite.session.Execution.Client().CallContext(ctx, &managed, "qrl_accounts"); err != nil {
				return err
			}
			if len(managed) != 1 || managed[0] != externalSignerSuite.account {
				return fmt.Errorf("unexpected managed accounts after restart: %v", managed)
			}
			return nil
		}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}

func registerRestartedSignerSpec() {
	ginkgo.It("reconnects to Clef after the signer restarts", func(ctx ginkgo.SpecContext) {
		gomega.Expect(externalSignerSuite.session.Services.Restart(ctx, "signer-clef")).To(gomega.Succeed())

		message := []byte("go-qrl external signer restart E2E")
		gomega.Eventually(func() error {
			var managed []common.Address
			if err := externalSignerSuite.session.Execution.Client().CallContext(ctx, &managed, "qrl_accounts"); err != nil {
				return err
			}
			if len(managed) != 1 || managed[0] != externalSignerSuite.account {
				return fmt.Errorf("unexpected managed accounts after restart: %v", managed)
			}

			var signature hexutil.Bytes
			if err := externalSignerSuite.session.Execution.Client().CallContext(
				ctx, &signature, "qrl_sign", externalSignerSuite.account, hexutil.Bytes(message),
			); err != nil {
				return err
			}
			valid, err := pqcrypto.MLDSA87VerifySignature(
				signature,
				accounts.TextHash(message),
				externalSignerSuite.wallet.GetPK(),
				externalSignerSuite.wallet.GetDescriptor(),
			)
			if err != nil {
				return err
			}
			if !valid {
				return fmt.Errorf("invalid signature after Clef restart")
			}
			return nil
		}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(time.Second).Should(gomega.Succeed())

		args := externalSignerSuite.transactionArgs(ctx)
		var signed qrlapi.SignTransactionResult
		gomega.Eventually(func() error {
			return externalSignerSuite.session.Execution.Client().CallContext(
				ctx, &signed, "qrl_signTransaction", args,
			)
		}).WithContext(ctx).WithTimeout(time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
		gomega.Expect(signed.Tx).NotTo(gomega.BeNil())
		gomega.Expect(transactionSender(signed.Tx, externalSignerSuite.session.ChainID)).To(
			gomega.Equal(externalSignerSuite.account),
		)
		expectTransactionMatchesArgs(signed.Tx, args)
	}, ginkgo.SpecTimeout(liveSpecTimeout))
}
