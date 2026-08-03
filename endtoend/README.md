# End-to-end suites

These Ginkgo suites validate go-qrl against an already-running
[development network](../devnet/README.md). Suites inspect the network but never
create or destroy it.

## Run

```bash
make network-start
make e2e-test E2E_PACKAGES=./endtoend/suites/...
make network-stop
```

Use the same `DEVNET_ENCLAVE_NAME` for all commands when overriding the default.
`E2E_SUITE_TIMEOUT` controls the Ginkgo execution budget and defaults to `45m`.
Ginkgo continues through all selected suite packages when a suite fails and
writes merged JUnit and JSON reports under `reports/` by default.

The repository exposes profile-oriented lanes:

| Lane | Network profile | Coverage |
| --- | --- | --- |
| `make e2e-core` | `single` | Client APIs, ABI, console, signing, Engine, transactions, and VM |
| `go run ./endtoend/cmd/e2e run workloads` | `multi` | Long transaction-volume and calldata workloads |
| `make e2e-consensus` | `single` or `multi` | Beacon/validator APIs, signatures, protocol state, peers, metrics, and fee recipients |
| `make e2e-validator` | `lifecycle` | Deposits, activation, exits, withdrawals, and slashings |
| `make e2e-validator-operations` | `operations` | Multi-client deposit, exit, and slashing workloads |
| `make e2e-chaos` | `chaos` | Multi-client health, native partitions, outages, restart, and catch-up |
| `make e2e-scenarios` | `multi` | Full QRL workloads, network scenarios, Engine checks, and restart recovery |
| `make e2e-sync` | `sync` | Fresh EL/CL database sync and validator doppelganger protection |
| `make e2e-execution-sync` | `execution-sync` | Fresh execution-client sync plus exact state and restart persistence |
| `make e2e-cold` | `cold` | Historical validator assignments across archived state boundaries |
| `make e2e-optimistic` | `optimistic` | Optimistic consensus sync while the execution client is unavailable, then recovery |
| `make e2e-soak` | `chaos` | Repeated transaction workloads, participant restarts, partitions, and finality recovery |

Start the network with the corresponding `DEVNET_PROFILE` before running an
individual lane. `make e2e-all` instead provisions a fresh matching network for
every registered lane. List that registry with `go run ./endtoend/cmd/e2e list`.

The coverage inventories under [`coverage/`](../coverage) are executable:
repository tests fail when a supported source scenario lacks a matching Ginkgo
label or an API manifest points at a package outside its configured lane.

Suites are also grouped by protocol boundary:

| Domain | Contents | Command |
| --- | --- | --- |
| Execution | ABI, JSON-RPC, GraphQL, console, VM, and precompiles | `make e2e-execution` |
| Consensus | Beacon and validator APIs, signatures, protocol invariants, sync, and historical state | `make e2e-consensus` |
| Cross-layer | Engine, transactions, validators, partitions, and recovery | `make e2e-crosslayer` |
| Signer | Clef and node-to-Clef integration | `make e2e-signer` |

Run every implemented domain with `make e2e-all`.

## Adding a suite

Add suites under `suites/<domain>/<suite>`. Live bootstrap files use the `e2e`
build tag and open `internal/live.Session` for endpoints, execution clients, the
development wallet, and chain ID. Keep network lifecycle management outside the
suites.

The [ABI](suites/execution/abi/README.md),
[API](suites/execution/api/README.md),
[console](suites/execution/console/README.md),
[VM/precompile](suites/execution/vm/README.md),
[Clef](suites/signer/clef/README.md),
[external signer](suites/signer/externalsigner/README.md),
[beacon API](suites/consensus/beaconapi/README.md),
[validator API](suites/consensus/validatorapi/README.md),
[consensus protocol](suites/consensus/protocol/README.md),
[fresh sync](suites/consensus/sync/README.md),
[execution sync](suites/execution/sync/README.md),
[cold state](suites/consensus/coldstate/README.md),
[optimistic sync](suites/consensus/optimistic/README.md),
[Engine](suites/crosslayer/engine/README.md),
[network](suites/crosslayer/network/README.md),
[transactions](suites/crosslayer/transactions/README.md),
[validator](suites/crosslayer/validator/README.md),
[partition](suites/crosslayer/partition/README.md), and
[resilience](suites/crosslayer/resilience/README.md), and
[soak](suites/system/soak/README.md) suites document their
focused commands and coverage.
