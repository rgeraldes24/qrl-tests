package health

import (
	"context"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyyber/qrl-tests/e2e/internal/beacon"
	"github.com/cyyber/qrl-tests/e2e/internal/chaininfo"
	containertypes "github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/require"
	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/go-qrl/p2p/qnode"
)

func TestExcept(t *testing.T) {
	own, other := qnode.ID{1}, qnode.ID{2}
	drops := []PeerDrop{{Peer: own}, {Peer: other}, {Peer: own}}
	require.Equal(t, []PeerDrop{{Peer: other}}, except(drops, own))
	require.Len(t, drops, 3, "the drops passed in are not changed")
}

type fakeExecution map[uint64]*types.Header

func (execution fakeExecution) HeaderByNumber(_ context.Context, number *big.Int) (*types.Header, error) {
	header, ok := execution[number.Uint64()]
	if !ok {
		return nil, qrl.NotFound
	}
	return header, nil
}

func (fakeExecution) Close() {}

// fakeMonitor monitors a fake container, node and beacon chain. It started at
// slot 2 with 3 beacon peers; the chain has blocks at 3 and 5, the head's
// payload is execution block 9, the clock is in slot 7 and epoch 3, and epoch 1
// is finalized. The beacon node has 2 peers left and is optimistic.
func fakeMonitor(t *testing.T) (*Monitor, *fakeDocker, *fakeAdmin) {
	t.Helper()
	block := &types.Header{Number: big.NewInt(9)}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/qrl/v1/node/syncing":
			_, _ = writer.Write([]byte(`{"data":{"head_slot":"5","sync_distance":"1","is_syncing":false,"is_optimistic":true,"el_offline":false}}`))
		case "/qrl/v1/node/peer_count":
			_, _ = writer.Write([]byte(`{"data":{"connected":"2"}}`))
		case "/qrl/v1/beacon/blocks/head":
			_, _ = writer.Write([]byte(`{"data":{"message":{"slot":"5","body":{"execution_payload":{"block_number":"9","block_hash":"` + block.Hash().Hex() + `"}}}}}`))
		case "/qrl/v1/beacon/headers/head", "/qrl/v1/beacon/headers/5":
			_, _ = writer.Write([]byte(`{"data":{"root":"0xab","header":{"message":{"slot":"5"}}}}`))
		case "/qrl/v1/beacon/headers/3":
			_, _ = writer.Write([]byte(`{"data":{"root":"0xab","header":{"message":{"slot":"3"}}}}`))
		case "/qrl/v1/beacon/states/head/finality_checkpoints":
			_, _ = writer.Write([]byte(`{"data":{"previous_justified":{"epoch":"0","root":"0x00"},"current_justified":{"epoch":"1","root":"0x00"},"finalized":{"epoch":"1","root":"0x00"}}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	client, err := beacon.New(server.URL)
	require.NoError(t, err)

	docker := &fakeDocker{state: containertypes.State{Running: true, StartedAt: "earlier"}}
	admin, watch := startPeerWatch(t)
	t.Cleanup(watch.Close)
	monitor := &Monitor{
		containers:  &containerStates{docker: docker},
		peers:       watch,
		execution:   fakeExecution{9: block},
		beacon:      client,
		beaconPeers: 3,
		chain: chaininfo.Info{
			GenesisTime:   time.Now().Add(-7*time.Hour - 30*time.Minute),
			SlotsPerEpoch: 2,
			SlotDuration:  time.Hour,
			EpochDuration: 2 * time.Hour,
		},
		firstSlot: 2,
	}
	require.NoError(t, monitor.AddContainers(t.Context(), Container{Name: "node"}))
	return monitor, docker, admin
}

func TestReport(t *testing.T) {
	monitor, docker, admin := fakeMonitor(t)
	own, other := qnode.ID{1}, qnode.ID{2}
	admin.emit(t, dropped(own, "requested"))
	admin.emit(t, dropped(other, "useless peer"))
	awaitDrops(t, monitor.peers, 2)
	docker.state.StartedAt = "later"

	report, err := monitor.Report(t.Context(), own)
	require.NoError(t, err)
	require.Equal(t, []string{"node: restarted"}, report.ContainerProblems)
	require.Equal(t, []string{"beacon node optimistic"}, report.SyncProblems)
	require.Len(t, report.PeerDrops, 1)
	require.Equal(t, other, report.PeerDrops[0].Peer)
	require.Equal(t, uint64(1), report.LostBeaconPeers)
	require.Equal(t, []uint64{4, 6}, report.MissedSlots, "up to the slot before the clock's")
	require.Equal(t, uint64(4), report.DueSlots)
	require.Equal(t, uint64(2), report.FinalityLag)
	require.Equal(t, "container problems: 1, sync problems: 1, peer drops: 1, lost beacon peers: 1, missed slots: 2 of 4, finality lag: 2 epochs", report.String())
}

func TestSyncProblemsHead(t *testing.T) {
	for _, test := range []struct {
		name      string
		execution fakeExecution
		problem   string
	}{
		{name: "a missing block", execution: fakeExecution{}, problem: "execution client lacks the beacon head's block 9"},
		{
			name:      "another block",
			execution: fakeExecution{9: {Number: big.NewInt(9), GasLimit: 1}},
			problem:   "execution block 9 is " + (&types.Header{Number: big.NewInt(9), GasLimit: 1}).Hash().Hex() + ", not the beacon head's",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			monitor, _, _ := fakeMonitor(t)
			monitor.execution = test.execution

			problems, err := monitor.syncProblems(t.Context())
			require.NoError(t, err)
			require.Len(t, problems, 2)
			require.Contains(t, problems[1], test.problem)
		})
	}
}

func TestReportAheadOfClock(t *testing.T) {
	monitor, _, _ := fakeMonitor(t)
	monitor.chain.GenesisTime = time.Now().Add(time.Hour)

	report, err := monitor.Report(t.Context())
	require.NoError(t, err)
	require.Empty(t, report.ContainerProblems)
	require.Equal(t, []uint64{4}, report.MissedSlots, "up to the head")
	require.Equal(t, uint64(3), report.DueSlots)
	require.Zero(t, report.FinalityLag)
}

func TestReportFailure(t *testing.T) {
	monitor, docker, _ := fakeMonitor(t)
	docker.state.Running = false
	monitor.beacon, _ = beacon.New("http://127.0.0.1:1")

	_, err := monitor.Report(t.Context())
	require.ErrorContains(t, err, `the containers had ["node: exited with code 0"]`)

	docker.err = errors.New("gone")
	_, err = monitor.Report(t.Context())
	require.ErrorContains(t, err, "node: gone")
	require.NotContains(t, err.Error(), "the containers had")
}
