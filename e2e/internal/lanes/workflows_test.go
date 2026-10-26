package lanes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// The parts of a GitHub workflow the tests read.
type workflow struct {
	Env  map[string]string      `yaml:"env"`
	Jobs map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	Env   map[string]string `yaml:"env"`
	Steps []struct {
		Name string            `yaml:"name"`
		Env  map[string]string `yaml:"env"`
	} `yaml:"steps"`
}

func readWorkflow(t *testing.T, name string) workflow {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(repositoryRoot(t), ".github", "workflows", name))
	require.NoError(t, err)
	var parsed workflow
	require.NoError(t, yaml.Unmarshal(payload, &parsed))
	return parsed
}

// The image scripts build the commit REMOTESIGNER_REF or QRYSM_REF names. Only
// the build steps may set them, to the commit the images job resolved: the
// checkouts have no local branch to resolve a branch name against.
func TestRemoteSignerNightlyBuildsTheResolvedCommits(t *testing.T) {
	nightly := readWorkflow(t, "remote-signer-nightly.yml")
	commits := map[string]string{
		"REMOTESIGNER_REF": "${{ needs.images.outputs.remotesigner-sha }}",
		"QRYSM_REF":        "${{ needs.images.outputs.qrysm-sha }}",
	}
	for variable, commit := range commits {
		require.NotContainsf(t, nightly.Env, variable, "the workflow sets %s for every step", variable)
		steps := 0
		for name, job := range nightly.Jobs {
			require.NotContainsf(t, job.Env, variable, "job %s sets %s for every step", name, variable)
			for _, step := range job.Steps {
				if value, ok := step.Env[variable]; ok {
					require.Equalf(t, commit, value, "step %q", step.Name)
					steps++
				}
			}
		}
		require.Equalf(t, 1, steps, "steps that set %s", variable)
	}
}
