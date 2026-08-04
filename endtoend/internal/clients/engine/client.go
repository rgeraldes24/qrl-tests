// Package engine provides authenticated Engine JSON-RPC operations for live suites.
package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	protocolengine "github.com/theQRL/go-qrl/beacon/engine"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/go-qrl/node"
	"github.com/theQRL/go-qrl/rpc"
)

type Client struct {
	rpc *rpc.Client
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

func New(ctx context.Context, endpoint, hexSecret string) (*Client, error) {
	secretBytes, err := hex.DecodeString(strings.TrimPrefix(hexSecret, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode Engine JWT secret: %w", err)
	}
	if len(secretBytes) != sha256.Size {
		return nil, fmt.Errorf("Engine JWT secret is %d bytes, want %d", len(secretBytes), sha256.Size)
	}
	var secret [sha256.Size]byte
	copy(secret[:], secretBytes)
	client, err := rpc.DialOptions(ctx, endpoint, rpc.WithHTTPAuth(node.NewJWTAuth(secret)))
	if err != nil {
		return nil, fmt.Errorf("connect to Engine RPC: %w", err)
	}
	return &Client{rpc: client}, nil
}

func (client *Client) Close() {
	client.rpc.Close()
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

func (client *Client) PayloadBodiesByRange(ctx context.Context, start, count uint64) ([]*PayloadBody, error) {
	var result []*PayloadBody
	err := client.Call(
		ctx,
		&result,
		"engine_getPayloadBodiesByRangeV1",
		hexutil.Uint64(start),
		hexutil.Uint64(count),
	)
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
	return client.rpc.CallContext(ctx, result, method, params...)
}
