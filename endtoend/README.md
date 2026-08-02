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
| `make e2e-validator` | `lifecycle` | Deposits, activation, exits, withdrawals, and slashings |
| `make e2e-validator-operations` | `operations` | Multi-client deposit, exit, and slashing workloads |
| `make e2e-chaos` | `chaos` | Multi-client health, native partitions, outages, restart, and catch-up |
| `make e2e-scenarios` | `multi` | Full QRL workloads, network scenarios, Engine checks, and restart recovery |

Start the network with the corresponding `DEVNET_PROFILE` before running a
lane. The scenario coverage inventory is executable: repository tests
fail if a supported source scenario lacks a matching Ginkgo coverage label.

## Adding a suite

Add suites under `suites/<suite>`. Live bootstrap files use the `e2e` build tag
and open `internal/live.Session` for endpoints, clients, the development wallet,
and chain ID. Keep network lifecycle management outside the suites.

The [network](suites/network/README.md),
[transactions](suites/transactions/README.md), [ABI](suites/abi/README.md), [API](suites/api/README.md),
[console](suites/console/README.md), [Clef](suites/clef/README.md),
[external signer](suites/externalsigner/README.md),
[Engine](suites/engine/README.md), [validator](suites/validator/README.md),
[partition](suites/partition/README.md), and [VM/precompile](suites/vm/README.md)
suites document their focused commands and coverage.
