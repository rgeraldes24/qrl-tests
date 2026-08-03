package rpcjson

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type response struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Call invokes a JSON-RPC method and decodes its result.
func Call(ctx context.Context, endpoint, method string, params []any, result any) error {
	if endpoint == "" {
		return fmt.Errorf("execution RPC endpoint is required")
	}
	if params == nil {
		params = []any{}
	}
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	httpResponse, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(httpResponse.Body, 4096))
		return fmt.Errorf("%s returned %s: %s", method, httpResponse.Status, strings.TrimSpace(string(body)))
	}
	var envelope response
	if err := json.NewDecoder(httpResponse.Body).Decode(&envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("%s RPC error %d: %s", method, envelope.Error.Code, envelope.Error.Message)
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("decode %s result: %w", method, err)
	}
	return nil
}
