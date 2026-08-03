# Coverage Inventories

This directory contains the machine-checked contracts for the E2E suite:

- `scenarios.yaml` is the source of truth for source-scenario dispositions and
  maps each supported behavior to a Ginkgo label.
- `docs/scenario-coverage.md` contains the generated source scenario inventory.
- `surfaces.yaml` maps API surfaces to the suite package and lane that exercises
  them.

`go test ./...` verifies that referenced packages and lanes exist and that each
supported scenario behavior has executable coverage.

Run `go generate ./endtoend/coverage` after changing `scenarios.yaml`.
