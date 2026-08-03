// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

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
	Packages    []string
	LabelFilter string
	Timeout     time.Duration

	// KubernetesPackages overrides Packages when running on Kubernetes. A
	// non-nil empty slice marks the entire lane unsupported.
	KubernetesPackages []string
}

var registry = []Lane{
	{
		Name:    "single",
		Profile: devnet.ProfileSingle,
		Packages: []string{
			"./endtoend/suites/execution/abi",
			"./endtoend/suites/execution/api",
			"./endtoend/suites/execution/console",
			"./endtoend/suites/execution/vm",
			"./endtoend/suites/signer/...",
			"./endtoend/suites/consensus/api",
			"./endtoend/suites/consensus/protocol",
			"./endtoend/suites/crosslayer/engine",
		},
		LabelFilter: "!scenario-full && !profile-operations",
		Timeout:     90 * time.Minute,
	},
	{
		Name:        "multi",
		Profile:     devnet.ProfileMulti,
		Packages:    []string{"./endtoend/suites/crosslayer/network", "./endtoend/suites/crosslayer/transactions"},
		LabelFilter: "!scenario-full",
		Timeout:     90 * time.Minute,
	},
	{
		Name:        "workloads",
		Profile:     devnet.ProfileMulti,
		Packages:    []string{"./endtoend/suites/crosslayer/transactions"},
		LabelFilter: "scenario-full",
		Timeout:     3 * time.Hour,
	},
	{
		Name:        "lifecycle",
		Profile:     devnet.ProfileLifecycle,
		Packages:    []string{"./endtoend/suites/crosslayer/validator"},
		LabelFilter: "!profile-operations",
		Timeout:     2 * time.Hour,
	},
	{
		Name:     "chaos",
		Profile:  devnet.ProfileChaos,
		Packages: []string{"./endtoend/suites/crosslayer/network", "./endtoend/suites/crosslayer/resilience", "./endtoend/suites/crosslayer/partition"},
		KubernetesPackages: []string{
			"./endtoend/suites/crosslayer/network",
			"./endtoend/suites/crosslayer/resilience",
		},
		Timeout: 2 * time.Hour,
	},
	{
		Name:     "consensus-sync",
		Profile:  devnet.ProfileSync,
		Packages: []string{"./endtoend/suites/consensus/sync"},
		Timeout:  45 * time.Minute,
	},
	{
		Name:     "execution-sync",
		Profile:  devnet.ProfileExecutionSync,
		Packages: []string{"./endtoend/suites/execution/sync"},
		Timeout:  45 * time.Minute,
	},
	{
		Name:        "operations",
		Profile:     devnet.ProfileOperations,
		Packages:    []string{"./endtoend/suites/crosslayer/validator"},
		LabelFilter: "profile-operations",
		Timeout:     4 * time.Hour,
	},
	{
		Name:     "cold-state",
		Profile:  devnet.ProfileCold,
		Packages: []string{"./endtoend/suites/consensus/coldstate"},
		Timeout:  45 * time.Minute,
	},
	{
		Name:     "optimistic",
		Profile:  devnet.ProfileOptimistic,
		Packages: []string{"./endtoend/suites/consensus/optimistic"},
		Timeout:  45 * time.Minute,
	},
	{
		Name:               "soak",
		Profile:            devnet.ProfileChaos,
		Packages:           []string{"./endtoend/suites/system/soak"},
		LabelFilter:        "scenario-full",
		Timeout:            4 * time.Hour,
		KubernetesPackages: []string{},
	},
}

func All() []Lane {
	result := make([]Lane, len(registry))
	copy(result, registry)
	for index := range result {
		result[index].Packages = slices.Clone(result[index].Packages)
		result[index].KubernetesPackages = slices.Clone(result[index].KubernetesPackages)
	}
	return result
}

func (lane Lane) ForBackend(backend devnet.Backend) (Lane, bool) {
	if backend == devnet.BackendKubernetes && lane.KubernetesPackages != nil {
		lane.Packages = slices.Clone(lane.KubernetesPackages)
	}
	return lane, len(lane.Packages) != 0
}

func Named(name string) (Lane, error) {
	for _, lane := range registry {
		if lane.Name == name {
			lane.Packages = slices.Clone(lane.Packages)
			lane.KubernetesPackages = slices.Clone(lane.KubernetesPackages)
			return lane, nil
		}
	}
	return Lane{}, fmt.Errorf("unknown E2E lane %q", name)
}
