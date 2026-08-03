package rpcjson

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Equal(t, "qrl_getBalance", payload.Method)
		require.Equal(t, []any{"Q123", "latest"}, payload.Params)
		fmt.Fprint(writer, `{"jsonrpc":"2.0","id":1,"result":"0x2a"}`)
	}))
	defer server.Close()

	var result string
	require.NoError(t, Call(
		context.Background(),
		server.URL,
		"qrl_getBalance",
		[]any{"Q123", "latest"},
		&result,
	))
	require.Equal(t, "0x2a", result)
}

func TestCallRPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(writer, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`)
	}))
	defer server.Close()

	err := Call(context.Background(), server.URL, "missing", nil, nil)
	require.EqualError(t, err, "missing RPC error -32601: method not found")
}
