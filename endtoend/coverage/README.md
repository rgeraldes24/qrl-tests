# Coverage Inventories

This directory contains the machine-checked contracts for the E2E suite:

- `scenarios.yaml` is the source of truth for source-scenario dispositions and
  maps each supported behavior to a Ginkgo label.
- `docs/scenario-coverage.md` explains the coverage model and links to the
  machine-readable inventory.

`go test ./...` verifies that each supported scenario behavior has executable
coverage selected by an E2E lane.
