package consensus

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, []string{"head", "block"}, request.URL.Query()["topics"])
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(writer, "event: head\ndata: {\"slot\":\"12\"}\n\n")
	}))
	defer server.Close()

	client, err := New(server.URL)
	require.NoError(t, err)
	events, failures, err := client.Events(context.Background(), "head", "block")
	require.NoError(t, err)
	require.Equal(t, Event{Topic: "head", Data: []byte(`{"slot":"12"}`)}, <-events)
	require.Empty(t, <-failures)
}

func TestEventsReturnsBeforeFirstEvent(t *testing.T) {
	requestStarted := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-release
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(writer, "event: chain_reorg\ndata: {}\n\n")
	}))
	defer server.Close()
	defer close(release)

	client, err := New(server.URL)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	returned := make(chan error, 1)
	go func() {
		_, _, err := client.Events(ctx, "chain_reorg")
		returned <- err
	}()
	<-requestStarted
	select {
	case err := <-returned:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Events waited for the first server response")
	}
}
