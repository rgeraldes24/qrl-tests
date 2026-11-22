package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadMetrics(t *testing.T) {
	serve := func(t *testing.T, status int, page string) string {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(status)
			_, _ = writer.Write([]byte(page))
		}))
		t.Cleanup(server.Close)
		return server.URL
	}

	t.Run("skips comments, labelled and bad values", func(t *testing.T) {
		page := "# TYPE chain_head gauge\nchain_head 7\nchain_inserts {quantile=\"0.5\"} 9\nnot_a_number x\n"
		values, err := readMetrics(t.Context(), serve(t, http.StatusOK, page))
		require.NoError(t, err)
		require.Equal(t, map[string]float64{"chain_head": 7}, values)
	})
	t.Run("an empty page", func(t *testing.T) {
		_, err := readMetrics(t.Context(), serve(t, http.StatusOK, "# nothing\n"))
		require.ErrorContains(t, err, "has no values")
	})
	t.Run("a failed request", func(t *testing.T) {
		_, err := readMetrics(t.Context(), serve(t, http.StatusServiceUnavailable, ""))
		require.ErrorContains(t, err, "503")
	})
}

func TestGaugeAndSummaryMean(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("txpool_slots 6144\nfast_sum 120\nfast_count 5\nempty_sum 0\nempty_count 0\n"))
	}))
	defer server.Close()

	slots, err := Gauge(t.Context(), server.URL, "txpool_slots")
	require.NoError(t, err)
	require.InDelta(t, 6144.0, slots, 1e-9)
	_, err = Gauge(t.Context(), server.URL, "missing")
	require.ErrorContains(t, err, "no missing")

	mean, err := SummaryMean(t.Context(), server.URL, "fast")
	require.NoError(t, err)
	require.InDelta(t, 24.0, mean, 1e-9)
	mean, err = SummaryMean(t.Context(), server.URL, "empty")
	require.NoError(t, err)
	require.Zero(t, mean)
}
