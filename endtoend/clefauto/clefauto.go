// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

// Package clefauto runs Clef with an automated UI for disposable E2E networks.
package clefauto

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/endtoend/internal/fixture"
	"github.com/theQRL/go-qrl/rpc"
	signercore "github.com/theQRL/go-qrl/signer/core"
)

type automatedUI struct{}

func (*automatedUI) ApproveTx(request *signercore.SignTxRequest) (signercore.SignTxResponse, error) {
	if request.Transaction.Value.ToInt().Cmp(big.NewInt(fixture.RemoteSignerRejectedTransaction)) == 0 {
		return signercore.SignTxResponse{Transaction: request.Transaction, Approved: false}, nil
	}
	if request.Transaction.Value.ToInt().Cmp(big.NewInt(fixture.RemoteSignerDelayedTransaction)) == 0 {
		time.Sleep(3 * time.Second)
	}
	return signercore.SignTxResponse{Transaction: request.Transaction, Approved: true}, nil
}

func (*automatedUI) ApproveSignData(request *signercore.SignDataRequest) (signercore.SignDataResponse, error) {
	for _, message := range request.Messages {
		if strings.Contains(fmt.Sprint(message.Value), fixture.RemoteSignerRejectedText) {
			return signercore.SignDataResponse{Approved: false}, nil
		}
	}
	return signercore.SignDataResponse{Approved: true}, nil
}

func (*automatedUI) ApproveListing(request *signercore.ListRequest) (signercore.ListResponse, error) {
	return signercore.ListResponse{Accounts: request.Accounts}, nil
}

func (*automatedUI) ApproveNewAccount(*signercore.NewAccountRequest) (signercore.NewAccountResponse, error) {
	return signercore.NewAccountResponse{Approved: true}, nil
}

func (*automatedUI) ShowError(signercore.Message) {}

func (*automatedUI) ShowInfo(signercore.Message) {}

func (*automatedUI) OnApprovedTx(any) {}

func (*automatedUI) OnSignerStartup(signercore.StartupInfo) {}

func (*automatedUI) OnInputRequired(signercore.UserInputRequest) (signercore.UserInputResponse, error) {
	return signercore.UserInputResponse{Text: fixture.RemoteSignerPassword}, nil
}

// Run starts Clef with the disposable-network automated UI.
func Run(ctx context.Context, arguments []string) error {
	args, cleanup, err := clefArgs(ctx, arguments)
	if err != nil {
		return err
	}
	defer cleanup()

	command := exec.CommandContext(ctx, "clef-bin", args...)
	command.Stderr = os.Stderr

	input, err := command.StdinPipe()
	if err != nil {
		return err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	client, err := rpc.DialIO(ctx, output, input)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.RegisterName("ui", new(automatedUI)); err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	if err := command.Wait(); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

func clefArgs(ctx context.Context, args []string) ([]string, func(), error) {
	dir, err := os.MkdirTemp("", "go-qrl-clef-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }

	passwordPath := filepath.Join(dir, "password")
	seedPath := filepath.Join(dir, "seed")
	keystorePath := filepath.Join(dir, "keystore")
	if err := os.WriteFile(passwordPath, []byte(fixture.RemoteSignerPassword), 0o600); err != nil {
		cleanup()
		return nil, nil, err
	}
	if err := os.WriteFile(seedPath, []byte(fixture.RemoteSignerSeed), 0o600); err != nil {
		cleanup()
		return nil, nil, err
	}

	importer := exec.CommandContext(ctx, "clef-bin",
		"--suppress-bootwarn",
		"--keystore="+keystorePath,
		"importraw",
		"--password="+passwordPath,
		seedPath,
	)
	importer.Stdout = os.Stderr
	importer.Stderr = os.Stderr
	if err := importer.Run(); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("import development account: %w", err)
	}

	configured := make([]string, 0, len(args)+2)
	keystoreSet := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "--keystore=") {
			arg = "--keystore=" + keystorePath
			keystoreSet = true
		}
		configured = append(configured, arg)
	}
	if !keystoreSet {
		configured = append(configured, "--keystore="+keystorePath)
	}
	return append(configured, "--stdio-ui"), cleanup, nil
}
