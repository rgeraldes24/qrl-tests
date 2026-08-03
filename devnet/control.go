// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package devnet

import (
	"context"
	"fmt"
	"strings"
)

type ServiceController struct {
	manager *Manager
	enclave string
}

func (manager *Manager) ServiceController(enclave string) *ServiceController {
	return &ServiceController{manager: manager, enclave: enclave}
}

func (controller *ServiceController) Stop(ctx context.Context, services ...string) error {
	client, err := controller.client("stop", services)
	if err != nil {
		return err
	}
	if err := client.StopServices(ctx, controller.enclave, services...); err != nil {
		return fmt.Errorf("kurtosis service stop: %w", err)
	}
	return nil
}

func (controller *ServiceController) Start(ctx context.Context, services ...string) error {
	client, err := controller.client("start", services)
	if err != nil {
		return err
	}
	if err := client.StartServices(ctx, controller.enclave, services...); err != nil {
		return fmt.Errorf("kurtosis service start: %w", err)
	}
	return nil
}

func (controller *ServiceController) Restart(ctx context.Context, services ...string) error {
	client, err := controller.client("restart", services)
	if err != nil {
		return err
	}
	if err := client.StopServices(ctx, controller.enclave, services...); err != nil {
		return fmt.Errorf("kurtosis service stop: %w", err)
	}
	if err := client.StartServices(ctx, controller.enclave, services...); err != nil {
		return fmt.Errorf("kurtosis service start: %w", err)
	}
	return nil
}

func (controller *ServiceController) client(action string, services []string) (kurtosisClient, error) {
	if controller.manager == nil {
		return nil, fmt.Errorf("%s service: network manager is nil", action)
	}
	if strings.TrimSpace(controller.enclave) == "" {
		return nil, fmt.Errorf("%s service: enclave name is empty", action)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("%s service: no service names supplied", action)
	}
	client, err := controller.manager.newClient()
	if err != nil {
		return nil, err
	}
	return client, nil
}
