// Package stability verifies network convergence after mutating workloads.
package stability

import (
	"context"
	"fmt"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/clients/consensus"
	endtoendlive "github.com/cyyber/qrl-tests/endtoend/internal/live"
)

func Await(ctx context.Context, sessions []*endtoendlive.Session, startEpoch, advance uint64) error {
	beacons := make([]*consensus.Client, len(sessions))
	for index, session := range sessions {
		beacon, err := consensus.New(session.Participant.ConsensusURL)
		if err != nil {
			return err
		}
		beacons[index] = beacon
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastErr error
	for {
		if lastErr = stable(ctx, sessions, beacons, startEpoch+advance); lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("network did not stabilize: %w: %v", ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func stable(
	ctx context.Context,
	sessions []*endtoendlive.Session,
	beacons []*consensus.Client,
	targetEpoch uint64,
) error {
	var expected consensus.Checkpoint
	for index, beacon := range beacons {
		status, err := beacon.Syncing(ctx)
		if err != nil {
			return err
		}
		if status.Syncing || status.Optimistic || status.ELOffline {
			return fmt.Errorf("consensus participant %d is not ready: %+v", index+1, status)
		}
		checkpoint, err := beacon.FinalizedCheckpoint(ctx)
		if err != nil {
			return err
		}
		if checkpoint.Epoch < targetEpoch {
			return fmt.Errorf("consensus participant %d finalized epoch %d, need %d", index+1, checkpoint.Epoch, targetEpoch)
		}
		if index == 0 {
			expected = checkpoint
		} else if checkpoint != expected {
			return fmt.Errorf("finalized checkpoints have not converged: %+v != %+v", checkpoint, expected)
		}
		progress, err := sessions[index].Execution.SyncProgress(ctx)
		if err != nil {
			return err
		}
		if progress != nil {
			return fmt.Errorf("execution participant %d is still syncing", index+1)
		}
	}
	return nil
}
