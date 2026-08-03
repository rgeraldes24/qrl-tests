// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cyyber/qrl-tests/devnet"
	"github.com/urfave/cli/v2"
)

type controller interface {
	Start(context.Context, devnet.StartOptions) (devnet.Environment, error)
	Stop(context.Context, string) error
}

func networkCommand(networks controller) *cli.Command {
	startFlags := []cli.Flag{
		enclaveNameFlag(),
		backendFlag(),
		&cli.StringFlag{Name: "profile", Usage: "built-in network profile", Value: string(devnet.ProfileSingle), EnvVars: []string{"DEVNET_PROFILE"}},
		&cli.StringFlag{Name: "params-file", Usage: "complete YAML or JSON qrl-package parameters", EnvVars: []string{"DEVNET_PARAMS_FILE"}},
		&cli.DurationFlag{Name: "timeout", Usage: "network start budget", Value: devnet.DefaultStartTimeout, EnvVars: []string{"DEVNET_START_TIMEOUT"}},
	}
	startFlags = append(startFlags, imageFlags()...)
	return &cli.Command{
		Name:  "network",
		Usage: "control a separately managed development network",
		Subcommands: []*cli.Command{
			{
				Name:  "start",
				Usage: "start the development network and wait for readiness",
				Flags: startFlags,
				Action: func(command *cli.Context) error {
					if err := rejectPositional(command); err != nil {
						return err
					}
					var parameters []byte
					if file := command.String("params-file"); file != "" {
						var err error
						parameters, err = os.ReadFile(file)
						if err != nil {
							return fmt.Errorf("read parameters file: %w", err)
						}
					}
					ctx, cancel := context.WithTimeout(command.Context, command.Duration("timeout"))
					defer cancel()
					if _, err := networks.Start(ctx, devnet.StartOptions{
						EnclaveName: command.String("enclave-name"),
						Backend:     devnet.Backend(command.String("backend")),
						Images:      imagesFrom(command),
						Parameters:  parameters,
						Profile:     devnet.Profile(command.String("profile")),
					}); err != nil {
						return err
					}
					_, err := fmt.Fprintln(command.App.Writer, "network ready")
					return err
				},
			},
			{
				Name:  "stop",
				Usage: "stop the development network",
				Flags: []cli.Flag{enclaveNameFlag()},
				Action: func(command *cli.Context) error {
					if err := rejectPositional(command); err != nil {
						return err
					}
					if err := networks.Stop(command.Context, command.String("enclave-name")); err != nil {
						return err
					}
					_, err := fmt.Fprintln(command.App.Writer, "network stopped")
					return err
				},
			},
		},
	}
}
