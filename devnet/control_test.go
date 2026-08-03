// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package devnet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type serviceClient struct {
	startClient
	calls [][]string
}

func (client *serviceClient) StartServices(_ context.Context, enclave string, services ...string) error {
	client.calls = append(client.calls, append([]string{"start", enclave}, services...))
	return nil
}

func (client *serviceClient) StopServices(_ context.Context, enclave string, services ...string) error {
	client.calls = append(client.calls, append([]string{"stop", enclave}, services...))
	return nil
}

func TestServiceController(t *testing.T) {
	client := new(serviceClient)
	manager := &Manager{
		newClient: func() (kurtosisClient, error) { return client, nil },
	}
	controller := manager.ServiceController("qrl-e2e")

	require.NoError(t, controller.Restart(t.Context(), "el-2-gqrl-qrysm", "cl-2-qrysm-gqrl"))
	require.Equal(t, [][]string{
		{"stop", "qrl-e2e", "el-2-gqrl-qrysm", "cl-2-qrysm-gqrl"},
		{"start", "qrl-e2e", "el-2-gqrl-qrysm", "cl-2-qrysm-gqrl"},
	}, client.calls)
}
