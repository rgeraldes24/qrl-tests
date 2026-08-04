// Copyright 2026 The qrl-tests Authors
// This file is part of qrl-tests.

package api

import (
	"strings"
	"testing"

	"github.com/cyyber/qrl-tests/devnet"
)

const (
	qrysmRouteRevision = "8b80fa0c3f5a"

	consensusCoverageBehavior = "live behavior"
	consensusCoverageShape    = "live response shape"
	consensusCoverageProfile  = "excluded: not exposed by the devnet profile"
	consensusCoverageUnsafe   = "excluded: mutates node configuration"
	consensusCoverageVCConfig = "excluded: mutates validator configuration"
	consensusCoverageBuilder  = "excluded: builder mode is disabled"
	consensusCoverageBinary   = "excluded: binary SSZ response is not consumed by the current profile"
	consensusCoverageLegacy   = "excluded: unsupported legacy gateway endpoint"
	consensusCoverageObsolete = "excluded: deprecated endpoint"
	consensusCoverageIndirect = "excluded: exercised indirectly by running validators"

	scenarioConsensusMetadata  = "consensus-metadata"
	scenarioConsensusState     = "consensus-state"
	scenarioConsensusPools     = "consensus-pools"
	scenarioConsensusEvents    = "consensus-events"
	scenarioConsensusRewards   = "consensus-rewards"
	scenarioValidatorDuties    = "validator-duties"
	scenarioValidatorLiveness  = "validator-liveness"
	scenarioLegacyParity       = "legacy-gateway-parity"
	scenarioValidatorLifecycle = "validator-lifecycle"
)

type consensusCoverageEntry struct {
	kind     string
	scenario string
}

func consensusBehavior(scenario string) consensusCoverageEntry {
	return consensusCoverageEntry{kind: consensusCoverageBehavior, scenario: scenario}
}

func consensusShape(scenario string) consensusCoverageEntry {
	return consensusCoverageEntry{kind: consensusCoverageShape, scenario: scenario}
}

func consensusExcluded(kind string) consensusCoverageEntry {
	return consensusCoverageEntry{kind: kind}
}

var consensusScenarioDescriptions = map[string]string{
	scenarioConsensusMetadata:  "cross-checks node, genesis, deposit, and configuration metadata",
	scenarioConsensusState:     "cross-checks canonical blocks, headers, state, validators, and committees",
	scenarioConsensusPools:     "checks the live operation-pool response shapes",
	scenarioConsensusEvents:    "observes live head, block, attestation, finality, and reorg events",
	scenarioConsensusRewards:   "validates block, attestation, and sync-committee reward responses",
	scenarioValidatorDuties:    "cross-checks attester, proposer, and sync-committee duties",
	scenarioValidatorLiveness:  "checks prior-epoch validator liveness",
	scenarioLegacyParity:       "compares supported legacy gateway results with standard REST results",
	scenarioValidatorLifecycle: "submits deposits, exits, slashings, and withdrawals through cross-layer suites",
}

