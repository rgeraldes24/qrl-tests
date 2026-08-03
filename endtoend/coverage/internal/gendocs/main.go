package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cyyber/qrl-tests/endtoend/coverage"
)

func main() {
	coverageDirectory, err := os.Getwd()
	check(err)
	catalog, err := coverage.Load(filepath.Join(coverageDirectory, "scenarios.yaml"))
	check(err)
	documentPath := filepath.Clean(filepath.Join(coverageDirectory, "../../docs/scenario-coverage.md"))
	document, err := os.ReadFile(documentPath)
	check(err)
	updated, err := coverage.ReplaceScenarioInventory(string(document), coverage.RenderScenarioInventory(catalog))
	check(err)
	check(os.WriteFile(documentPath, []byte(updated), 0o644))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
