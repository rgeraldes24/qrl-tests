// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensus

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

func (client *Client) Block(ctx context.Context, blockID string) (SignedBlock, error) {
	return getData[SignedBlock](ctx, client, "/qrl/v1/beacon/blocks/"+url.PathEscape(blockID))
}

func (client *Client) BlockHeader(ctx context.Context, blockID string) (BlockHeader, error) {
	return getData[BlockHeader](ctx, client, "/qrl/v1/beacon/headers/"+url.PathEscape(blockID))
}

func (client *Client) BlockGraffitiText(ctx context.Context, blockID string) (string, error) {
	block, err := client.Block(ctx, blockID)
	if err != nil {
		return "", err
	}
	graffiti, err := hex.DecodeString(strings.TrimPrefix(block.Message.Body.Graffiti, "0x"))
	if err != nil {
		return "", fmt.Errorf("decode block graffiti: %w", err)
	}
	return strings.TrimRight(string(graffiti), "\x00"), nil
}

func (client *Client) BlockExecutionPayload(ctx context.Context, blockID string) (ExecutionPayload, error) {
	block, err := client.Block(ctx, blockID)
	if err != nil {
		return ExecutionPayload{}, err
	}
	return block.Message.Body.ExecutionPayload, nil
}

func (client *Client) BlockOperations(ctx context.Context, blockID string) (BlockOperations, error) {
	block, err := client.Block(ctx, blockID)
	if err != nil {
		return BlockOperations{}, err
	}
	return block.Operations(), nil
}

func (block SignedBlock) Operations() BlockOperations {
	body := block.Message.Body
	result := BlockOperations{}
	for _, item := range body.Deposits {
		result.Deposits = append(result.Deposits, item.Data)
	}
	for _, item := range body.VoluntaryExits {
		result.VoluntaryExits = append(result.VoluntaryExits, item.Message.ValidatorIndex)
	}
	for _, item := range body.ProposerSlashings {
		result.ProposerSlashings = append(result.ProposerSlashings, item.Header1.Message.ProposerIndex)
	}
	for _, item := range body.AttesterSlashings {
		result.AttesterSlashings = append(result.AttesterSlashings, item.Attestation1.AttestingIndices...)
	}
	result.Withdrawals = append(result.Withdrawals, body.ExecutionPayload.Withdrawals...)
	return result
}
