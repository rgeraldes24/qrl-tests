// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package coverage

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

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
