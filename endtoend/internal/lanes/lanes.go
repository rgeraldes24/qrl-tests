// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

// Package lanes defines the live E2E execution matrix.
package lanes

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cyyber/qrl-tests/devnet"
)

type Lane struct {
	Name        string
	Profile     devnet.Profile
	Suites      []SuiteID
	LabelFilter string
	Timeout     time.Duration
}

type Tool string

const (
	ToolGQRL Tool = "gqrl"
	ToolClef Tool = "clef"
)

type SuiteID string

const (
	SuiteExecutionABI        SuiteID = "execution-abi"
	SuiteExecutionAPI        SuiteID = "execution-api"
	SuiteExecutionConsole    SuiteID = "execution-console"
	SuiteExecutionVM         SuiteID = "execution-vm"
	SuiteExternalSigner      SuiteID = "external-signer"
	SuiteClef                SuiteID = "clef"
	SuiteConsensusAPI        SuiteID = "consensus-api"
	SuiteConsensusProtocol   SuiteID = "consensus-protocol"
	SuiteEngine              SuiteID = "engine"
	SuiteNetwork             SuiteID = "network"
	SuiteTransactions        SuiteID = "transactions"
	SuiteValidator           SuiteID = "validator"
	SuiteResilience          SuiteID = "resilience"
	SuitePartition           SuiteID = "partition"
	SuiteConsensusSync       SuiteID = "consensus-sync"
	SuiteExecutionSync       SuiteID = "execution-sync"
	SuiteConsensusColdState  SuiteID = "consensus-cold-state"
	SuiteConsensusOptimistic SuiteID = "consensus-optimistic"
	SuiteSoak                SuiteID = "soak"
)

type Suite struct {
	ID       SuiteID
	Package  string
	Requires []devnet.Capability
	Tools    []Tool
}

var suites = map[SuiteID]Suite{
	SuiteExecutionABI:        {ID: SuiteExecutionABI, Package: "./endtoend/suites/execution/abi"},
	SuiteExecutionAPI:        {ID: SuiteExecutionAPI, Package: "./endtoend/suites/execution/api"},
	SuiteExecutionConsole:    {ID: SuiteExecutionConsole, Package: "./endtoend/suites/execution/console", Tools: []Tool{ToolGQRL}},
	SuiteExecutionVM:         {ID: SuiteExecutionVM, Package: "./endtoend/suites/execution/vm"},
	SuiteExternalSigner:      {ID: SuiteExternalSigner, Package: "./endtoend/suites/signer/externalsigner"},
	SuiteClef:                {ID: SuiteClef, Package: "./endtoend/suites/signer/clef", Tools: []Tool{ToolClef}},
	SuiteConsensusAPI:        {ID: SuiteConsensusAPI, Package: "./endtoend/suites/consensus/api"},
	SuiteConsensusProtocol:   {ID: SuiteConsensusProtocol, Package: "./endtoend/suites/consensus/protocol"},
	SuiteEngine:              {ID: SuiteEngine, Package: "./endtoend/suites/crosslayer/engine"},
	SuiteNetwork:             {ID: SuiteNetwork, Package: "./endtoend/suites/crosslayer/network"},
	SuiteTransactions:        {ID: SuiteTransactions, Package: "./endtoend/suites/crosslayer/transactions"},
	SuiteValidator:           {ID: SuiteValidator, Package: "./endtoend/suites/crosslayer/validator"},
	SuiteResilience:          {ID: SuiteResilience, Package: "./endtoend/suites/crosslayer/resilience"},
	SuitePartition:           {ID: SuitePartition, Package: "./endtoend/suites/crosslayer/partition", Requires: []devnet.Capability{devnet.CapabilityNetworkPartition}},
	SuiteConsensusSync:       {ID: SuiteConsensusSync, Package: "./endtoend/suites/consensus/sync"},
	SuiteExecutionSync:       {ID: SuiteExecutionSync, Package: "./endtoend/suites/execution/sync"},
	SuiteConsensusColdState:  {ID: SuiteConsensusColdState, Package: "./endtoend/suites/consensus/coldstate"},
	SuiteConsensusOptimistic: {ID: SuiteConsensusOptimistic, Package: "./endtoend/suites/consensus/optimistic"},
	SuiteSoak:                {ID: SuiteSoak, Package: "./endtoend/suites/system/soak", Requires: []devnet.Capability{devnet.CapabilityNetworkPartition}},
}

