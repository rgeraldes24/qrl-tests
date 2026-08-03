//go:build e2e

package transactions

import (
	"context"
	"math/big"

	"github.com/cyyber/qrl-tests/endtoend/internal/execfixture"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	"github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func signTransaction(ctx context.Context, session *endtoendlive.Session, to common.Address, value *big.Int, data []byte) *types.Transaction {
	ginkgo.GinkgoHelper()

	nonce, err := session.Execution.PendingNonceAt(ctx, session.Address)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	parameters := loadTransactionParameters(ctx, session, to, value, data)
	return signTransactionAt(session, nonce, to, value, data, parameters)
}

func loadTransactionParameters(ctx context.Context, session *endtoendlive.Session, to common.Address, value *big.Int, data []byte) execfixture.TransactionParameters {
	ginkgo.GinkgoHelper()

	parameters, err := execfixture.EstimateCall(ctx, session, to, value, data)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return parameters
}

func signTransactionAt(session *endtoendlive.Session, nonce uint64, to common.Address, value *big.Int, data []byte, parameters execfixture.TransactionParameters) *types.Transaction {
	ginkgo.GinkgoHelper()

	tx, err := execfixture.SignCallWithParameters(session, nonce, to, value, data, parameters)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	signer := types.LatestSignerForChainID(session.ChainID)
	sender, err := types.Sender(signer, tx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(sender).To(gomega.Equal(session.Address))
	return tx
}

func submitAndWait(ctx context.Context, session *endtoendlive.Session, tx *types.Transaction) *types.Receipt {
	ginkgo.GinkgoHelper()
	gomega.Expect(session.Execution.SendTransaction(ctx, tx)).To(gomega.Succeed())
	return waitForReceipt(ctx, session, tx)
}

func waitForReceipt(ctx context.Context, session *endtoendlive.Session, tx *types.Transaction) *types.Receipt {
	ginkgo.GinkgoHelper()

	waitCtx, cancel := context.WithTimeout(ctx, transactionTimeout)
	defer cancel()
	receipt, err := bind.WaitMined(waitCtx, session.Execution, tx)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(receipt).NotTo(gomega.BeNil())
	return receipt
}
