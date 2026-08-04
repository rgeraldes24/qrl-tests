// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

// Package apicoverage defines the common coverage contract used by execution
// and consensus API inventories.
package apicoverage

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
)

type Level string

const (
	Behavior Level = "live behavior"
	Shape    Level = "live response shape"
	Dispatch Level = "live dispatch and error contract"
)

type Scenario string

type Entry struct {
	Level     Level
	Scenario  Scenario
	Exclusion string
}

func Live(level Level, scenario Scenario) Entry {
	return Entry{Level: level, Scenario: scenario}
}

func Excluded(reason string) Entry {
	return Entry{Exclusion: reason}
}

func Validate(entries map[string]Entry, scenarios map[Scenario]string) error {
	for endpoint, entry := range entries {
		switch {
		case entry.Exclusion != "":
			if entry.Level != "" || entry.Scenario != "" {
				return fmt.Errorf("%s is excluded but references live coverage", endpoint)
			}
		case entry.Level == "":
			return fmt.Errorf("%s has no coverage category", endpoint)
		case entry.Level != Behavior && entry.Level != Shape && entry.Level != Dispatch:
			return fmt.Errorf("%s has unknown coverage category %q", endpoint, entry.Level)
		case strings.TrimSpace(string(entry.Scenario)) == "":
			return fmt.Errorf("%s has no live scenario", endpoint)
		default:
			if _, ok := scenarios[entry.Scenario]; !ok {
				return fmt.Errorf("%s references unknown live scenario %q", endpoint, entry.Scenario)
			}
		}
	}
	return nil
}

func InventoryDigest(entries map[string]Entry) string {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(keys, "\n")+"\n")))
}
