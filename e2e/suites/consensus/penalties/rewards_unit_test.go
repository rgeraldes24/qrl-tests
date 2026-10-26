package penalties

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cyyber/qrl-tests/e2e/internal/beacon"
	"github.com/cyyber/qrl-tests/e2e/internal/chaininfo"
	"github.com/stretchr/testify/require"
)

const (
	shorPerQRL = 1_000_000_000
	spareIndex = 64
)

func testRewardSpec() rewardSpec {
	return rewardSpec{
		maxEffectiveBalance:          40_000 * shorPerQRL,
		increment:                    shorPerQRL,
		baseRewardFactor:             2048,
		sourceWeight:                 14,
		targetWeight:                 26,
		weightDenominator:            64,
		minEpochsToInactivityPenalty: 4,
	}
}

func TestMissedVotePenalties(t *testing.T) {
	spec := testRewardSpec()

	// 65 validators at 40,000 QRL: the square root of the total active
	// balance is 50,990,195, so each increment's base reward is 40,164 shor.
	source, target := spec.missedVotePenalties(40_000*shorPerQRL, 65*40_000*shorPerQRL)
	require.Equal(t, uint64(351_435_000), source)
	require.Equal(t, uint64(652_665_000), target)

	// A fraction of an increment counts for nothing, and the total includes
	// the validator's own lower effective balance.
	source, target = spec.missedVotePenalties(39_997*shorPerQRL+shorPerQRL/2, 64*40_000*shorPerQRL+39_997*shorPerQRL)
	require.Equal(t, uint64(351_408_642), source)
	require.Equal(t, uint64(652_616_050), target)
}

// fakeBeacon serves canned data per path and records every path requested.
// Handlers run on the server goroutine, so its state is guarded by mu.
type fakeBeacon struct {
	mu        sync.Mutex
	responses map[string]string
	requested []string
}

func newFakeBeacon(t *testing.T, responses map[string]string) (*fakeBeacon, *beacon.Client) {
	t.Helper()
	fake := &fakeBeacon{responses: responses}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client, err := beacon.New(server.URL)
	require.NoError(t, err)
	return fake, client
}

func (fake *fakeBeacon) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	path := strings.TrimPrefix(request.URL.Path, "/qrl/v1/")
	fake.requested = append(fake.requested, path)
	data, ok := fake.responses[path]
	if !ok {
		http.NotFound(writer, request)
		return
	}
	_, _ = fmt.Fprintf(writer, `{"data":%s}`, data)
}

func (fake *fakeBeacon) paths() []string {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return append([]string(nil), fake.requested...)
}

func testSpare(client *beacon.Client) spareValidator {
	return spareValidator{
		client: client,
		chain:  chaininfo.Info{SlotsPerEpoch: 8},
		spec:   testRewardSpec(),
		index:  spareIndex,
	}
}

func validatorJSON(balance, effectiveBalance uint64) string {
	return fmt.Sprintf(`{"index":"%d","balance":"%d","validator":{"effective_balance":"%d"}}`,
		spareIndex, balance, effectiveBalance)
}

func finalizedJSON(epoch uint64) string {
	return fmt.Sprintf(`{"finalized":{"epoch":"%d","root":"0x00"}}`, epoch)
}

func TestLoadRewardSpec(t *testing.T) {
	_, client := newFakeBeacon(t, map[string]string{"config/spec": `{
		"MAX_EFFECTIVE_BALANCE":"40000000000000","EFFECTIVE_BALANCE_INCREMENT":"1000000000",
		"BASE_REWARD_FACTOR":"2048","TIMELY_SOURCE_WEIGHT":"14","TIMELY_TARGET_WEIGHT":"26",
		"WEIGHT_DENOMINATOR":"64","MIN_EPOCHS_TO_INACTIVITY_PENALTY":"4"}`})

	spec, err := loadRewardSpec(t.Context(), client)
	require.NoError(t, err)
	require.Equal(t, testRewardSpec(), spec)
}

func TestExpectedMissedVotePenaltiesReadsTheProcessingState(t *testing.T) {
	// The rewards of epoch 5 are processed in the state at slot 55, the last
	// of epoch 6.
	responses := map[string]string{
		"beacon/states/55/finality_checkpoints": finalizedJSON(3),
		"beacon/states/55/validators/64":        validatorJSON(39_997*shorPerQRL, 39_997*shorPerQRL),
		"beacon/states/55/validators":           fmt.Sprintf(`[{"validator":{"effective_balance":"%d"}},{"validator":{"effective_balance":"%d"}}]`, 64*40_000*shorPerQRL, 39_997*shorPerQRL),
	}
	fake, client := newFakeBeacon(t, responses)

	source, target, err := testSpare(client).expectedMissedVotePenalties(t.Context(), 5)
	require.NoError(t, err)
	require.Equal(t, uint64(351_408_642), source)
	require.Equal(t, uint64(652_616_050), target)
	for _, path := range fake.paths() {
		require.True(t, strings.HasPrefix(path, "beacon/states/55/"), "read %s", path)
	}
}

