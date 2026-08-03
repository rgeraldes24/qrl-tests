# Consensus API suite

This live suite validates QRL Beacon, Node, Config, and Validator REST responses
against a running network. It cross-checks head and state roots, validator
counts, common configuration, operation pools, and every consensus signature
carried by a live block. Validator coverage checks attester, proposer, and sync
committee duties against the active validator set and verifies exact prior-epoch
liveness results.
