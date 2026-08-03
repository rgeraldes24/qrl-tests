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
	path := fmt.Sprintf(
		"/qrl/v1/beacon/states/%s/committees?slot=%d&index=%d",
		url.PathEscape(stateID),
		slot,
		index,
	)
	committees, err := getData[[]committeeWire](ctx, client, path)
	if err != nil {
		return nil, err
	}
	if len(committees) != 1 {
		return nil, fmt.Errorf("expected one committee, got %d", len(committees))
	}
	return committees[0].Validators, nil
}

func (client *Client) SyncCommittee(ctx context.Context, stateID string) ([]uint64, error) {
	path := "/qrl/v1/beacon/states/" + url.PathEscape(stateID) + "/sync_committees"
	committee, err := getData[committeeWire](ctx, client, path)
	if err != nil {
		return nil, err
	}
	return committee.Validators, nil
}
