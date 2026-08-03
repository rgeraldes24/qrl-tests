package consensus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (client *Client) Post(ctx context.Context, path string, payload any) error {
	return client.PostJSON(ctx, path, payload, nil)
}

func (client *Client) GetJSON(ctx context.Context, path string, result any) error {
	return client.get(ctx, path, result)
}

func (client *Client) PostJSON(ctx context.Context, path string, payload, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return client.do(ctx, http.MethodPost, path, bytes.NewReader(body), result)
}

func (client *Client) get(ctx context.Context, path string, result any) error {
	return client.do(ctx, http.MethodGet, path, nil, result)
}

func (client *Client) do(ctx context.Context, method, path string, body io.Reader, result any) error {
	reference, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("parse consensus path %q: %w", path, err)
	}
	endpoint := client.baseURL.ResolveReference(reference)
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return &responseError{
			method: method, path: path, status: response.Status,
			statusCode: response.StatusCode, body: strings.TrimSpace(string(body)),
		}
	}
	if result == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(result); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}
