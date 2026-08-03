// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensus

import (
	"context"
	"net/url"
)

func (client *Client) Health(ctx context.Context) error {
	return client.get(ctx, "/qrl/v1/node/health", nil)
}

func (client *Client) Syncing(ctx context.Context) (SyncStatus, error) {
	var response dataResponse[SyncStatus]
	if err := client.get(ctx, "/qrl/v1/node/syncing", &response); err != nil {
		return SyncStatus{}, err
	}
	return response.Data, nil
}

func (client *Client) HeadSlot(ctx context.Context) (uint64, error) {
	head, err := client.Head(ctx)
	return head.Slot, err
}

func (client *Client) Head(ctx context.Context) (Head, error) {
	return client.Header(ctx, "head")
}

func (client *Client) Header(ctx context.Context, blockID string) (Head, error) {
	var response struct {
		Data struct {
			Root   string `json:"root"`
			Header struct {
				Message struct {
					Slot uint64 `json:"slot,string"`
				} `json:"message"`
			} `json:"header"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/headers/"+url.PathEscape(blockID), &response); err != nil {
		return Head{}, err
	}
	return Head{Slot: response.Data.Header.Message.Slot, Root: response.Data.Root}, nil
}

func (client *Client) ValidatorParticipation(ctx context.Context) (ValidatorParticipation, error) {
	var response struct {
		Participation ValidatorParticipation `json:"participation"`
	}
	if err := client.get(ctx, "/qrl/v1alpha1/validators/participation", &response); err != nil {
		return ValidatorParticipation{}, err
	}
	return response.Participation, nil
}

func (client *Client) FinalizedEpoch(ctx context.Context) (uint64, error) {
	checkpoint, err := client.FinalizedCheckpoint(ctx)
	return checkpoint.Epoch, err
}

func (client *Client) FinalizedCheckpoint(ctx context.Context) (Checkpoint, error) {
	var response dataResponse[struct {
		Finalized Checkpoint `json:"finalized"`
	}]
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/finality_checkpoints", &response); err != nil {
		return Checkpoint{}, err
	}
	return response.Data.Finalized, nil
}

func (client *Client) Genesis(ctx context.Context) (Genesis, error) {
	var response dataResponse[Genesis]
	if err := client.get(ctx, "/qrl/v1/beacon/genesis", &response); err != nil {
		return Genesis{}, err
	}
	return response.Data, nil
}

func (client *Client) Fork(ctx context.Context) (Fork, error) {
	var response dataResponse[Fork]
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/fork", &response); err != nil {
		return Fork{}, err
	}
	return response.Data, nil
}
