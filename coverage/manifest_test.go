package coverage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cyyber/qrl-tests/internal/lanes"
	"github.com/stretchr/testify/require"
)

type manifest struct {
	Surface string  `json:"surface"`
	Entries []entry `json:"entries"`
}

type entry struct {
	ID             string `json:"id"`
	Package        string `json:"package"`
	Lane           string `json:"lane"`
	Classification string `json:"classification"`
}

func TestAPISurfaceManifests(t *testing.T) {
	root := repositoryRoot(t)
	for _, name := range []string{"execution-rpc.json", "beacon-rest.json", "validator-rest.json", "engine-rpc.json"} {
		t.Run(name, func(t *testing.T) {
			payload, err := os.ReadFile(filepath.Join(root, "coverage", name))
			require.NoError(t, err)
			var source manifest
			require.NoError(t, json.Unmarshal(payload, &source))
			require.NotEmpty(t, source.Surface)
			require.NotEmpty(t, source.Entries)
			seen := make(map[string]struct{})
			for _, item := range source.Entries {
				require.NotEmpty(t, item.ID)
				_, duplicate := seen[item.ID]
				require.Falsef(t, duplicate, "duplicate entry %q", item.ID)
				seen[item.ID] = struct{}{}
				require.Contains(t, []string{"behavior", "shape", "expected-error", "excluded"}, item.Classification)
				if item.Classification == "excluded" {
					continue
				}
				lane, err := lanes.Named(item.Lane)
				require.NoError(t, err)
				require.Truef(t, laneSelects(lane.Packages, item.Package), "lane %s does not select %s", lane.Name, item.Package)
				info, err := os.Stat(filepath.Join(root, item.Package[2:]))
				require.NoError(t, err)
				require.True(t, info.IsDir())
			}
		})
	}
}

func laneSelects(patterns []string, packagePath string) bool {
	for _, pattern := range patterns {
		if pattern == packagePath || strings.HasSuffix(pattern, "/...") && strings.HasPrefix(packagePath, strings.TrimSuffix(pattern, "/...")) {
			return true
		}
	}
	return false
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))
}
