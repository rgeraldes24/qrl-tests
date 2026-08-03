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
	var response dataResponse[struct {
		HeadSlot   string `json:"head_slot"`
		Syncing    bool   `json:"is_syncing"`
		Optimistic bool   `json:"is_optimistic"`
		ELOffline  bool   `json:"el_offline"`
	}]
	if err := client.get(ctx, "/qrl/v1/node/syncing", &response); err != nil {
		return SyncStatus{}, err
	}
	headSlot, err := decimal("head slot", response.Data.HeadSlot)
	if err != nil {
		return SyncStatus{}, err
	}
	return SyncStatus{
		HeadSlot:   headSlot,
		Syncing:    response.Data.Syncing,
		Optimistic: response.Data.Optimistic,
		ELOffline:  response.Data.ELOffline,
	}, nil
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
					Slot string `json:"slot"`
				} `json:"message"`
			} `json:"header"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/headers/"+url.PathEscape(blockID), &response); err != nil {
		return Head{}, err
	}
	slot, err := decimal("head slot", response.Data.Header.Message.Slot)
	if err != nil {
		return Head{}, err
	}
	return Head{Slot: slot, Root: response.Data.Root}, nil
}

func (client *Client) ValidatorParticipation(ctx context.Context) (ValidatorParticipation, error) {
	var response struct {
		Participation struct {
			PreviousActive string `json:"previousEpochActiveShor"`
			PreviousTarget string `json:"previousEpochTargetAttestingShor"`
			PreviousHead   string `json:"previousEpochHeadAttestingShor"`
		} `json:"participation"`
	}
	if err := client.get(ctx, "/qrl/v1alpha1/validators/participation", &response); err != nil {
		return ValidatorParticipation{}, err
	}
	active, err := decimal("previous active balance", response.Participation.PreviousActive)
	if err != nil {
		return ValidatorParticipation{}, err
	}
	target, err := decimal("previous target-attesting balance", response.Participation.PreviousTarget)
	if err != nil {
		return ValidatorParticipation{}, err
	}
	head, err := decimal("previous head-attesting balance", response.Participation.PreviousHead)
	if err != nil {
		return ValidatorParticipation{}, err
	}
	return ValidatorParticipation{PreviousActive: active, PreviousTarget: target, PreviousHead: head}, nil
}

func (client *Client) FinalizedEpoch(ctx context.Context) (uint64, error) {
	checkpoint, err := client.FinalizedCheckpoint(ctx)
	return checkpoint.Epoch, err
}

func (client *Client) FinalizedCheckpoint(ctx context.Context) (Checkpoint, error) {
	var response struct {
		Data struct {
			Finalized struct {
				Epoch string `json:"epoch"`
				Root  string `json:"root"`
			} `json:"finalized"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/finality_checkpoints", &response); err != nil {
		return Checkpoint{}, err
	}
	epoch, err := decimal("finalized epoch", response.Data.Finalized.Epoch)
	if err != nil {
		return Checkpoint{}, err
	}
	return Checkpoint{Epoch: epoch, Root: response.Data.Finalized.Root}, nil
}

func (client *Client) Genesis(ctx context.Context) (Genesis, error) {
	var response struct {
		Data struct {
			Time           string `json:"genesis_time"`
			ValidatorsRoot string `json:"genesis_validators_root"`
			ForkVersion    string `json:"genesis_fork_version"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/genesis", &response); err != nil {
		return Genesis{}, err
	}
	genesisTime, err := decimal("genesis time", response.Data.Time)
	if err != nil {
		return Genesis{}, err
	}
	return Genesis{genesisTime, response.Data.ValidatorsRoot, response.Data.ForkVersion}, nil
}

func (client *Client) Fork(ctx context.Context) (Fork, error) {
	var response struct {
		Data struct {
			PreviousVersion string `json:"previous_version"`
			CurrentVersion  string `json:"current_version"`
			Epoch           string `json:"epoch"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/fork", &response); err != nil {
		return Fork{}, err
	}
	epoch, err := decimal("fork epoch", response.Data.Epoch)
	if err != nil {
		return Fork{}, err
	}
	return Fork{response.Data.PreviousVersion, response.Data.CurrentVersion, epoch}, nil
}
