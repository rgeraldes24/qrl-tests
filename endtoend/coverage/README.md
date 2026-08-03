# Coverage Inventories

This directory contains the machine-checked contracts for the E2E suite:

- `source-scenarios.txt` pins the external scenario inventory.
- `source-behaviors.json` maps each supported scenario behavior to a Ginkgo label.
- `execution-rpc.json`, `beacon-rest.json`, `validator-rest.json`, and
  `engine-rpc.json` map API surfaces to the suite package and lane that exercises
  them.

`go test ./...` verifies that referenced packages and lanes exist and that each
supported scenario behavior has executable coverage.
