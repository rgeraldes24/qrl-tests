package rewards

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cyyber/qrl-tests/e2e/internal/beacon"
	"github.com/cyyber/qrl-tests/e2e/internal/chaininfo"
	"github.com/stretchr/testify/require"
)

const validatorIndex = 64

// fakeBeacon serves the head and the attestation rewards the scan reads.
// Handlers run on the server goroutine, so its state is guarded by mu.
type fakeBeacon struct {
	mu       sync.Mutex
	head     uint64
	rewards  map[uint64]string
	failures map[string]int
	requests map[string]int
}

func newFakeBeacon(t *testing.T, head uint64) (*fakeBeacon, *beacon.Client) {
	t.Helper()
	fake := &fakeBeacon{
		head:     head,
		rewards:  map[uint64]string{},
		failures: map[string]int{},
		requests: map[string]int{},
	}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client, err := beacon.New(server.URL)
	require.NoError(t, err)
	return fake, client
}

func (fake *fakeBeacon) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	path := request.URL.Path
	fake.requests[path]++
	if fake.failures[path] > 0 {
		fake.failures[path]--
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var data string
	var ok bool
	switch {
	case path == "/qrl/v1/beacon/headers/head":
		data, ok = fmt.Sprintf(`{"header":{"message":{"slot":"%d"}}}`, fake.head), true
	case strings.HasPrefix(path, "/qrl/v1/beacon/rewards/attestations/"):
		epoch, _ := strconv.ParseUint(strings.TrimPrefix(path, "/qrl/v1/beacon/rewards/attestations/"), 10, 64)
		data, ok = fake.rewards[epoch]
	}
	if !ok {
		http.NotFound(writer, request)
		return
	}
	_ = json.NewEncoder(writer).Encode(map[string]json.RawMessage{"data": json.RawMessage(data)})
}

func (fake *fakeBeacon) set(update func(*fakeBeacon)) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	update(fake)
}

func (fake *fakeBeacon) requestCount(path string) int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.requests[path]
}

func reward(index uint64, head, target int64) string {
	return fmt.Sprintf(`{"total_rewards":[{"validator_index":"%d","head":"%d","target":"%d","source":"0"}]}`,
		index, head, target)
}

// anyReward is the staking suites' condition: any vote earned a reward.
func anyReward(reward beacon.AttestationReward) bool {
	return reward.Head > 0 || reward.Target > 0 || reward.Source > 0
}

func TestServed(t *testing.T) {
	chain := chaininfo.Info{SlotsPerEpoch: 8}
	// Slot 23 is in epoch 2, which serves epoch 0 only.
	require.True(t, Served(chain, 23, 0))
	require.False(t, Served(chain, 23, 1))
	require.True(t, Served(chain, 24, 1))
}

func TestRewardedFrom(t *testing.T) {
	chain := chaininfo.Info{SlotsPerEpoch: 8}
	fake, client := newFakeBeacon(t, 25)
	fake.set(func(fake *fakeBeacon) {
		fake.rewards[1] = reward(validatorIndex, 0, 0)
		fake.rewards[2] = reward(validatorIndex, -5, -5)
		fake.rewards[3] = reward(validatorIndex, 7, 7)
	})

	// The head is in epoch 3, so rewards are served through epoch 1.
	next, attested, err := rewardedFrom(t.Context(), client, chain, validatorIndex, 1, anyReward)
	require.NoError(t, err)
	require.False(t, attested)
	require.Equal(t, uint64(2), next)

	// A missed epoch is followed by a rewarded one.
	fake.set(func(fake *fakeBeacon) { fake.head = 41 })
	next, attested, err = rewardedFrom(t.Context(), client, chain, validatorIndex, next, anyReward)
	require.NoError(t, err)
	require.True(t, attested)
	require.Equal(t, uint64(3), next)
	require.Equal(t, 1, fake.requestCount("/qrl/v1/beacon/rewards/attestations/1"))

	// A failed request keeps its epoch as next, so the retry does not skip it.
	fake.set(func(fake *fakeBeacon) { fake.failures["/qrl/v1/beacon/rewards/attestations/3"] = 1 })
	next, attested, err = rewardedFrom(t.Context(), client, chain, validatorIndex, 3, anyReward)
	require.Error(t, err)
	require.False(t, attested)
	require.Equal(t, uint64(3), next)

	// A late attestation misses the head reward but still earns the target one.
	fake.set(func(fake *fakeBeacon) { fake.rewards[3] = reward(validatorIndex, 0, 7) })
	_, attested, err = rewardedFrom(t.Context(), client, chain, validatorIndex, 3, anyReward)
	require.NoError(t, err)
	require.True(t, attested)

	// Rewards for another validator are an error, not a miss.
	fake.set(func(fake *fakeBeacon) { fake.rewards[3] = reward(validatorIndex+1, 7, 7) })
	_, _, err = rewardedFrom(t.Context(), client, chain, validatorIndex, 3, anyReward)
	require.ErrorContains(t, err, "epoch 3 rewards do not cover validator 64")
}
