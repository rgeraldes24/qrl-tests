package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"
)

// participantMetricsPort is the port of a participant's go-qrl metrics page.
const participantMetricsPort = 9001

// ParticipantContainers returns a devnet participant's execution container,
// with its metrics page, and its consensus and validator containers if it runs
// them.
func ParticipantContainers(ctx context.Context, docker ContainerReader, participant devnet.Participant) ([]Container, error) {
	execution := serviceContainer(participant.Execution.ServiceInfo)
	metrics, err := metricsURL(ctx, docker, execution, participantMetricsPort)
	if err != nil {
		return nil, err
	}

	containers := []Container{{Name: execution, MetricsURL: metrics}}
	for _, service := range []devnet.ServiceInfo{participant.Consensus.ServiceInfo, participant.Validator.ServiceInfo} {
		if service.Name != "" {
			containers = append(containers, Container{Name: serviceContainer(service)})
		}
	}
	return containers, nil
}

// serviceContainer returns the Docker name of a Kurtosis service's container.
func serviceContainer(service devnet.ServiceInfo) string {
	return service.Name + "--" + service.ID
}

// PublishedPort returns the host port that a container's TCP port is
// published to.
func PublishedPort(ctx context.Context, docker ContainerReader, container string, port uint16) (string, error) {
	inspected, err := docker.ContainerInspect(ctx, container, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", container, err)
	}
	published, _ := network.PortFrom(port, network.TCP)
	settings := inspected.Container.NetworkSettings
	if settings == nil || len(settings.Ports[published]) == 0 || settings.Ports[published][0].HostPort == "" {
		return "", fmt.Errorf("port %d of %s is not published", port, container)
	}
	return settings.Ports[published][0].HostPort, nil
}

// StatsReader is the part of the Docker client MemoryUsage uses.
type StatsReader interface {
	ContainerStats(context.Context, string, dockerclient.ContainerStatsOptions) (dockerclient.ContainerStatsResult, error)
}

// MemoryUsage returns the bytes of memory a container uses, as docker stats
// shows them: without the page cache of files it has not touched lately, which
// grows with the files it writes and the kernel can take back.
func MemoryUsage(ctx context.Context, docker StatsReader, container string) (uint64, error) {
	result, err := docker.ContainerStats(ctx, container, dockerclient.ContainerStatsOptions{})
	if err != nil {
		return 0, fmt.Errorf("read the stats of %s: %w", container, err)
	}
	defer result.Body.Close()
	var stats struct {
		Memory struct {
			Usage uint64 `json:"usage"`
			Stats struct {
				// cgroup v2, and cgroup v1.
				InactiveFile      uint64 `json:"inactive_file"`
				TotalInactiveFile uint64 `json:"total_inactive_file"`
			} `json:"stats"`
		} `json:"memory_stats"`
	}
	if err := json.NewDecoder(result.Body).Decode(&stats); err != nil {
		return 0, fmt.Errorf("decode the stats of %s: %w", container, err)
	}
	if stats.Memory.Usage == 0 {
		return 0, errors.New("the stats of " + container + " have no memory usage")
	}
	inactive := max(stats.Memory.Stats.InactiveFile, stats.Memory.Stats.TotalInactiveFile)
	return stats.Memory.Usage - min(inactive, stats.Memory.Usage), nil
}
