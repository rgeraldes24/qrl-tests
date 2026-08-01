// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package devnet

import (
	"testing"

	"github.com/cyyber/qrl-tests/devnet/internal/kurtosis"
	"github.com/stretchr/testify/require"
)

func TestParticipantsFromServices(t *testing.T) {
	services := map[string]kurtosis.Service{
		"cl-2-qrysm-gqrl": service("cl-2-qrysm-gqrl", "beacon", 4202, 0, 0, 0),
		"el-2-gqrl-qrysm": service("el-2-gqrl-qrysm", "execution", 3202, 3302, 3402, 0),
		"vc-2-gqrl-qrysm": service("vc-2-gqrl-qrysm", "validator", 0, 0, 0, 5202),
		"cl-1-qrysm-gqrl": service("cl-1-qrysm-gqrl", "beacon", 4201, 0, 0, 0),
		"el-1-gqrl-qrysm": service("el-1-gqrl-qrysm", "execution", 3201, 3301, 3401, 0),
		"vc-1-gqrl-qrysm": service("vc-1-gqrl-qrysm", "validator", 0, 0, 0, 5201),
		"prometheus":      {Name: "prometheus", Labels: map[string]string{"qrl-package.client-type": "utility"}},
	}

	participants, err := participantsFromServices(services)
	require.NoError(t, err)
	require.Equal(t, []Participant{
		{
			Index:                1,
			ExecutionServiceName: "el-1-gqrl-qrysm", ExecutionServiceID: "el-1-gqrl-qrysm-id", ExecutionPrivateIP: "10.0.0.1",
			ConsensusServiceName: "cl-1-qrysm-gqrl", ConsensusServiceID: "cl-1-qrysm-gqrl-id", ConsensusPrivateIP: "10.0.0.1",
			ValidatorServiceName: "vc-1-gqrl-qrysm", ValidatorServiceID: "vc-1-gqrl-qrysm-id",
			RPCURL: "http://127.0.0.1:3201", GraphQLURL: "http://127.0.0.1:3201/graphql", WebSocketURL: "ws://127.0.0.1:3301",
			EngineURL: "http://127.0.0.1:3401", ConsensusURL: "http://127.0.0.1:4201", ValidatorURL: "http://127.0.0.1:5201",
		},
		{
			Index:                2,
			ExecutionServiceName: "el-2-gqrl-qrysm", ExecutionServiceID: "el-2-gqrl-qrysm-id", ExecutionPrivateIP: "10.0.0.2",
			ConsensusServiceName: "cl-2-qrysm-gqrl", ConsensusServiceID: "cl-2-qrysm-gqrl-id", ConsensusPrivateIP: "10.0.0.2",
			ValidatorServiceName: "vc-2-gqrl-qrysm", ValidatorServiceID: "vc-2-gqrl-qrysm-id",
			RPCURL: "http://127.0.0.1:3202", GraphQLURL: "http://127.0.0.1:3202/graphql", WebSocketURL: "ws://127.0.0.1:3302",
			EngineURL: "http://127.0.0.1:3402", ConsensusURL: "http://127.0.0.1:4202", ValidatorURL: "http://127.0.0.1:5202",
		},
	}, participants)
}

func service(name, clientType string, rpc, ws, engine, validator uint16) kurtosis.Service {
	ports := map[string]uint16{}
	for id, port := range map[string]uint16{
		"rpc": rpc, "ws": ws, "engine-rpc": engine, "http": rpc, "http-validator": validator,
	} {
		if port != 0 {
			ports[id] = port
		}
	}
	return kurtosis.Service{
		Name: name, UUID: name + "-id", PrivateIP: "10.0.0." + name[3:4], PublicIP: "127.0.0.1", PublicPorts: ports,
		Labels: map[string]string{"qrl-package.client-type": clientType},
	}
}
