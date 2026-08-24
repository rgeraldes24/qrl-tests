package devnet

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const kurtosisServiceUUIDDockerLabel = "com.kurtosistech.guid"

// ResolveExecutionImage returns the image ID used by the primary execution service.
func ResolveExecutionImage(ctx context.Context, environment Environment) (string, error) {
	return resolveExecutionImage(ctx, environment, executeOutput)
}

func resolveExecutionImage(
	ctx context.Context,
	environment Environment,
	output func(context.Context, string, ...string) ([]byte, error),
) (string, error) {
	if environment.Backend != BackendDocker {
		return "", fmt.Errorf("resolve execution image: backend %q is not Docker", environment.Backend)
	}

	primary, err := environment.Primary()
	if err != nil {
		return "", fmt.Errorf("resolve execution image: %w", err)
	}
	serviceID := strings.TrimSpace(primary.Execution.ID)
	if serviceID == "" {
		return "", errors.New("resolve execution image: primary execution service has no ID")
	}

	containerOutput, err := output(
		ctx,
		"docker",
		"container", "ls",
		"--no-trunc",
		"--quiet",
		"--filter", "label="+kurtosisServiceUUIDDockerLabel+"="+serviceID,
	)
	if err != nil {
		return "", fmt.Errorf("find primary execution container: %w", err)
	}
	containerIDs := strings.Fields(string(containerOutput))
	if len(containerIDs) != 1 {
		return "", fmt.Errorf(
			"resolve execution image: expected one running Docker container for service %q, found %d",
			serviceID,
			len(containerIDs),
		)
	}

	imageOutput, err := output(
		ctx,
		"docker",
		"container", "inspect",
		"--format", "{{.Image}}",
		containerIDs[0],
	)
	if err != nil {
		return "", fmt.Errorf("inspect primary execution container %q: %w", containerIDs[0], err)
	}
	imageID := strings.TrimSpace(string(imageOutput))
	if !validSHA256ID(imageID) {
		return "", fmt.Errorf("resolve execution image: Docker returned invalid image ID %q", imageID)
	}
	return imageID, nil
}

func validSHA256ID(value string) bool {
	encoded, found := strings.CutPrefix(value, "sha256:")
	if !found || len(encoded) != 64 {
		return false
	}
	_, err := hex.DecodeString(encoded)
	return err == nil
}

func executeOutput(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, arguments...)
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		if detail := strings.TrimSpace(string(exitError.Stderr)); detail != "" {
			return output, errors.Join(fmt.Errorf("%w: %s", err, detail), ctx.Err())
		}
	}
	return output, errors.Join(err, ctx.Err())
}
