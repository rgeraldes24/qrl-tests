// Package beacon provides the beacon REST operations used by live suites.
package beacon

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

type Client struct {
	baseURL *url.URL
	http    *http.Client
}

type responseError struct {
	method     string
	path       string
	status     string
	statusCode int
	body       string
}

func (err *responseError) Error() string {
	return fmt.Sprintf("%s %s returned %s: %s", err.method, err.path, err.status, err.body)
}

func IsNotFound(err error) bool {
	var responseErr *responseError
	return errors.As(err, &responseErr) && responseErr.statusCode == http.StatusNotFound
}

func New(endpoint string) (*Client, error) {
	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse consensus endpoint: %w", err)
	}
	return &Client{baseURL: baseURL, http: http.DefaultClient}, nil
}
