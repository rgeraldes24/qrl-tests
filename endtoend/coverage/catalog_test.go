// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package coverage

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cyyber/qrl-tests/endtoend/internal/lanes"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

type catalog struct {
	Version             int        `yaml:"version"`
	SourceScenarioCount int        `yaml:"source_scenario_count"`
	Scenarios           []scenario `yaml:"scenarios"`
}

type scenario struct {
	ID          string     `yaml:"id"`
	Disposition string     `yaml:"disposition"`
	Replacement string     `yaml:"replacement"`
	Behaviors   []behavior `yaml:"behaviors,omitempty"`
}

type behavior struct {
	ID     string `yaml:"id"`
	Status string `yaml:"status"`
	Label  string `yaml:"label,omitempty"`
	Reason string `yaml:"reason,omitempty"`
}

func TestScenarioInventoryIsExhaustive(t *testing.T) {
	root := repositoryRoot(t)
	catalog, err := loadCatalog(filepath.Join(root, "endtoend/coverage/scenarios.yaml"))
	require.NoError(t, err)
	require.Equal(t, 1, catalog.Version)
	require.Len(t, catalog.Scenarios, catalog.SourceScenarioCount)
	suites := suiteSources(t, root)
	seen := make(map[string]struct{}, len(catalog.Scenarios))

	for _, scenario := range catalog.Scenarios {
		require.NotEmpty(t, scenario.ID)
		_, duplicate := seen[scenario.ID]
		require.Falsef(t, duplicate, "duplicate scenario %q", scenario.ID)
		seen[scenario.ID] = struct{}{}
		require.Contains(t, []string{"Full", "Equivalent", "Partial", "Failing", "Unsupported"}, scenario.Disposition)
		require.NotEmptyf(t, scenario.Replacement, "scenario %q has no coverage contract or exclusion reason", scenario.ID)
		if scenario.Disposition == "Unsupported" {
			require.Emptyf(t, scenario.Behaviors, "unsupported scenario %q must not claim a behavior contract", scenario.ID)
			continue
		}
		require.NotEmptyf(t, scenario.Behaviors, "scenario %q has no behavior-level contract", scenario.ID)
		for _, behavior := range scenario.Behaviors {
			require.NotEmpty(t, behavior.ID, scenario.ID)
			require.Contains(t, []string{"Covered", "Equivalent", "Missing", "Failing", "Unsupported"}, behavior.Status)
			if behavior.Status == "Covered" || behavior.Status == "Equivalent" || behavior.Status == "Failing" {
				require.NotEmptyf(t, behavior.Label, "%s behavior %s has no Ginkgo label", scenario.ID, behavior.ID)
				files := filesWithLabel(suites, behavior.Label)
				require.NotEmptyf(t, files, "%s behavior %s has no executable Ginkgo coverage", scenario.ID, behavior.ID)
				require.Truef(t, selectedByLane(root, files), "%s behavior %s is not selected by any E2E lane", scenario.ID, behavior.ID)
			} else {
				require.NotEmptyf(t, behavior.Reason, "%s behavior %s has no missing/unsupported reason", scenario.ID, behavior.ID)
			}
		}
	}
}

func loadCatalog(path string) (catalog, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return catalog{}, fmt.Errorf("read scenario catalog: %w", err)
	}
	var result catalog
	if err := yaml.Unmarshal(payload, &result); err != nil {
		return catalog{}, fmt.Errorf("decode scenario catalog: %w", err)
	}
	return result, nil
}

func suiteSources(t *testing.T, root string) map[string]string {
	t.Helper()
	sources := make(map[string]string)
	err := filepath.WalkDir(filepath.Join(root, "endtoend/suites"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sources[filepath.ToSlash(relative)] = string(contents)
		return nil
	})
	require.NoError(t, err)
	return sources
}

func filesWithLabel(sources map[string]string, label string) []string {
	var files []string
	for path, source := range sources {
		if strings.Contains(source, `"`+label+`"`) {
			files = append(files, path)
		}
	}
	slices.Sort(files)
	return files
}

func selectedByLane(root string, files []string) bool {
	for _, lane := range lanes.All() {
		for _, pattern := range lane.Packages() {
			packageRoot := strings.TrimSuffix(strings.TrimPrefix(pattern, "./"), "/...")
			for _, file := range files {
				if file == packageRoot || strings.HasPrefix(file, packageRoot+"/") {
					if _, err := os.Stat(filepath.Join(root, file)); err == nil {
						return true
					}
				}
			}
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
