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
// starting at epoch from, once it is available, and returns the first epoch whose
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
	scanner := &rewardScanner{client: client, chain: chain, index: index, match: match, next: from, from: from}
	gomega.Eventually(func(g gomega.Gomega) {
		found, err := scanner.scan(ctx)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(found).To(gomega.BeTrue(), "validator %d has no epoch since %d with %s: %s",
			index, from, want, scanner.progress())
	}).WithContext(ctx).
		WithTimeout(timeout).
		WithPolling(PollInterval).
		Should(gomega.Succeed())
	return scanner.next
}

// AttestationReward returns one validator's attestation reward for a completed
// epoch: its head, source and target rewards in shor, negative for a penalty.
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

// Available reports whether Qrysm serves the attestation rewards of epoch
// once the head is at headSlot: only two epochs later, so every attestation
// has had a chance of inclusion.
func Available(chain chaininfo.Info, headSlot, epoch uint64) bool {
	return epoch+2 <= chain.Epoch(headSlot)
}

// rewardScanner reads one validator's attestation rewards epoch by epoch, so
// polling callers read each epoch once. It starts at epoch from; next is the
// epoch it reads next, or the one that satisfied match, and last is the last
// reward it read, if any.
type rewardScanner struct {
	client *beacon.Client
	chain  chaininfo.Info
	index  uint64
	match  func(beacon.AttestationReward) bool
	next   uint64
	from   uint64
	last   *beacon.AttestationReward
}

// scan reads the rewards from next through the latest epoch whose rewards are
// available, two before the head, and reports whether one satisfied match. A
// failed read leaves next at its epoch, so the retry does not skip it.
func (scanner *rewardScanner) scan(ctx context.Context) (bool, error) {
	head, err := scanner.client.HeadSlot(ctx)
	if err != nil {
		return false, err
	}
	for ; Available(scanner.chain, head, scanner.next); scanner.next++ {
		reward, err := AttestationReward(ctx, scanner.client, scanner.next, scanner.index)
		if err != nil {
			return false, err
		}
		scanner.last = &reward
		if scanner.match(reward) {
			return true, nil
		}
	}
	return false, nil
}

// progress describes how far an unmatched scan got, for a failure message.
func (scanner *rewardScanner) progress() string {
	if scanner.last == nil {
		return fmt.Sprintf("the rewards of epoch %d are not available yet", scanner.from)
	}
	last := *scanner.last
	read := fmt.Sprintf("read epoch %d", scanner.from)
	if lastEpoch := scanner.next - 1; lastEpoch > scanner.from {
		read = fmt.Sprintf("read epochs %d to %d", scanner.from, lastEpoch)
	}
	return fmt.Sprintf("%s, the last with head %d, source %d and target %d", read, last.Head, last.Source, last.Target)
}
