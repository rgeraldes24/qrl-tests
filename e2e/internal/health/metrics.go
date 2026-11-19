package health

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// metricsURL returns the host URL of a container's go-qrl metrics page.
func metricsURL(ctx context.Context, docker ContainerReader, container string, port uint16) (string, error) {
	hostPort, err := PublishedPort(ctx, docker, container, port)
	if err != nil {
		return "", err
	}
	return "http://127.0.0.1:" + hostPort + "/debug/metrics/prometheus", nil
}

// readMetrics returns the unlabelled values of a Prometheus metrics page, by
// name.
func readMetrics(ctx context.Context, url string) (map[string]float64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %s", url, response.Status)
	}

	values := make(map[string]float64)
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		// go-qrl writes a labelled value as "name {labels} value", so the
		// value after the first space has another space in it.
		name, value, found := strings.Cut(scanner.Text(), " ")
		if !found || strings.HasPrefix(name, "#") || strings.Contains(value, " ") {
			continue
		}
		if number, err := strconv.ParseFloat(value, 64); err == nil {
			values[name] = number
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(values) == 0 {
		return nil, errors.New("the metrics page has no values")
	}
	return values, nil
}

// Gauge reads an unlabelled value, such as "txpool_slots", from a metrics
// page in the Prometheus text format.
func Gauge(ctx context.Context, metricsURL, name string) (float64, error) {
	values, err := readMetrics(ctx, metricsURL)
	if err != nil {
		return 0, err
	}
	value, found := values[name]
	if !found {
		return 0, fmt.Errorf("the metrics have no %s", name)
	}
	return value, nil
}

// SummaryMean returns the mean observation of a Prometheus summary or
// histogram, or zero for one without observations.
func SummaryMean(ctx context.Context, metricsURL, name string) (float64, error) {
	values, err := readMetrics(ctx, metricsURL)
	if err != nil {
		return 0, err
	}
	count, found := values[name+"_count"]
	if !found {
		return 0, fmt.Errorf("the metrics have no %s", name)
	}
	if count == 0 {
		return 0, nil
	}
	return values[name+"_sum"] / count, nil
}
