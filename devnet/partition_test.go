// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package devnet

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetworkPartition(t *testing.T) {
	var calls [][]string
	partition := &dockerNetworkPartition{
		run: func(_ context.Context, arguments ...string) (string, error) {
			calls = append(calls, arguments)
			if arguments[0] == "ps" {
				filter := arguments[len(arguments)-1]
				return strings.TrimPrefix(filter, "label=com.kurtosistech.guid=") + "-container\n", nil
			}
			return "", nil
		},
	}
	first := Participant{
		Execution: ExecutionService{ServiceInfo: ServiceInfo{ID: "el-1", PrivateIP: "10.0.0.1"}},
		Consensus: ConsensusService{ServiceInfo: ServiceInfo{ID: "cl-1", PrivateIP: "10.0.0.2"}},
	}
	second := Participant{
		Execution: ExecutionService{ServiceInfo: ServiceInfo{ID: "el-2", PrivateIP: "10.0.0.3"}},
		Consensus: ConsensusService{ServiceInfo: ServiceInfo{ID: "cl-2", PrivateIP: "10.0.0.4"}},
	}

	require.NoError(t, partition.Apply(t.Context(), []Participant{first}, []Participant{second}))
	require.Len(t, partition.rules, 4)
	require.NoError(t, partition.Clear(t.Context()))
	require.Empty(t, partition.rules)

	var inserts, deletes int
	for _, call := range calls {
		joined := strings.Join(call, " ")
		if strings.Contains(joined, "iptables -I OUTPUT") {
			inserts++
		}
		if strings.Contains(joined, "iptables -D OUTPUT") {
			deletes++
		}
	}
	require.Equal(t, 4, inserts)
	require.Equal(t, inserts, deletes)
}

func TestKubernetesNetworkPartitionUnsupported(t *testing.T) {
	partition, err := NewNetworkPartition(BackendKubernetes)
	require.ErrorIs(t, err, ErrNetworkPartitionUnsupported)
	require.Nil(t, partition)
}
