// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package consensus

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

func (client *Client) ValidatorAssignments(ctx context.Context, epoch uint64) ([]ValidatorAssignment, error) {
	var response struct {
		Assignments   []ValidatorAssignment `json:"assignments"`
		NextPageToken string                `json:"nextPageToken"`
		TotalSize     int                   `json:"totalSize"`
	}
	path := "/qrl/v1alpha1/validators/assignments?epoch=" + strconv.FormatUint(epoch, 10) + "&page_size=250"
	if err := client.get(ctx, path, &response); err != nil {
		return nil, err
	}
	if response.TotalSize != 0 && response.TotalSize != len(response.Assignments) {
		return nil, fmt.Errorf("validator assignments returned %d of %d entries", len(response.Assignments), response.TotalSize)
	}
	if response.NextPageToken != "" {
		return nil, errors.New("validator assignments exceed one response page")
	}
	return response.Assignments, nil
}
