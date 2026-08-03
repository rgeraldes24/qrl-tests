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

Run every lane against a fresh matching network profile:

```bash
make e2e-all
```

`e2e-all` provisions and removes each network automatically. Use the individual
`network-start`, suite, and `network-stop` commands when iterating on one lane.
The equivalent Kubernetes entry points are `network-start-k8s` and
`e2e-all-k8s`; they require registry images and a selected Kurtosis cluster with
its gateway running. See [`devnet/README.md`](devnet/README.md).

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

Run consensus API and protocol checks against the current network:

```bash
make e2e-consensus
```

Fresh-database sync, cold-state lookup, and optimistic-sync coverage use
dedicated network profiles:

```bash
DEVNET_PROFILE=sync make network-start
DEVNET_PROFILE=sync make e2e-sync
make network-stop

DEVNET_PROFILE=execution-sync make network-start
DEVNET_PROFILE=execution-sync make e2e-execution-sync
make network-stop

DEVNET_PROFILE=cold make network-start
DEVNET_PROFILE=cold make e2e-cold
make network-stop

DEVNET_PROFILE=optimistic make network-start
DEVNET_PROFILE=optimistic make e2e-optimistic
make network-stop
```

The long-running recovery lane uses the four-participant chaos profile:

```bash
DEVNET_PROFILE=chaos make network-start
make e2e-soak
make network-stop
```

Select suites with `E2E_PACKAGES`:

```bash
make e2e-test E2E_PACKAGES=./endtoend/suites/crosslayer/network
make e2e-test E2E_PACKAGES='./endtoend/suites/execution/api ./endtoend/suites/execution/console'
```

Network configuration is documented in [`devnet/README.md`](devnet/README.md).
Suite ownership and the client-repository boundary are documented in
[`docs/migration.md`](docs/migration.md).
