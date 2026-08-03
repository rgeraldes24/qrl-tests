//go:build e2e

package sync_test

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/endtoend/internal/execfixture"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/p2p"
	"github.com/theQRL/go-qrl/qrlclient/gqrlclient"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const executionSyncTimeout = 20 * time.Minute

var _ = ginkgo.Describe(
	"fresh execution sync and persistence",
	ginkgo.Serial,
	ginkgo.Ordered,
	ginkgo.Label("e2e", "live", "execution", "sync", "profile-execution-sync", "mutates-network"),
	func() {
		var primary, secondary *endtoendlive.Session
		var services *devnet.ServiceController
		var contract execfixture.StateContract
		var value common.StorageValue64
		var receipt *types.Receipt
		var canonical *types.Block
		var code []byte
		var contractBalance *big.Int
		var secondaryConsensusStopped bool

		ginkgo.BeforeAll(func(ctx ginkgo.SpecContext) {
			sessions, err := endtoendlive.OpenAll(ctx, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(sessions).To(gomega.HaveLen(2))
			primary, secondary = sessions[0], sessions[1]
			for _, session := range sessions {
				ginkgo.DeferCleanup(session.Close)
			}
			services = devnet.NewServiceController(primary.Environment.EnclaveName)
			gomega.Expect(services.Stop(ctx, secondary.Participant.ConsensusServiceName)).To(gomega.Succeed())
			secondaryConsensusStopped = true
			ginkgo.DeferCleanup(func(cleanupCtx ginkgo.SpecContext) {
				if secondaryConsensusStopped {
					gomega.Expect(services.Start(cleanupCtx, secondary.Participant.ConsensusServiceName)).To(gomega.Succeed())
				}
			})
		})

		ginkgo.It("builds deterministic full-width state on the source", func(ctx ginkgo.SpecContext) {
			var err error
			contract, err = execfixture.DeployStateContract(ctx, primary, execfixture.FullTopic(0x70))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			value = execfixture.FullWord(0x30)
			nonce, err := primary.Execution.PendingNonceAt(ctx, primary.Address)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			tx, err := execfixture.SignCall(ctx, primary, nonce, contract.Address, big.NewInt(987654321), value[:])
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(primary.Execution.SendTransaction(ctx, tx)).To(gomega.Succeed())
			gomega.Eventually(func() error {
				receipt, err = primary.Execution.TransactionReceipt(ctx, tx.Hash())
				return err
			}).WithContext(ctx).WithTimeout(3 * time.Minute).WithPolling(time.Second).Should(gomega.Succeed())
			gomega.Expect(receipt.Status).To(gomega.Equal(types.ReceiptStatusSuccessful))
			gomega.Expect(receipt.Logs).To(gomega.HaveLen(1))
			gomega.Expect(receipt.Logs[0].Topics).To(gomega.Equal([]common.LogTopic{contract.Topic}))
			gomega.Expect(receipt.Logs[0].Data).To(gomega.Equal(value[:]))
			canonical, err = primary.Execution.BlockByHash(ctx, receipt.BlockHash)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			code, err = primary.Execution.CodeAt(ctx, contract.Address, canonical.Number())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			contractBalance, err = primary.Execution.BalanceAt(ctx, contract.Address, canonical.Number())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			secondaryHead, err := secondary.Execution.BlockNumber(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(secondaryHead).To(gomega.BeNumerically("<", canonical.NumberU64()))
		}, ginkgo.SpecTimeout(executionSyncTimeout), ginkgo.Label("behavior:execution-sync:source-state"))

		ginkgo.It("syncs the isolated execution client and verifies exact state", func(ctx ginkgo.SpecContext) {
			var nodeInfo p2p.NodeInfo
			gomega.Expect(primary.Execution.Client().CallContext(ctx, &nodeInfo, "admin_nodeInfo")).To(gomega.Succeed())
			gomega.Expect(nodeInfo.Qnode).NotTo(gomega.BeEmpty())
			var added bool
			gomega.Expect(secondary.Execution.Client().CallContext(ctx, &added, "admin_addPeer", nodeInfo.Qnode)).To(gomega.Succeed())
			gomega.Expect(added).To(gomega.BeTrue())
			gomega.Expect(services.Start(ctx, secondary.Participant.ConsensusServiceName)).To(gomega.Succeed())
			secondaryConsensusStopped = false

			awaitExecutionState(ctx, secondary, canonical)
			assertState(ctx, secondary, canonical, receipt, contract, value, code, contractBalance)
		}, ginkgo.SpecTimeout(executionSyncTimeout), ginkgo.Label("behavior:execution-sync:fresh-database"))

		ginkgo.It("preserves the synced state across an execution-client restart", func(ctx ginkgo.SpecContext) {
			participantIndex := secondary.Participant.Index
			gomega.Expect(services.Restart(ctx, secondary.Participant.ExecutionServiceName)).To(gomega.Succeed())
			var replacement *endtoendlive.Session
			gomega.Eventually(func() error {
				var err error
				replacement, err = endtoendlive.OpenParticipant(ctx, participantIndex, false)
				return err
			}).WithContext(ctx).WithTimeout(executionSyncTimeout).WithPolling(time.Second).Should(gomega.Succeed())
			secondary = replacement
			ginkgo.DeferCleanup(secondary.Close)
			gomega.Eventually(func() error {
				return assertExecutionState(ctx, secondary, canonical)
			}).WithContext(ctx).WithTimeout(executionSyncTimeout).WithPolling(time.Second).Should(gomega.Succeed())
			assertState(ctx, secondary, canonical, receipt, contract, value, code, contractBalance)
		}, ginkgo.SpecTimeout(executionSyncTimeout), ginkgo.Label("behavior:execution-sync:persistence"))
	},
)

func awaitExecutionState(ctx context.Context, session *endtoendlive.Session, block *types.Block) {
	ginkgo.GinkgoHelper()
	gomega.Eventually(func() error {
		return assertExecutionState(ctx, session, block)
	}).WithContext(ctx).WithTimeout(executionSyncTimeout).WithPolling(time.Second).Should(gomega.Succeed())
}

func assertExecutionState(ctx context.Context, session *endtoendlive.Session, block *types.Block) error {
	progress, err := session.Execution.SyncProgress(ctx)
	if err != nil {
		return err
	}
	if progress != nil {
		return fmt.Errorf("execution sync is in progress: %+v", progress)
	}
	head, err := session.Execution.BlockNumber(ctx)
	if err != nil {
		return err
	}
	if head < block.NumberU64() {
		return fmt.Errorf("head %d is behind fixture block %d", head, block.NumberU64())
	}
	stored, err := session.Execution.BlockByNumber(ctx, block.Number())
	if err != nil {
		return err
	}
	if stored.Hash() != block.Hash() || stored.Root() != block.Root() {
		return fmt.Errorf("fixture block mismatch: got %s/%s, want %s/%s", stored.Hash(), stored.Root(), block.Hash(), block.Root())
	}
	return nil
}

func assertState(
	ctx context.Context,
	session *endtoendlive.Session,
	block *types.Block,
	receipt *types.Receipt,
	contract execfixture.StateContract,
	value common.StorageValue64,
	code []byte,
	balance *big.Int,
) {
	ginkgo.GinkgoHelper()
	storedReceipt, err := session.Execution.TransactionReceipt(ctx, receipt.TxHash)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(storedReceipt.BlockHash).To(gomega.Equal(block.Hash()))
	storedCode, err := session.Execution.CodeAt(ctx, contract.Address, block.Number())
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(storedCode).To(gomega.Equal(code))
	storedValue, err := session.Execution.StorageAt(ctx, contract.Address, common.Hash{}, block.Number())
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(bytes.Equal(storedValue, value[:])).To(gomega.BeTrue())
	storedBalance, err := session.Execution.BalanceAt(ctx, contract.Address, block.Number())
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(storedBalance).To(gomega.Equal(balance))
	logs, err := session.Execution.FilterLogs(ctx, qrl.FilterQuery{
		FromBlock: block.Number(), ToBlock: block.Number(), Addresses: []common.Address{contract.Address},
		Topics: [][]common.LogTopic{{contract.Topic}},
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(logs).To(gomega.HaveLen(1))
	gomega.Expect(logs[0].Data).To(gomega.Equal(value[:]))
	proof, err := gqrlclient.New(session.Execution.Client()).GetProof(ctx, contract.Address, []string{"0x0"}, block.Number())
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(proof.StorageProof).To(gomega.HaveLen(1))
	gomega.Expect(proof.StorageProof[0].Value).To(gomega.Equal(new(big.Int).SetBytes(value[:])))
}
