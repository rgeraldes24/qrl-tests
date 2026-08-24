package devnet

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveExecutionImageUsesPrimaryServiceContainer(t *testing.T) {
	containerID := strings.Repeat("cd", 32)
	imageID := "sha256:" + strings.Repeat("ab", 32)
	var calls [][]string
	output := func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		call := append([]string{name}, arguments...)
		calls = append(calls, call)
		switch len(calls) {
		case 1:
			return []byte(containerID + "\n"), nil
		case 2:
			return []byte(imageID + "\n"), nil
		default:
			t.Fatalf("unexpected command %q", call)
			return nil, nil
		}
	}

	resolved, err := resolveExecutionImage(t.Context(), executionImageTestEnvironment(), output)
	require.NoError(t, err)
	require.Equal(t, imageID, resolved)
	require.Equal(t, [][]string{
		{
			"docker", "container", "ls", "--no-trunc", "--quiet", "--filter",
			"label=com.kurtosistech.guid=primary-execution-service",
		},
		{
			"docker", "container", "inspect", "--format",
			"{{if .State.Running}}{{.Image}}{{end}}", containerID,
		},
	}, calls)
}

func TestResolveExecutionImageRejectsUnverifiableResults(t *testing.T) {
	containerID := strings.Repeat("cd", 32)
	for name, testCase := range map[string]struct {
		firstOutput  string
		secondOutput string
		wantErr      string
	}{
		"no matching container": {
			wantErr: "expected one running Docker container for service \"primary-execution-service\", found 0",
		},
		"multiple matching containers": {
			firstOutput: "first\nsecond\n",
			wantErr:     "expected one running Docker container for service \"primary-execution-service\", found 2",
		},
		"container stopped": {
			firstOutput: containerID,
			wantErr:     "is not running",
		},
		"malformed image ID": {
			firstOutput:  containerID,
			secondOutput: "registry.example/go-qrl:mutable",
			wantErr:      "Docker returned invalid image ID",
		},
	} {
		t.Run(name, func(t *testing.T) {
			call := 0
			output := func(context.Context, string, ...string) ([]byte, error) {
				call++
				if call == 1 {
					return []byte(testCase.firstOutput), nil
				}
				return []byte(testCase.secondOutput), nil
			}

			_, err := resolveExecutionImage(t.Context(), executionImageTestEnvironment(), output)
			require.ErrorContains(t, err, testCase.wantErr)
		})
	}
}

func TestResolveExecutionImageRequiresDockerServiceIdentity(t *testing.T) {
	environment := executionImageTestEnvironment()
	environment.Backend = BackendKubernetes
	_, err := resolveExecutionImage(t.Context(), environment, nil)
	require.ErrorContains(t, err, "is not Docker")

	environment.Backend = BackendDocker
	environment.Participants[0].Execution.ID = ""
	_, err = resolveExecutionImage(t.Context(), environment, nil)
	require.ErrorContains(t, err, "primary execution service has no ID")
}

func TestExecutionImageCommandHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := executeOutput(ctx, "go", "version")
	require.ErrorIs(t, err, context.Canceled)
	require.EqualError(t, err, context.Canceled.Error())
}

func executionImageTestEnvironment() Environment {
	return Environment{
		Backend: BackendDocker,
		Participants: []Participant{{
			Index: 1,
			Execution: ExecutionService{ServiceInfo: ServiceInfo{
				ID: "primary-execution-service",
			}},
		}},
	}
}
