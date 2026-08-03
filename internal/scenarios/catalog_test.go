// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package scenarios

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cyyber/qrl-tests/internal/lanes"
	"github.com/stretchr/testify/require"
)

const sourceTableHeading = "## Source Scenario Inventory"

func TestScenarioInventoryIsExhaustive(t *testing.T) {
	root := repositoryRoot(t)
	source := scenarioNames(t, filepath.Join(root, "coverage/source-scenarios.txt"))
	coverage := coverageRows(t, filepath.Join(root, "docs/scenario-coverage.md"))
	contracts := behaviorContracts(t, filepath.Join(root, "coverage/source-behaviors.json"))
	suites := suiteSources(t, root)

	require.Len(t, source, 79)
	require.Len(t, coverage, len(source))
	for name := range source {
		row, ok := coverage[name]
		require.Truef(t, ok, "scenario %q is not classified", name)
		require.Contains(t, []string{"Full", "Equivalent", "Partial", "Failing", "Unsupported"}, row.disposition)
		require.NotEmptyf(t, row.replacement, "scenario %q has no coverage contract or exclusion reason", name)
		if row.disposition != "Unsupported" {
			contract, ok := contracts[name]
			require.Truef(t, ok, "scenario %q has no behavior-level contract", name)
			require.Equal(t, row.disposition, contract.Status, name)
			require.NotEmpty(t, contract.Behaviors, name)
			for _, behavior := range contract.Behaviors {
				require.NotEmpty(t, behavior.ID, name)
				require.Contains(t, []string{"Covered", "Equivalent", "Missing", "Failing", "Unsupported"}, behavior.Status)
				if behavior.Status == "Covered" || behavior.Status == "Equivalent" || behavior.Status == "Failing" {
					require.NotEmptyf(t, behavior.Label, "%s behavior %s has no Ginkgo label", name, behavior.ID)
					files := filesWithLabel(suites, behavior.Label)
					require.NotEmptyf(t, files, "%s behavior %s has no executable Ginkgo coverage", name, behavior.ID)
					require.Truef(t, selectedByLane(root, files), "%s behavior %s is not selected by any E2E lane", name, behavior.ID)
				} else {
					require.NotEmptyf(t, behavior.Reason, "%s behavior %s has no missing/unsupported reason", name, behavior.ID)
				}
			}
		} else {
			_, ok := contracts[name]
			require.Falsef(t, ok, "unsupported scenario %q must not claim a behavior contract", name)
		}
	}
	for name := range contracts {
		_, ok := source[name]
		require.Truef(t, ok, "behavior contract references unknown scenario %q", name)
	}
}

type behaviorContract struct {
	Scenario  string     `json:"scenario"`
	Status    string     `json:"status"`
	Behaviors []behavior `json:"behaviors"`
}

type behavior struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

func behaviorContracts(t *testing.T, path string) map[string]behaviorContract {
	t.Helper()
	payload, err := os.ReadFile(path)
	require.NoError(t, err)
	var source []behaviorContract
	require.NoError(t, json.Unmarshal(payload, &source))

	contracts := make(map[string]behaviorContract, len(source))
	for _, contract := range source {
		require.NotEmpty(t, contract.Scenario)
		_, duplicate := contracts[contract.Scenario]
		require.Falsef(t, duplicate, "duplicate behavior contract for %q", contract.Scenario)
		contracts[contract.Scenario] = contract
	}
	return contracts
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
		for _, pattern := range lane.Packages {
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

type coverageRow struct {
	disposition string
	replacement string
}

func coverageRows(t *testing.T, path string) map[string]coverageRow {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	rows := make(map[string]coverageRow)
	inSourceTable := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == sourceTableHeading {
			inSourceTable = true
			continue
		}
		if inSourceTable && strings.HasPrefix(line, "## ") {
			break
		}
		if !inSourceTable || !strings.HasPrefix(line, "| `") {
			continue
		}
		columns := strings.Split(line, "|")
		require.Len(t, columns, 5, line)
		name := strings.Trim(strings.TrimSpace(columns[1]), "`")
		_, duplicate := rows[name]
		require.Falsef(t, duplicate, "duplicate scenario %q", name)
		rows[name] = coverageRow{
			disposition: strings.TrimSpace(columns[2]),
			replacement: strings.TrimSpace(columns[3]),
		}
	}
	require.NoError(t, scanner.Err())
	return rows
}

func scenarioNames(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	names := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		_, duplicate := names[name]
		require.Falsef(t, duplicate, "duplicate source scenario %q", name)
		names[name] = struct{}{}
	}
	require.NoError(t, scanner.Err())
	return names
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
}
