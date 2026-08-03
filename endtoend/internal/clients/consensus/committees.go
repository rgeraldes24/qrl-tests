// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensus

import (
	"context"
	"fmt"
	"net/url"
)

type committeeWire struct {
	Validators quotedUint64s `json:"validators"`
}

func (client *Client) Committee(ctx context.Context, stateID string, slot, index uint64) ([]uint64, error) {
	var response dataResponse[[]committeeWire]
	path := fmt.Sprintf(
		"/qrl/v1/beacon/states/%s/committees?slot=%d&index=%d",
		url.PathEscape(stateID),
		slot,
		index,
	)
	if err := client.get(ctx, path, &response); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 {
		return nil, fmt.Errorf("expected one committee, got %d", len(response.Data))
	}
	return response.Data[0].Validators, nil
}

func (client *Client) SyncCommittee(ctx context.Context, stateID string) ([]uint64, error) {
	var response dataResponse[committeeWire]
	path := "/qrl/v1/beacon/states/" + url.PathEscape(stateID) + "/sync_committees"
	if err := client.get(ctx, path, &response); err != nil {
		return nil, err
	}
	return response.Data.Validators, nil
}
