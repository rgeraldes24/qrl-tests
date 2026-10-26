package penalties

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/cyyber/qrl-tests/e2e/internal/beacon"
	"github.com/cyyber/qrl-tests/e2e/internal/chaininfo"
)

// rewardSpec holds the consensus constants of the Altair base reward.
type rewardSpec struct {
	maxEffectiveBalance          uint64
	increment                    uint64
	baseRewardFactor             uint64
	sourceWeight                 uint64
	targetWeight                 uint64
	weightDenominator            uint64
	minEpochsToInactivityPenalty uint64
}

func loadRewardSpec(ctx context.Context, client *beacon.Client) (rewardSpec, error) {
	var spec rewardSpec
	for _, constant := range []struct {
		name  string
		field *uint64
	}{
		{"MAX_EFFECTIVE_BALANCE", &spec.maxEffectiveBalance},
		{"EFFECTIVE_BALANCE_INCREMENT", &spec.increment},
		{"BASE_REWARD_FACTOR", &spec.baseRewardFactor},
		{"TIMELY_SOURCE_WEIGHT", &spec.sourceWeight},
		{"TIMELY_TARGET_WEIGHT", &spec.targetWeight},
		{"WEIGHT_DENOMINATOR", &spec.weightDenominator},
		{"MIN_EPOCHS_TO_INACTIVITY_PENALTY", &spec.minEpochsToInactivityPenalty},
	} {
		value, err := client.SpecUint(ctx, constant.name)
		if err != nil {
			return rewardSpec{}, err
		}
		*constant.field = value
	}
	return spec, nil
}

// missedVotePenalties returns what a validator with effectiveBalance loses
// for missing the source and the target vote of an epoch outside an
// inactivity leak. Its inactivity score then stays zero and adds nothing.
func (spec rewardSpec) missedVotePenalties(effectiveBalance, totalActiveBalance uint64) (source, target uint64) {
	root := new(big.Int).Sqrt(new(big.Int).SetUint64(totalActiveBalance)).Uint64()
	baseReward := effectiveBalance / spec.increment * (spec.increment * spec.baseRewardFactor / root)
	return baseReward * spec.sourceWeight / spec.weightDenominator,
		baseReward * spec.targetWeight / spec.weightDenominator
}

// spareValidator reads the balance and the rewards of the validator the
// suite runs.
type spareValidator struct {
	client *beacon.Client
	chain  chaininfo.Info
	spec   rewardSpec
	index  uint64
}

// expectedMissedVotePenalties computes the penalties for missing the votes of
// epoch from the state the rewards of epoch are processed in: the one at the
// last slot of the next epoch. It refuses an epoch processed in an inactivity
// leak, whose penalty Qrysm folds into the target score.
func (validator spareValidator) expectedMissedVotePenalties(ctx context.Context, epoch uint64) (source, target uint64, err error) {
	stateID := strconv.FormatUint((epoch+2)*validator.chain.SlotsPerEpoch-1, 10)
	checkpoints, err := validator.client.FinalityCheckpointsAt(ctx, stateID)
	if err != nil {
		return 0, 0, err
	}
	if finalized := checkpoints.Finalized.Epoch; epoch > finalized+validator.spec.minEpochsToInactivityPenalty {
		return 0, 0, fmt.Errorf(
			"the rewards of epoch %d are processed in an inactivity leak: the last finalized epoch is %d", epoch, finalized)
	}
	record, err := validator.client.ValidatorAt(ctx, stateID, strconv.FormatUint(validator.index, 10))
	if err != nil {
		return 0, 0, err
	}
	total, err := validator.client.TotalActiveBalance(ctx, stateID)
	if err != nil {
		return 0, 0, err
	}
	source, target = validator.spec.missedVotePenalties(record.EffectiveBalance, total)
	return source, target, nil
}

// balanceAtEpochStart is the balance in the state at the first slot of epoch:
// after the processing of the epoch before it, which applies the attestation
// rewards of the epoch before that, and after the block at that slot.
func (validator spareValidator) balanceAtEpochStart(ctx context.Context, epoch uint64) (uint64, error) {
	stateID := strconv.FormatUint(epoch*validator.chain.SlotsPerEpoch, 10)
	record, err := validator.client.ValidatorAt(ctx, stateID, strconv.FormatUint(validator.index, 10))
	return record.Balance, err
}

// blockBalanceChanges sums what the blocks after the first slot of epoch,
// through the first slot of the next, did to the balance of a validator that
// proposes none of them: its sync committee rewards, negative for missed
// duties, less its withdrawals. Those blocks lie between balanceAtEpochStart
// of epoch and of the next.
func (validator spareValidator) blockBalanceChanges(ctx context.Context, epoch uint64) (int64, error) {
	var change int64
	start := epoch * validator.chain.SlotsPerEpoch
	for slot := start + 1; slot <= start+validator.chain.SlotsPerEpoch; slot++ {
		blockID := strconv.FormatUint(slot, 10)
		operations, err := validator.client.BlockOperations(ctx, blockID)
		if beacon.IsNotFound(err) {
			continue
		}
		if err != nil {
			return 0, err
		}
		for _, withdrawal := range operations.Withdrawals {
			if withdrawal.ValidatorIndex == validator.index {
				change -= int64(withdrawal.Amount)
			}
		}
		rewards, err := validator.client.SyncCommitteeRewards(ctx, blockID, []uint64{validator.index})
		if err != nil {
			return 0, err
		}
		for _, reward := range rewards {
			change += reward.Reward
		}
	}
	return change, nil
}
