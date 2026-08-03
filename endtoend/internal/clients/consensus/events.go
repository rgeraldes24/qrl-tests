package consensus

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Event struct {
	Topic string
	Data  json.RawMessage
}

func (client *Client) Events(ctx context.Context, topics ...string) (<-chan Event, <-chan error, error) {
	endpoint := client.baseURL.ResolveReference(&url.URL{Path: "/qrl/v1/events"})
	query := endpoint.Query()
	for _, topic := range topics {
		query.Add("topics", topic)
	}
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", "text/event-stream")
	events := make(chan Event)
	errors := make(chan error, 1)
	go func() {
		defer close(events)
		defer close(errors)

		response, err := client.http.Do(request)
		if err != nil {
			if ctx.Err() == nil {
				errors <- err
			}
			return
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			errors <- fmt.Errorf("GET /qrl/v1/events returned %s", response.Status)
			return
		}

		scanner := bufio.NewScanner(response.Body)
		var topic string
		var data strings.Builder
		dispatch := func() bool {
			if topic == "" || data.Len() == 0 {
				topic = ""
				data.Reset()
				return true
			}
			event := Event{Topic: topic, Data: json.RawMessage(data.String())}
			topic = ""
			data.Reset()
			select {
			case events <- event:
				return true
			case <-ctx.Done():
				return false
			}
		}
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if !dispatch() {
					return
				}
				continue
			}
			field, value, found := strings.Cut(line, ":")
			if !found {
				continue
			}
			value = strings.TrimPrefix(value, " ")
			switch field {
			case "event":
				topic = value
			case "data":
				if data.Len() > 0 {
					data.WriteByte('\n')
				}
				data.WriteString(value)
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			errors <- err
		}
	}()
	return events, errors, nil
}
