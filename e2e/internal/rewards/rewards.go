// Package rewards reads a validator's attestation rewards from a Qrysm beacon
// node and waits for the first epoch whose reward meets a suite's condition.
package rewards

import (
	"context"
	"fmt"
	"time"

	"github.com/cyyber/qrl-tests/e2e/internal/beacon"
	"github.com/cyyber/qrl-tests/e2e/internal/chaininfo"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

const PollInterval = 2 * time.Second

// AwaitAttestationReward checks the validator's reward for each epoch,
// starting at epoch from, once it is served, and returns the first epoch whose
// reward satisfies match. want describes match in the failure message.
func AwaitAttestationReward(
	ctx context.Context,
	client *beacon.Client,
	chain chaininfo.Info,
	index, from uint64,
	timeout time.Duration,
	want string,
	match func(beacon.AttestationReward) bool,
) uint64 {
	ginkgo.GinkgoHelper()
	next := from
	gomega.Eventually(func(g gomega.Gomega) {
		var found bool
		var err error
		next, found, err = rewardedFrom(ctx, client, chain, index, next, match)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(found).To(gomega.BeTrue(), "validator %d has no epoch since %d with %s", index, from, want)
	}).WithContext(ctx).
		WithTimeout(timeout).
		WithPolling(PollInterval).
		Should(gomega.Succeed())
	return next
}

// AttestationReward returns the attestation scores of one validator in a
// completed epoch.
func AttestationReward(ctx context.Context, client *beacon.Client, epoch, index uint64) (beacon.AttestationReward, error) {
	rewards, err := client.AttestationRewards(ctx, epoch, []uint64{index})
	if err != nil {
		return beacon.AttestationReward{}, err
	}
	if len(rewards) != 1 || rewards[0].ValidatorIndex != index {
		return beacon.AttestationReward{}, fmt.Errorf("epoch %d rewards do not cover validator %d", epoch, index)
	}
	return rewards[0], nil
}

// Served reports whether Qrysm serves the attestation rewards of epoch
// once the head is at headSlot: only two epochs later, so every attestation
// has had a chance of inclusion.
func Served(chain chaininfo.Info, headSlot, epoch uint64) bool {
	return epoch+2 <= chain.Epoch(headSlot)
}

// rewardedFrom checks the validator's attestation rewards for each epoch,
// starting at epoch from, through the latest epoch whose rewards are served,
// two before the head. It reports whether one satisfied match, and the epoch
// to check next, which is that one if it did.
func rewardedFrom(
	ctx context.Context,
	client *beacon.Client,
	chain chaininfo.Info,
	index, from uint64,
	match func(beacon.AttestationReward) bool,
) (uint64, bool, error) {
	head, err := client.HeadSlot(ctx)
	if err != nil {
		return from, false, err
	}
	epoch := from
	for ; Served(chain, head, epoch); epoch++ {
		reward, err := AttestationReward(ctx, client, epoch, index)
		if err != nil {
			return epoch, false, err
		}
		if match(reward) {
			return epoch, true, nil
		}
	}
	return epoch, false, nil
}
