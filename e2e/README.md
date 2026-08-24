# End-to-end suites

The suites run against a [development network](../devnet/README.md). The
runner passes a generated manifest containing the participant endpoints and,
for the console suite, the authoritative execution image; suites do not
provision infrastructure.

## Lanes

| Lane | Profile | Coverage |
| --- | --- | --- |
| `execution` | `single` | Execution ABI plus CLI and embedded-console behavior |

The lane runs these suites in order:

| Suite | Coverage |
| --- | --- |
| `execution-abi` | ABI calls, events, errors, and WebSocket filters |
| `execution-console` | `gqrl attach` and embedded web3 APIs, contract deployment and calls, receipts, event filters, and WebSocket watches |

Run the full lane with a fresh local Docker network using built-in parameters:

```bash
make e2e-run
```

Run the ABI suite against an existing matching network:

```bash
make network-start
E2E_SUITE=execution-abi make e2e
make network-stop
```

The console suite always launches `gqrl attach` from the execution image used
to provision the lane. The container reaches the runner-published endpoints
through Docker's host-gateway mapping, supporting Linux and Docker Desktop with
Linux containers on macOS or Windows. Use WSL2 as the runner environment on
Windows.

The Ginkgo runner writes its JSON report, output log and resolved environment
manifest under `reports/lanes/<lane>/`, next to the run manifest and result
summaries at the report root. Every run records its Ginkgo seed in
`reports/run-manifest.json`; unexpected skipped or pending specs fail the run.
Inspect the registered lane and suites with `go run ./cmd/qrltest list`.

Files that register or execute live scenarios use the `e2e` build tag.
Deterministic fixture, encoding, and helper tests remain untagged so the default
`go test ./...` run continues to validate them without a network.

Build test inputs inside the suite when the building is the behavior under
test; shared client helpers and fixtures belong under `internal/`.