func TestExpectedMissedVotePenaltiesRefusesAnInactivityLeak(t *testing.T) {
	// Epoch 5 is four epochs past finalized epoch 1, the most without a leak.
	_, client := newFakeBeacon(t, map[string]string{
		"beacon/states/55/finality_checkpoints": finalizedJSON(1),
		"beacon/states/55/validators/64":        validatorJSON(40_000*shorPerQRL, 40_000*shorPerQRL),
		"beacon/states/55/validators":           fmt.Sprintf(`[{"validator":{"effective_balance":"%d"}}]`, 65*40_000*shorPerQRL),
	})
	_, _, err := testSpare(client).expectedMissedVotePenalties(t.Context(), 5)
	require.NoError(t, err)

	// Five epochs past finalized epoch 0 is a leak.
	_, client = newFakeBeacon(t, map[string]string{"beacon/states/55/finality_checkpoints": finalizedJSON(0)})
	_, _, err = testSpare(client).expectedMissedVotePenalties(t.Context(), 5)
	require.EqualError(t, err,
		"the rewards of epoch 5 are processed in an inactivity leak: the last finalized epoch is 0")
}

func TestBalanceAtEpochStartReadsTheFirstSlot(t *testing.T) {
	_, client := newFakeBeacon(t, map[string]string{
		"beacon/states/48/validators/64": validatorJSON(39_999*shorPerQRL, 39_999*shorPerQRL),
	})
	balance, err := testSpare(client).balanceAtEpochStart(t.Context(), 6)
	require.NoError(t, err)
	require.Equal(t, uint64(39_999*shorPerQRL), balance)
}

func TestBlockBalanceChangesCoverTheBlocksBetweenTwoEpochStarts(t *testing.T) {
	block := func(withdrawals string) string {
		return fmt.Sprintf(`{"message":{"body":{"execution_payload":{"withdrawals":[%s]}}}}`, withdrawals)
	}
	withdrawal := func(index, amount uint64) string {
		return fmt.Sprintf(`{"validator_index":"%d","amount":"%d"}`, index, amount)
	}
	syncReward := func(reward int64) string {
		return fmt.Sprintf(`[{"validator_index":"%d","reward":"%d"}]`, spareIndex, reward)
	}
	// Epoch 6 covers slots 49 through 56. Slots 48 and 57 belong to the
	// balances either side, slot 50 is empty, and at slot 52 the validator
	// is outside the sync committee.
	responses := map[string]string{
		"beacon/blocks/48":                 block(withdrawal(spareIndex, 1000)),
		"beacon/blocks/49":                 block(withdrawal(spareIndex, 7) + "," + withdrawal(3, 500)),
		"beacon/rewards/sync_committee/49": syncReward(-2),
		"beacon/blocks/51":                 block(""),
		"beacon/rewards/sync_committee/51": syncReward(5),
		"beacon/blocks/52":                 block(""),
		"beacon/rewards/sync_committee/52": `[]`,
		"beacon/blocks/56":                 block(""),
		"beacon/rewards/sync_committee/56": syncReward(-3),
		"beacon/blocks/57":                 block(withdrawal(spareIndex, 1000)),
	}
	for slot := 53; slot <= 55; slot++ {
		responses[fmt.Sprintf("beacon/blocks/%d", slot)] = block("")
		responses[fmt.Sprintf("beacon/rewards/sync_committee/%d", slot)] = `[]`
	}
	fake, client := newFakeBeacon(t, responses)

	change, err := testSpare(client).blockBalanceChanges(t.Context(), 6)
	require.NoError(t, err)
	require.Equal(t, int64(-7-2+5-3), change)
	require.NotContains(t, fake.paths(), "beacon/blocks/48")
	require.NotContains(t, fake.paths(), "beacon/blocks/57")
	require.NotContains(t, fake.paths(), "beacon/rewards/sync_committee/50")
}

func TestBlockBalanceChangesReportsAFailedRead(t *testing.T) {
	_, client := newFakeBeacon(t, map[string]string{
		"beacon/blocks/49": `{"message":{"body":{"execution_payload":{}}}}`,
	})
	_, err := testSpare(client).blockBalanceChanges(t.Context(), 6)
	require.ErrorContains(t, err, "sync_committee/49")
}
