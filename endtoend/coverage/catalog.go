// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package coverage

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

//go:generate go run ./internal/gendocs

const SourceTableHeading = "## Source Scenario Inventory"

type Catalog struct {
	Version             int        `yaml:"version"`
	SourceScenarioCount int        `yaml:"source_scenario_count"`
	Scenarios           []Scenario `yaml:"scenarios"`
}

type Scenario struct {
	ID          string     `yaml:"id"`
	Disposition string     `yaml:"disposition"`
	Replacement string     `yaml:"replacement"`
	Behaviors   []Behavior `yaml:"behaviors,omitempty"`
}

type Behavior struct {
	ID     string `yaml:"id"`
	Status string `yaml:"status"`
	Label  string `yaml:"label,omitempty"`
	Reason string `yaml:"reason,omitempty"`
}

func Load(path string) (Catalog, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("read scenario catalog: %w", err)
	}
	var catalog Catalog
	if err := yaml.Unmarshal(payload, &catalog); err != nil {
		return Catalog{}, fmt.Errorf("decode scenario catalog: %w", err)
	}
	return catalog, nil
}

func RenderScenarioInventory(catalog Catalog) string {
	var output bytes.Buffer
	fmt.Fprintln(&output, SourceTableHeading)
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "| Source scenario | Disposition | QRL replacement or reason |")
	fmt.Fprintln(&output, "| --- | --- | --- |")
	for _, scenario := range catalog.Scenarios {
		fmt.Fprintf(&output, "| `%s` | %s | %s |\n", scenario.ID, scenario.Disposition, scenario.Replacement)
	}
	return strings.TrimSpace(output.String())
}

func ReplaceScenarioInventory(document string, inventory string) (string, error) {
	start := strings.Index(document, SourceTableHeading)
	if start < 0 {
		return "", fmt.Errorf("documentation section %q not found", SourceTableHeading)
	}
	rest := document[start+len(SourceTableHeading):]
	next := strings.Index(rest, "\n## ")
	if next < 0 {
		return strings.TrimSpace(document[:start]) + "\n\n" + inventory + "\n", nil
	}
	end := start + len(SourceTableHeading) + next + 1
	return strings.TrimSpace(document[:start]) + "\n\n" + inventory + "\n\n" + strings.TrimLeft(document[end:], "\n"), nil
}
