// Package consensus provides the beacon REST operations used by live suites.
package consensus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Client struct {
	baseURL *url.URL
	http    *http.Client
}

type SyncStatus struct {
	HeadSlot   uint64
	Syncing    bool
	Optimistic bool
	ELOffline  bool
}

func New(endpoint string) (*Client, error) {
	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse consensus endpoint: %w", err)
	}
	return &Client{baseURL: baseURL, http: http.DefaultClient}, nil
}

func (client *Client) Health(ctx context.Context) error {
	return client.get(ctx, "/qrl/v1/node/health", nil)
}

func (client *Client) Syncing(ctx context.Context) (SyncStatus, error) {
	var response struct {
		Data struct {
			HeadSlot   string `json:"head_slot"`
			Syncing    bool   `json:"is_syncing"`
			Optimistic bool   `json:"is_optimistic"`
			ELOffline  bool   `json:"el_offline"`
		} `json:"data"`
	}
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
	var response struct {
		Data struct {
			Header struct {
				Message struct {
					Slot string `json:"slot"`
				} `json:"message"`
			} `json:"header"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/headers/head", &response); err != nil {
		return 0, err
	}
	return decimal("head slot", response.Data.Header.Message.Slot)
}

func (client *Client) FinalizedEpoch(ctx context.Context) (uint64, error) {
	var response struct {
		Data struct {
			Finalized struct {
				Epoch string `json:"epoch"`
			} `json:"finalized"`
		} `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/finality_checkpoints", &response); err != nil {
		return 0, err
	}
	return decimal("finalized epoch", response.Data.Finalized.Epoch)
}

func (client *Client) ActiveValidatorCount(ctx context.Context) (int, error) {
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/states/head/validators?status=active", &response); err != nil {
		return 0, err
	}
	return len(response.Data), nil
}

func (client *Client) BlockAttestationCount(ctx context.Context, blockID string) (int, error) {
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := client.get(ctx, "/qrl/v1/beacon/blocks/"+url.PathEscape(blockID)+"/attestations", &response); err != nil {
		return 0, err
	}
	return len(response.Data), nil
}

func (client *Client) get(ctx context.Context, path string, result any) error {
	reference, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("parse consensus path %q: %w", path, err)
	}
	endpoint := client.baseURL.ResolveReference(reference)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("GET %s returned %s: %s", path, response.Status, strings.TrimSpace(string(body)))
	}
	if result == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(result); err != nil {
		return fmt.Errorf("decode GET %s: %w", path, err)
	}
	return nil
}

func decimal(name, value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return parsed, nil
}
