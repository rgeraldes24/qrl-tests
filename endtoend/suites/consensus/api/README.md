# Consensus API suite

This live suite validates QRL Beacon, Node, Config, and Validator REST responses
against a running network. It cross-checks head and state roots, validator
counts, common configuration, operation pools, block, attestation, and
sync-committee rewards, and every consensus signature carried by a live block.
Validator coverage checks attester, proposer, and sync committee duties against
the active validator set and verifies exact prior-epoch liveness results.

`coverage_test.go` inventories the HTTP routes exposed by the pinned Qrysm
revision. Each route is classified as live behavior, response shape, or an
explicit exclusion. Cross-layer lifecycle routes are mapped to their direct
submission scenarios; validator runtime routes are marked as indirectly
exercised by the running validators. Direct gRPC and unsupported legacy
`/qrl/v1alpha1` gateway methods are explicitly excluded because the default
devnet profile exposes only the REST gateway. Validator key-management routes
are inventoried but excluded because they mutate the running validator setup.