var registry = []Lane{
	{
		Name:    "single",
		Profile: devnet.ProfileSingle,
		Suites: []SuiteID{
			SuiteExecutionABI,
			SuiteExecutionAPI,
			SuiteExecutionConsole,
			SuiteExecutionVM,
			SuiteExternalSigner,
			SuiteClef,
			SuiteConsensusAPI,
			SuiteConsensusProtocol,
			SuiteEngine,
		},
		LabelFilter: "!scenario-full && !profile-operations",
		Timeout:     90 * time.Minute,
	},
	{
		Name:        "multi",
		Profile:     devnet.ProfileMulti,
		Suites:      []SuiteID{SuiteNetwork, SuiteTransactions},
		LabelFilter: "!scenario-full",
		Timeout:     90 * time.Minute,
	},
	{
		Name:        "workloads",
		Profile:     devnet.ProfileMulti,
		Suites:      []SuiteID{SuiteTransactions},
		LabelFilter: "scenario-full",
		Timeout:     3 * time.Hour,
	},
	{
		Name:        "lifecycle",
		Profile:     devnet.ProfileSingle,
		Suites:      []SuiteID{SuiteValidator},
		LabelFilter: "!profile-operations",
		Timeout:     2 * time.Hour,
	},
	{
		Name:    "chaos",
		Profile: devnet.ProfileChaos,
		Suites:  []SuiteID{SuiteNetwork, SuiteResilience, SuitePartition},
		Timeout: 2 * time.Hour,
	},
	{
		Name:    "consensus-sync",
		Profile: devnet.ProfileSync,
		Suites:  []SuiteID{SuiteConsensusSync},
		Timeout: 45 * time.Minute,
	},
	{
		Name:    "execution-sync",
		Profile: devnet.ProfileExecutionSync,
		Suites:  []SuiteID{SuiteExecutionSync},
		Timeout: 45 * time.Minute,
	},
	{
		Name:        "operations",
		Profile:     devnet.ProfileOperations,
		Suites:      []SuiteID{SuiteValidator},
		LabelFilter: "profile-operations",
		Timeout:     4 * time.Hour,
	},
	{
		Name:    "cold-state",
		Profile: devnet.ProfileCold,
		Suites:  []SuiteID{SuiteConsensusColdState},
		Timeout: 45 * time.Minute,
	},
	{
		Name:    "optimistic",
		Profile: devnet.ProfileOptimistic,
		Suites:  []SuiteID{SuiteConsensusOptimistic},
		Timeout: 45 * time.Minute,
	},
	{
		Name:        "soak",
		Profile:     devnet.ProfileChaos,
		Suites:      []SuiteID{SuiteSoak},
		LabelFilter: "scenario-full",
		Timeout:     4 * time.Hour,
	},
}

func All() []Lane {
	return slices.Clone(registry)
}

func (lane Lane) ForBackend(backend devnet.Backend) (Lane, bool) {
	selected := make([]SuiteID, 0, len(lane.Suites))
	for _, id := range lane.Suites {
		suite := suites[id]
		if supportsAll(backend, suite.Requires) {
			selected = append(selected, id)
		}
	}
	lane.Suites = selected
	return lane, len(lane.Suites) != 0
}

func (lane Lane) Select(names []string) (Lane, error) {
	if len(names) == 0 {
		return lane, nil
	}
	wanted := make(map[SuiteID]struct{}, len(names))
	for _, name := range names {
		id := SuiteID(strings.TrimSpace(name))
		if _, exists := suites[id]; !exists {
			return Lane{}, fmt.Errorf("unknown E2E suite %q", name)
		}
		wanted[id] = struct{}{}
	}
	selected := make([]SuiteID, 0, len(wanted))
	for _, id := range lane.Suites {
		if _, exists := wanted[id]; exists {
			selected = append(selected, id)
			delete(wanted, id)
		}
	}
	if len(wanted) != 0 {
		missing := make([]string, 0, len(wanted))
		for id := range wanted {
			missing = append(missing, string(id))
		}
		slices.Sort(missing)
		return Lane{}, fmt.Errorf("suites %s are not available in lane %q", strings.Join(missing, ", "), lane.Name)
	}
	lane.Suites = selected
	return lane, nil
}

func (lane Lane) Packages() []string {
	result := make([]string, len(lane.Suites))
	for index, id := range lane.Suites {
		result[index] = suites[id].Package
	}
	return result
}

func (lane Lane) Tools() []Tool {
	var result []Tool
	for _, id := range lane.Suites {
		for _, tool := range suites[id].Tools {
			if !slices.Contains(result, tool) {
				result = append(result, tool)
			}
		}
	}
	return result
}

func RegisteredSuites() []Suite {
	result := make([]Suite, 0, len(suites))
	for _, suite := range suites {
		result = append(result, suite)
	}
	slices.SortFunc(result, func(left, right Suite) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	return result
}

func Named(name string) (Lane, error) {
	for _, lane := range registry {
		if lane.Name == name {
			return lane, nil
		}
	}
	return Lane{}, fmt.Errorf("unknown E2E lane %q", name)
}

func supportsAll(backend devnet.Backend, required []devnet.Capability) bool {
	for _, capability := range required {
		if !backend.Supports(capability) {
			return false
		}
	}
	return true
}
