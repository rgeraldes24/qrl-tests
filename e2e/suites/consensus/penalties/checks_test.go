//go:build e2e

package penalties

import (
	"context"
	"time"

	"github.com/cyyber/qrl-tests/e2e/internal/beacon"
	"github.com/cyyber/qrl-tests/e2e/internal/staking"
	"github.com/cyyber/qrl-tests/e2e/internal/testsuite"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

// awaitRewards waits until Qrysm serves the attestation rewards of epoch.
func (validator spareValidator) awaitRewards(ctx context.Context, epoch uint64) {
	ginkgo.GinkgoHelper()
	head := testsuite.MustSucceed(validator.client.HeadSlot(ctx))
	remaining := max(int64(epoch+2)-int64(validator.chain.Epoch(head)), 0)
	gomega.Eventually(func(g gomega.Gomega) {
		head, err := validator.client.HeadSlot(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(staking.RewardsServed(validator.chain, head, epoch)).To(gomega.BeTrue())
	}).WithContext(ctx).
		WithTimeout(time.Duration(remaining+2)*validator.chain.EpochDuration).
		WithPolling(staking.PollInterval).
		Should(gomega.Succeed(), "the rewards of epoch %d are not served", epoch)
}

func (validator spareValidator) reward(ctx context.Context, epoch uint64) (beacon.AttestationReward, error) {
	return staking.AttestationReward(ctx, validator.client, epoch, validator.index)
}

// firstAttestedEpoch waits for the first epoch, starting at epoch from, whose
// source and target votes were both timely, and returns it.
func (validator spareValidator) firstAttestedEpoch(ctx context.Context, from uint64, timeout time.Duration) uint64 {
	ginkgo.GinkgoHelper()
	return staking.AwaitAttestationReward(ctx, validator.client, validator.chain, validator.index, from,
		timeout, "timely source and target votes", attested)
}

func attested(reward beacon.AttestationReward) bool {
	return reward.Source > 0 && reward.Target > 0
}
