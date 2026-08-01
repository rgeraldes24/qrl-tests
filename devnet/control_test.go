// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package devnet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceController(t *testing.T) {
	var calls [][]string
	controller := &ServiceController{
		enclave: "qrl-e2e",
		run: func(_ context.Context, arguments ...string) error {
			calls = append(calls, arguments)
			return nil
		},
	}

	require.NoError(t, controller.Restart(t.Context(), "el-2-gqrl-qrysm", "cl-2-qrysm-gqrl"))
	require.Equal(t, [][]string{
		{"service", "stop", "qrl-e2e", "el-2-gqrl-qrysm", "cl-2-qrysm-gqrl"},
		{"service", "start", "qrl-e2e", "el-2-gqrl-qrysm", "cl-2-qrysm-gqrl"},
	}, calls)
}
