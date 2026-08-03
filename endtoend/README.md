# End-to-end suites

The suites run against a separately managed [development network](../devnet/README.md).
The runner passes a generated environment manifest containing participant
endpoints and prebuilt helper binaries; suites do not provision infrastructure.

## Lanes

| Lane | Profile | Coverage |
| --- | --- | --- |
| `single` | `single` | Execution APIs, ABI, console, VM, signing, consensus APIs, protocol, and Engine |
| `multi` | `multi` | Multi-client networking and transactions |
| `workloads` | `multi` | Long transaction-volume and calldata workloads |
| `lifecycle` | `single` | Validator deposits, activation, exits, withdrawals, and slashings |
| `chaos` | `chaos` | Outages, partitions, restart, and recovery |
| `consensus-sync` | `sync` | Fresh consensus sync and validator protection |
| `execution-sync` | `execution-sync` | Fresh execution sync and state persistence |
| `operations` | `operations` | Multi-client validator operation workloads |
| `cold-state` | `cold` | Historical state and assignment lookups |
| `optimistic` | `optimistic` | Optimistic sync and execution recovery |
| `soak` | `chaos` | Repeated workloads and recovery cycles |

Run one lane with a fresh network:

```bash
make e2e-run E2E_LANE=single
```

Run a lane against an existing matching network:

```bash
make network-start DEVNET_PROFILE=single
make e2e E2E_LANE=single
make network-stop
```

The Ginkgo runner continues across suite packages and writes JUnit, JSON, and
the resolved environment manifest under `reports/<lane>/`. Run all registered
lanes with `make e2e-all` or list them with `go run ./cmd/qrl-tests list`.

## Adding a suite

Add the package under `suites/<domain>/<suite>`, register it in
`internal/lanes`, and add the corresponding coverage contract under
`coverage/`. Live bootstrap code uses the `e2e` build tag and
`internal/live.Session` for participant endpoints, clients, the development
wallet, and chain ID.

Files that register or execute live scenarios use the `e2e` build tag.
Deterministic fixture, encoding, and helper tests remain untagged so the default
`go test ./...` run continues to validate them without a network.

Keep construction paths local when they are the behavior under test. Shared
network inspection, process building, fixture data, stability checks, and
client transports belong under `internal/`.
