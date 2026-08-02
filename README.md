# QRL Tests

`qrl-tests` owns black-box end-to-end testing across QRL execution and
consensus clients. It starts a pinned Kurtosis development network and runs
QRL-native Ginkgo suites through public RPC, REST, GraphQL, WebSocket, console,
and signer interfaces.

The suites cover QRL network health, workloads, lifecycle operations, failure
recovery, and protocol boundaries without maintaining a second YAML runner.
Unsupported protocol families such as blobs, Pectra/Fusaka/Gloas/Verkle
behavior, EIP-7702, and MEV builders are explicitly excluded. See
[`docs/scenario-coverage.md`](docs/scenario-coverage.md).

## Run

Point the harness at the go-qrl checkout whose image and helper binaries should
be tested:

```bash
export GO_QRL_SOURCE_DIR=/path/to/go-qrl

make test
make e2e-compile
make network-start
make e2e-test
make network-stop
```

Normal runs exclude long `scenario-full` workloads. Run the full QRL network
scenarios against a multi-client network with:

```bash
DEVNET_PROFILE=multi make network-start
make e2e-scenarios
make network-stop
```

Run destructive validator operation workloads on a fresh, larger network:

```bash
DEVNET_PROFILE=operations make network-start
make e2e-validator-operations
make network-stop
```

Select suites with `E2E_PACKAGES`:

```bash
make e2e-test E2E_PACKAGES=./endtoend/suites/network
make e2e-test E2E_PACKAGES='./endtoend/suites/api ./endtoend/suites/console'
```

Network configuration is documented in [`devnet/README.md`](devnet/README.md).
Suite ownership and the client-repository boundary are documented in
[`docs/migration.md`](docs/migration.md).