// consensusAPICoverage inventories the HTTP surface exposed by the pinned
// Qrysm image. Query strings are omitted from route keys.
var consensusAPICoverage = map[string]consensusCoverageEntry{
	"GET /qrl/v1/node/health":          consensusBehavior(scenarioConsensusMetadata),
	"GET /qrl/v1/node/identity":        consensusBehavior(scenarioConsensusMetadata),
	"GET /qrl/v1/node/peer_count":      consensusBehavior(scenarioConsensusMetadata),
	"GET /qrl/v1/node/peers":           consensusBehavior(scenarioConsensusMetadata),
	"GET /qrl/v1/node/peers/{peer_id}": consensusExcluded(consensusCoverageProfile),
	"GET /qrl/v1/node/syncing":         consensusBehavior(scenarioConsensusMetadata),
	"GET /qrl/v1/node/version":         consensusBehavior(scenarioConsensusMetadata),

	"GET /qrl/v1/beacon/genesis":          consensusBehavior(scenarioConsensusMetadata),
	"GET /qrl/v1/config/deposit_contract": consensusBehavior(scenarioConsensusMetadata),
	"GET /qrl/v1/config/fork_schedule":    consensusBehavior(scenarioConsensusMetadata),
	"GET /qrl/v1/config/spec":             consensusBehavior(scenarioConsensusMetadata),

	"GET /qrl/v1/beacon/blocks/{block_id}":                           consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/blocks/{block_id}/attestations":              consensusShape(scenarioConsensusState),
	"GET /qrl/v1/beacon/blocks/{block_id}/root":                      consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/blocks/{block_id}/ssz":                       consensusExcluded(consensusCoverageBinary),
	"GET /qrl/v1/beacon/headers":                                     consensusShape(scenarioConsensusState),
	"GET /qrl/v1/beacon/headers/{block_id}":                          consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/states/{state_id}/committees":                consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/states/{state_id}/finality_checkpoints":      consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/states/{state_id}/fork":                      consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/states/{state_id}/randao":                    consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/states/{state_id}/root":                      consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/states/{state_id}/sync_committees":           consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/states/{state_id}/validator_balances":        consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/states/{state_id}/validator_count":           consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/states/{state_id}/validators":                consensusBehavior(scenarioConsensusState),
	"GET /qrl/v1/beacon/states/{state_id}/validators/{validator_id}": consensusBehavior(scenarioConsensusState),

	"GET /qrl/v1/beacon/pool/attestations":       consensusShape(scenarioConsensusPools),
	"GET /qrl/v1/beacon/pool/attester_slashings": consensusShape(scenarioConsensusPools),
	"GET /qrl/v1/beacon/pool/proposer_slashings": consensusShape(scenarioConsensusPools),
	"GET /qrl/v1/beacon/pool/voluntary_exits":    consensusShape(scenarioConsensusPools),
	"GET /qrl/v1/events":                         consensusBehavior(scenarioConsensusEvents),

	"GET /qrl/v1/beacon/rewards/blocks/{block_id}":          consensusBehavior(scenarioConsensusRewards),
	"POST /qrl/v1/beacon/rewards/attestations/{epoch}":      consensusBehavior(scenarioConsensusRewards),
	"POST /qrl/v1/beacon/rewards/sync_committee/{block_id}": consensusBehavior(scenarioConsensusRewards),

	"POST /qrl/v1/validator/duties/attester/{epoch}": consensusBehavior(scenarioValidatorDuties),
	"GET /qrl/v1/validator/duties/proposer/{epoch}":  consensusBehavior(scenarioValidatorDuties),
	"POST /qrl/v1/validator/duties/sync/{epoch}":     consensusBehavior(scenarioValidatorDuties),
	"POST /qrl/v1/validator/liveness/{epoch}":        consensusBehavior(scenarioValidatorLiveness),

	"POST /qrl/v1/beacon/pool/attester_slashings": consensusBehavior(scenarioValidatorLifecycle),
	"POST /qrl/v1/beacon/pool/proposer_slashings": consensusBehavior(scenarioValidatorLifecycle),
	"POST /qrl/v1/beacon/pool/voluntary_exits":    consensusBehavior(scenarioValidatorLifecycle),

	"POST /qrl/v1/beacon/blocks":                            consensusExcluded(consensusCoverageIndirect),
	"POST /qrl/v1/beacon/pool/attestations":                 consensusExcluded(consensusCoverageIndirect),
	"POST /qrl/v1/beacon/pool/sync_committees":              consensusExcluded(consensusCoverageIndirect),
	"GET /qrl/v1/validator/aggregate_attestation":           consensusExcluded(consensusCoverageIndirect),
	"POST /qrl/v1/validator/aggregate_and_proofs":           consensusExcluded(consensusCoverageIndirect),
	"GET /qrl/v1/validator/attestation_data":                consensusExcluded(consensusCoverageIndirect),
	"POST /qrl/v1/validator/beacon_committee_subscriptions": consensusExcluded(consensusCoverageIndirect),
	"POST /qrl/v1/validator/contribution_and_proofs":        consensusExcluded(consensusCoverageIndirect),
	"POST /qrl/v1/validator/prepare_beacon_proposer":        consensusExcluded(consensusCoverageIndirect),
	"POST /qrl/v1/validator/register_validator":             consensusExcluded(consensusCoverageIndirect),
	"GET /qrl/v1/validator/sync_committee_contribution":     consensusExcluded(consensusCoverageIndirect),
	"POST /qrl/v1/validator/sync_committee_subscriptions":   consensusExcluded(consensusCoverageIndirect),
	"GET /qrl/v1/validator/blocks/{slot}":                   consensusExcluded(consensusCoverageIndirect),
	"GET /qrl/v1/validator/blocks/{slot}/ssz":               consensusExcluded(consensusCoverageBinary),
	"GET /qrl/v1/validator/blinded_blocks/{slot}":           consensusExcluded(consensusCoverageBuilder),
	"GET /qrl/v1/validator/blinded_blocks/{slot}/ssz":       consensusExcluded(consensusCoverageBinary),
	"GET /qrl/v3/validator/blocks/{slot}":                   consensusExcluded(consensusCoverageIndirect),

	"GET /qrl/v1/keystores":                          consensusExcluded(consensusCoverageVCConfig),
	"POST /qrl/v1/keystores":                         consensusExcluded(consensusCoverageVCConfig),
	"DELETE /qrl/v1/keystores":                       consensusExcluded(consensusCoverageVCConfig),
	"GET /qrl/v1/validator/{pubkey}/feerecipient":    consensusExcluded(consensusCoverageVCConfig),
	"POST /qrl/v1/validator/{pubkey}/feerecipient":   consensusExcluded(consensusCoverageVCConfig),
	"DELETE /qrl/v1/validator/{pubkey}/feerecipient": consensusExcluded(consensusCoverageVCConfig),
	"GET /qrl/v1/validator/{pubkey}/gas_limit":       consensusExcluded(consensusCoverageVCConfig),
	"POST /qrl/v1/validator/{pubkey}/gas_limit":      consensusExcluded(consensusCoverageVCConfig),
	"DELETE /qrl/v1/validator/{pubkey}/gas_limit":    consensusExcluded(consensusCoverageVCConfig),
	"POST /qrl/v1/validator/{pubkey}/voluntary_exit": consensusExcluded(consensusCoverageVCConfig),

	"GET /qrl/v1/builder/states/{state_id}/expected_withdrawals": consensusExcluded(consensusCoverageBuilder),
	"GET /qrl/v1/beacon/blinded_blocks/{block_id}":               consensusExcluded(consensusCoverageBuilder),
	"GET /qrl/v1/beacon/blinded_blocks/{block_id}/ssz":           consensusExcluded(consensusCoverageBinary),
	"POST /qrl/v1/beacon/blinded_blocks":                         consensusExcluded(consensusCoverageBuilder),
	"GET /qrl/v1/beacon/weak_subjectivity":                       consensusExcluded(consensusCoverageObsolete),

	"GET /qrysm/node/trusted_peers":              consensusExcluded(consensusCoverageUnsafe),
	"POST /qrysm/node/trusted_peers":             consensusExcluded(consensusCoverageUnsafe),
	"DELETE /qrysm/node/trusted_peers/{peer_id}": consensusExcluded(consensusCoverageUnsafe),
	"POST /qrysm/validators/performance":         consensusExcluded(consensusCoverageProfile),

	"GET /qrl/v1/debug/beacon/heads":                 consensusExcluded(consensusCoverageProfile),
	"GET /qrl/v1/debug/beacon/states/{state_id}":     consensusExcluded(consensusCoverageProfile),
	"GET /qrl/v1/debug/beacon/states/{state_id}/ssz": consensusExcluded(consensusCoverageProfile),
	"GET /qrl/v1/debug/fork_choice":                  consensusExcluded(consensusCoverageProfile),

	"GET /qrl/v1alpha1/validators/assignments":                           consensusBehavior(scenarioLegacyParity),
	"GET /qrl/v1alpha1/validators/participation":                         consensusBehavior(scenarioLegacyParity),
	"GET /qrl/v1alpha1/beacon/attestations":                              consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/beacon/attestations/indexed":                      consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/beacon/attestations/pool":                         consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/beacon/chainhead":                                 consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/beacon/committees":                                consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/beacon/config":                                    consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/beacon/individual_votes":                          consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/beacon/slashings/attester/submit":                 consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/beacon/slashings/proposer/submit":                 consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha2/beacon/blocks":                                    consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validators":                                       consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator":                                        consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validators/activesetchanges":                      consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validators/balances":                              consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validators/performance":                           consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/node/execution/connections":                       consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/node/genesis":                                     consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/node/p2p":                                         consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/node/peer":                                        consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/node/peers":                                       consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/node/services":                                    consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/node/syncing":                                     consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/node/version":                                     consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/debug/block":                                      consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/debug/peer":                                       consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/debug/peers":                                      consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/debug/state":                                      consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/debug/logging":                                   consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/activation/stream":                      consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/attestation":                            consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/validator/attestation":                           consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/validator/aggregate":                             consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/validator/blocks/assign_validator_to_subnet":     consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/blocks/signatures_and_aggregation_bits": consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/blocks/stream":                          consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/chainstart/stream":                      consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/validator/contribution_and_proof":                consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/doppelganger":                           consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/domain":                                 consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/duties":                                 consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/validator/exit":                                  consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/validator/fee_recipient_by_pub_key":              consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/index":                                  consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/validator/prepare_beacon_proposer":               consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/validator/registration":                          consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/validator/signed_contribution_and_proof":         consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/status":                                 consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/statuses":                               consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/validator/subnet/subscribe":                      consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha1/validator/sync_message":                          consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/validator/sync_message_block_root":                consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha1/sync_subcommittee_index":                          consensusExcluded(consensusCoverageLegacy),
	"GET /qrl/v1alpha2/validator/block":                                  consensusExcluded(consensusCoverageLegacy),
	"POST /qrl/v1alpha2/validator/block":                                 consensusExcluded(consensusCoverageLegacy),
}

