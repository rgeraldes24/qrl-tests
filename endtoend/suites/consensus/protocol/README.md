# Consensus Protocol Suite

This suite validates the deterministic network's consensus invariants across a
complete epoch: genesis validator state, peers and metrics, execution-data
votes, fee recipients, sync-committee participation, and all consensus
signatures carried by sampled blocks.

It uses only the existing Qrysm REST and metrics endpoints. Missing blocks are
treated as skipped slots, not successful assertions.
