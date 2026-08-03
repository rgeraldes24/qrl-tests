package coverage

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/lanes"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

type manifest struct {
	Surfaces []surface `yaml:"surfaces"`
}

type surface struct {
	Name    string  `yaml:"name"`
	Entries []entry `yaml:"entries"`
}

type entry struct {
	ID             string `yaml:"id"`
	Package        string `yaml:"package"`
	Lane           string `yaml:"lane"`
	Classification string `yaml:"classification"`
}

func TestAPISurfaceManifests(t *testing.T) {
	root := repositoryRoot(t)
	payload, err := os.ReadFile(filepath.Join(root, "endtoend", "coverage", "surfaces.yaml"))
	require.NoError(t, err)
	var source manifest
	require.NoError(t, yaml.Unmarshal(payload, &source))
	require.NotEmpty(t, source.Surfaces)
	for _, surface := range source.Surfaces {
		t.Run(surface.Name, func(t *testing.T) {
			require.NotEmpty(t, surface.Name)
			require.NotEmpty(t, surface.Entries)
			seen := make(map[string]struct{})
			for _, item := range surface.Entries {
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
				require.Truef(t, laneSelects(lane.Packages(), item.Package), "lane %s does not select %s", lane.Name, item.Package)
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
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
}
