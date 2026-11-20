package health

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/cyyber/qrl-tests/e2e/internal/beacon"
	"github.com/cyyber/qrl-tests/e2e/internal/chaininfo"
	"github.com/cyyber/qrl-tests/internal/dockerapi"
	dockerclient "github.com/moby/moby/client"
	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/p2p/qnode"
	"github.com/theQRL/go-qrl/qrlclient"
)

// Monitor follows a participant's containers, peers and chain during a suite.
type Monitor struct {
	containers  *containerStates
	docker      dockerclient.APIClient
	peers       *PeerWatch
	execution   headerReader
	beacon      *beacon.Client
	beaconPeers uint64
	chain       chaininfo.Info
	firstSlot   uint64
}

type headerReader interface {
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	Close()
}

// StartMonitor starts monitoring participant.
func StartMonitor(ctx context.Context, participant devnet.Participant, client *beacon.Client, chain chaininfo.Info) (*Monitor, error) {
	docker, err := dockerapi.New()
	if err != nil {
		return nil, fmt.Errorf("create Docker client: %w", err)
	}

	monitor := &Monitor{containers: &containerStates{docker: docker}, docker: docker, beacon: client, chain: chain}
	if err := monitor.start(ctx, participant); err != nil {
		monitor.Close()
		return nil, err
	}
	return monitor, nil
}

func (monitor *Monitor) start(ctx context.Context, participant devnet.Participant) error {
	containers, err := ParticipantContainers(ctx, monitor.docker, participant)
	if err != nil {
		return err
	}
	if err := monitor.containers.record(ctx, containers...); err != nil {
		return err
	}

	if monitor.peers, err = WatchPeers(ctx, participant.Execution.WebSocketURL); err != nil {
		return err
	}
	if monitor.execution, err = qrlclient.DialContext(ctx, participant.Execution.RPCURL); err != nil {
		return fmt.Errorf("dial execution RPC: %w", err)
	}

	if monitor.beaconPeers, err = monitor.beacon.PeerCount(ctx); err != nil {
		return err
	}
	monitor.firstSlot, err = monitor.beacon.HeadSlot(ctx)
	return err
}

// AddContainers records containers started later, such as a sync node, for
// the report.
func (monitor *Monitor) AddContainers(ctx context.Context, containers ...Container) error {
	return monitor.containers.record(ctx, containers...)
}

// RetireContainer takes a container's last reading, for the report, before it
// is removed.
func (monitor *Monitor) RetireContainer(ctx context.Context, name string) error {
	return monitor.containers.retire(ctx, name)
}

// Close releases the monitor's Docker client, peer watch and execution client.
func (monitor *Monitor) Close() {
	if monitor.peers != nil {
		monitor.peers.Close()
	}
	if monitor.execution != nil {
		monitor.execution.Close()
	}
	_ = monitor.docker.Close()
}

// Report is what a monitor has seen since it started.
type Report struct {
	ContainerProblems []string
	SyncProblems      []string
	PeerDrops         []PeerDrop
	LostBeaconPeers   uint64
	MissedSlots       []uint64
	DueSlots          uint64
	FinalityLag       uint64
}

const MaxFinalityLag = 3

func (report Report) String() string {
	return fmt.Sprintf("container problems: %d, sync problems: %d, peer drops: %d, lost beacon peers: %d, missed slots: %d of %d, finality lag: %d epochs",
		len(report.ContainerProblems), len(report.SyncProblems), len(report.PeerDrops), report.LostBeaconPeers,
		len(report.MissedSlots), report.DueSlots, report.FinalityLag)
}

// Report returns what the monitor has seen. It leaves out the drops of the
// given peers, which the suite disconnects itself.
func (monitor *Monitor) Report(ctx context.Context, ownPeers ...qnode.ID) (Report, error) {
	// A node that is down fails the steps below, and its problems say why.
	problems, err := monitor.containers.problems(ctx)
	fail := func(err error) (Report, error) {
		if len(problems) > 0 {
			err = fmt.Errorf("%w; the containers had %q", err, problems)
		}
		return Report{}, err
	}
	if err != nil {
		return fail(err)
	}

	drops, err := monitor.peers.Drops(ctx)
	if err != nil {
		return fail(err)
	}

	syncProblems, err := monitor.syncProblems(ctx)
	if err != nil {
		return fail(err)
	}

	beaconPeers, err := monitor.beacon.PeerCount(ctx)
	if err != nil {
		return fail(err)
	}

	head, err := monitor.beacon.HeadSlot(ctx)
	if err != nil {
		return fail(err)
	}

	// The clock's slot may still get its block; every slot before it is due one.
	now := time.Now()
	last := max(head+1, monitor.chain.SlotAt(now)) - 1
	missed, err := MissedSlots(ctx, monitor.beacon, monitor.firstSlot+1, last)
	if err != nil {
		return fail(err)
	}

	checkpoints, err := monitor.beacon.FinalityCheckpoints(ctx)
	if err != nil {
		return fail(err)
	}

	epoch := monitor.chain.EpochAt(now)
	return Report{
		ContainerProblems: problems,
		SyncProblems:      syncProblems,
		PeerDrops:         except(drops, ownPeers...),
		LostBeaconPeers:   monitor.beaconPeers - min(monitor.beaconPeers, beaconPeers),
		MissedSlots:       missed,
		DueSlots:          last - min(last, monitor.firstSlot),
		FinalityLag:       epoch - min(epoch, checkpoints.Finalized.Epoch),
	}, nil
}

// syncProblems returns how the beacon node is out of sync: syncing, optimistic,
// cut off from its execution client, or on a head that client does not have.
func (monitor *Monitor) syncProblems(ctx context.Context) ([]string, error) {
	status, err := monitor.beacon.SyncStatus(ctx)
	if err != nil {
		return nil, err
	}

	var problems []string
	if status.IsSyncing {
		problems = append(problems, fmt.Sprintf("beacon node syncing, %d slots behind", status.SyncDistance))
	}
	if status.IsOptimistic {
		problems = append(problems, "beacon node optimistic")
	}
	if status.ELOffline {
		problems = append(problems, "execution client offline")
	}

	// The clients may be mid-update, so a mismatch is checked again.
	problem, err := monitor.headProblem(ctx)
	if problem != "" && err == nil {
		select {
		case <-ctx.Done():
			return problems, ctx.Err()
		case <-time.After(time.Second):
		}
		problem, err = monitor.headProblem(ctx)
	}
	if problem != "" {
		problems = append(problems, problem)
	}
	return problems, err
}

// headProblem returns how the execution client's block differs from the beacon
// head's payload, if it does.
func (monitor *Monitor) headProblem(ctx context.Context) (string, error) {
	payload, err := monitor.beacon.ExecutionBlock(ctx, "head")
	if err != nil {
		return "", err
	}
	header, err := monitor.execution.HeaderByNumber(ctx, new(big.Int).SetUint64(payload.Number))
	switch {
	case errors.Is(err, qrl.NotFound):
		return fmt.Sprintf("execution client lacks the beacon head's block %d", payload.Number), nil
	case err != nil:
		return "", err
	case header.Hash().Hex() != payload.Hash:
		return fmt.Sprintf("execution block %d is %s, not the beacon head's %s", payload.Number, header.Hash().Hex(), payload.Hash), nil
	}
	return "", nil
}

// except returns the drops of peers other than the given ones.
func except(drops []PeerDrop, peers ...qnode.ID) []PeerDrop {
	return slices.DeleteFunc(slices.Clone(drops), func(drop PeerDrop) bool {
		return slices.Contains(peers, drop.Peer)
	})
}
