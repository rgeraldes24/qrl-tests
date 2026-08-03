// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

// Package lanes defines the live E2E execution matrix.
package lanes

import (
	"fmt"
	"slices"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
)

type Lane struct {
	Name        string
	Profile     devnet.Profile
	Suites      []Suite
	LabelFilter string
	Timeout     time.Duration
	Tools       []Tool
}

type Tool string

const (
	ToolGQRL Tool = "gqrl"
	ToolClef Tool = "clef"
)

type Suite struct {
	Package  string
	Requires []devnet.Capability
}

var registry = []Lane{
	{
		Name:    "single",
		Profile: devnet.ProfileSingle,
		Suites: packages(
			"./endtoend/suites/execution/abi",
			"./endtoend/suites/execution/api",
			"./endtoend/suites/execution/console",
			"./endtoend/suites/execution/vm",
			"./endtoend/suites/signer/...",
			"./endtoend/suites/consensus/api",
			"./endtoend/suites/consensus/protocol",
			"./endtoend/suites/crosslayer/engine",
		),
		LabelFilter: "!scenario-full && !profile-operations",
		Timeout:     90 * time.Minute,
		Tools:       []Tool{ToolGQRL, ToolClef},
	},
	{
		Name:        "multi",
		Profile:     devnet.ProfileMulti,
		Suites:      packages("./endtoend/suites/crosslayer/network", "./endtoend/suites/crosslayer/transactions"),
		LabelFilter: "!scenario-full",
		Timeout:     90 * time.Minute,
	},
	{
		Name:        "workloads",
		Profile:     devnet.ProfileMulti,
		Suites:      packages("./endtoend/suites/crosslayer/transactions"),
		LabelFilter: "scenario-full",
		Timeout:     3 * time.Hour,
	},
	{
		Name:        "lifecycle",
		Profile:     devnet.ProfileLifecycle,
		Suites:      packages("./endtoend/suites/crosslayer/validator"),
		LabelFilter: "!profile-operations",
		Timeout:     2 * time.Hour,
	},
	{
		Name:    "chaos",
		Profile: devnet.ProfileChaos,
		Suites: []Suite{
			{Package: "./endtoend/suites/crosslayer/network"},
			{Package: "./endtoend/suites/crosslayer/resilience"},
			{Package: "./endtoend/suites/crosslayer/partition", Requires: []devnet.Capability{devnet.CapabilityNetworkPartition}},
		},
		Timeout: 2 * time.Hour,
	},
	{
		Name:    "consensus-sync",
		Profile: devnet.ProfileSync,
		Suites:  packages("./endtoend/suites/consensus/sync"),
		Timeout: 45 * time.Minute,
	},
	{
		Name:    "execution-sync",
		Profile: devnet.ProfileExecutionSync,
		Suites:  packages("./endtoend/suites/execution/sync"),
		Timeout: 45 * time.Minute,
	},
	{
		Name:        "operations",
		Profile:     devnet.ProfileOperations,
		Suites:      packages("./endtoend/suites/crosslayer/validator"),
		LabelFilter: "profile-operations",
		Timeout:     4 * time.Hour,
	},
	{
		Name:    "cold-state",
		Profile: devnet.ProfileCold,
		Suites:  packages("./endtoend/suites/consensus/coldstate"),
		Timeout: 45 * time.Minute,
	},
	{
		Name:    "optimistic",
		Profile: devnet.ProfileOptimistic,
		Suites:  packages("./endtoend/suites/consensus/optimistic"),
		Timeout: 45 * time.Minute,
	},
	{
		Name:        "soak",
		Profile:     devnet.ProfileChaos,
		Suites:      []Suite{{Package: "./endtoend/suites/system/soak", Requires: []devnet.Capability{devnet.CapabilityNetworkPartition}}},
		LabelFilter: "scenario-full",
		Timeout:     4 * time.Hour,
	},
}

func packages(names ...string) []Suite {
	result := make([]Suite, len(names))
	for index, name := range names {
		result[index].Package = name
	}
	return result
}

func All() []Lane {
	result := make([]Lane, len(registry))
	copy(result, registry)
	for index := range result {
		result[index].Suites = cloneSuites(result[index].Suites)
		result[index].Tools = slices.Clone(result[index].Tools)
	}
	return result
}

func (lane Lane) ForBackend(backend devnet.Backend) (Lane, bool) {
	suites := make([]Suite, 0, len(lane.Suites))
	for _, suite := range lane.Suites {
		if supportsAll(backend, suite.Requires) {
			suites = append(suites, suite)
		}
	}
	lane.Suites = cloneSuites(suites)
	return lane, len(lane.Suites) != 0
}

func (lane Lane) Packages() []string {
	result := make([]string, len(lane.Suites))
	for index, suite := range lane.Suites {
		result[index] = suite.Package
	}
	return result
}

func Named(name string) (Lane, error) {
	for _, lane := range registry {
		if lane.Name == name {
			lane.Suites = cloneSuites(lane.Suites)
			lane.Tools = slices.Clone(lane.Tools)
			return lane, nil
		}
	}
	return Lane{}, fmt.Errorf("unknown E2E lane %q", name)
}

func cloneSuites(source []Suite) []Suite {
	result := make([]Suite, len(source))
	copy(result, source)
	for index := range result {
		result[index].Requires = slices.Clone(result[index].Requires)
	}
	return result
}

func supportsAll(backend devnet.Backend, required []devnet.Capability) bool {
	for _, capability := range required {
		if !backend.Supports(capability) {
			return false
		}
	}
	return true
}
