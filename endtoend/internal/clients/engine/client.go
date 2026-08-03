// Package engine provides authenticated Engine JSON-RPC operations for live suites.
package engine

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	protocolengine "github.com/theQRL/go-qrl/beacon/engine"
)

type Client struct {
	endpoint string
	secret   []byte
	http     *http.Client
	nextID   atomic.Uint64
}

type PayloadBody struct {
	Transactions []string     `json:"transactions"`
	Withdrawals  []Withdrawal `json:"withdrawals"`
}

type Withdrawal struct {
	Index          string `json:"index"`
	ValidatorIndex string `json:"validatorIndex"`
	Address        string `json:"address"`
	Amount         string `json:"amount"`
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (err *rpcError) Error() string {
	return fmt.Sprintf("engine RPC error %d: %s", err.Code, err.Message)
}

func New(endpoint, hexSecret string) (*Client, error) {
	secret, err := hex.DecodeString(strings.TrimPrefix(hexSecret, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode Engine JWT secret: %w", err)
	}
	if len(secret) != sha256.Size {
		return nil, fmt.Errorf("Engine JWT secret is %d bytes, want %d", len(secret), sha256.Size)
	}
	return &Client{endpoint: endpoint, secret: secret, http: http.DefaultClient}, nil
}

func (client *Client) ExchangeCapabilities(ctx context.Context) ([]string, error) {
	var result []string
	err := client.Call(ctx, &result, "engine_exchangeCapabilities", []string{})
	return result, err
}

func (client *Client) PayloadBodiesByHash(ctx context.Context, hashes []string) ([]*PayloadBody, error) {
	var result []*PayloadBody
	err := client.Call(ctx, &result, "engine_getPayloadBodiesByHashV1", hashes)
	return result, err
}

func (client *Client) ForkchoiceUpdatedV2(
	ctx context.Context,
	state protocolengine.ForkchoiceStateV1,
	attributes *protocolengine.PayloadAttributes,
) (protocolengine.ForkChoiceResponse, error) {
	var result protocolengine.ForkChoiceResponse
	err := client.Call(ctx, &result, "engine_forkchoiceUpdatedV2", state, attributes)
	return result, err
}

func (client *Client) GetPayloadV2(ctx context.Context, id protocolengine.PayloadID) (*protocolengine.ExecutionPayloadEnvelope, error) {
	var result protocolengine.ExecutionPayloadEnvelope
	err := client.Call(ctx, &result, "engine_getPayloadV2", id)
	return &result, err
}

func (client *Client) NewPayloadV2(ctx context.Context, payload protocolengine.ExecutableData) (protocolengine.PayloadStatusV1, error) {
	var result protocolengine.PayloadStatusV1
	err := client.Call(ctx, &result, "engine_newPayloadV2", payload)
	return result, err
}

func (client *Client) Call(ctx context.Context, result any, method string, params ...any) error {
	payload, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      client.nextID.Add(1),
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if len(client.secret) > 0 {
		request.Header.Set("Authorization", "Bearer "+client.token(time.Now()))
	}
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("Engine RPC returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var envelope rpcResponse
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode Engine RPC response: %w", err)
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("decode Engine RPC result: %w", err)
	}
	return nil
}

func (client *Client) token(now time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"iat":%d}`, now.Unix())))
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, client.secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