var consensusTransportCoverage = map[string]consensusCoverageEntry{
	"standard REST gateway versus direct gRPC": consensusExcluded(consensusCoverageProfile),
	"legacy REST gateway versus direct gRPC":   consensusExcluded(consensusCoverageProfile),
}

func TestConsensusAPICoverageManifest(t *testing.T) {
	if !strings.Contains(devnet.DefaultConsensusImage, qrysmRouteRevision) ||
		!strings.Contains(devnet.DefaultValidatorImage, qrysmRouteRevision) {
		t.Fatalf("coverage manifest targets Qrysm %s, images are %q and %q",
			qrysmRouteRevision, devnet.DefaultConsensusImage, devnet.DefaultValidatorImage)
	}
	for endpoint, entry := range consensusAPICoverage {
		method, path, ok := strings.Cut(endpoint, " ")
		if !ok || method == "" || !strings.HasPrefix(path, "/") {
			t.Errorf("invalid endpoint key %q", endpoint)
		}
		validateConsensusCoverageEntry(t, endpoint, entry)
	}
	for surface, entry := range consensusTransportCoverage {
		validateConsensusCoverageEntry(t, surface, entry)
	}
}

func validateConsensusCoverageEntry(t *testing.T, endpoint string, entry consensusCoverageEntry) {
	t.Helper()
	if strings.TrimSpace(entry.kind) == "" {
		t.Errorf("%s has no coverage category", endpoint)
	}
	if strings.HasPrefix(entry.kind, "live ") {
		if _, ok := consensusScenarioDescriptions[entry.scenario]; !ok {
			t.Errorf("%s references unknown live scenario %q", endpoint, entry.scenario)
		}
	} else if entry.scenario != "" {
		t.Errorf("%s is excluded but references scenario %q", endpoint, entry.scenario)
	}
}
