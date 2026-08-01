// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package devnet

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type ServiceController struct {
	enclave string
	run     func(context.Context, ...string) error
}

func NewServiceController(enclave string) *ServiceController {
	return &ServiceController{enclave: enclave, run: runKurtosis}
}

func (controller *ServiceController) Stop(ctx context.Context, services ...string) error {
	return controller.serviceCommand(ctx, "stop", services)
}

func (controller *ServiceController) Start(ctx context.Context, services ...string) error {
	return controller.serviceCommand(ctx, "start", services)
}

func (controller *ServiceController) Restart(ctx context.Context, services ...string) error {
	if err := controller.Stop(ctx, services...); err != nil {
		return err
	}
	return controller.Start(ctx, services...)
}

func (controller *ServiceController) serviceCommand(ctx context.Context, action string, services []string) error {
	if strings.TrimSpace(controller.enclave) == "" {
		return fmt.Errorf("%s service: enclave name is empty", action)
	}
	if len(services) == 0 {
		return fmt.Errorf("%s service: no service names supplied", action)
	}
	arguments := append([]string{"service", action, controller.enclave}, services...)
	if err := controller.run(ctx, arguments...); err != nil {
		return fmt.Errorf("kurtosis service %s: %w", action, err)
	}
	return nil
}

func runKurtosis(ctx context.Context, arguments ...string) error {
	output, err := exec.CommandContext(ctx, "kurtosis", arguments...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
