# QRL Tests

`qrl-tests` owns black-box testing across QRL execution and consensus clients.
It provisions pinned Kurtosis networks and runs QRL-native Ginkgo suites through
public RPC, REST, GraphQL, WebSocket, console, signer, and Engine interfaces.

## Run

Point the harness at the go-qrl checkout used to build the execution image and
helper binaries:

```bash
export GO_QRL_SOURCE_DIR=/path/to/go-qrl

make test
make e2e-compile
make e2e-run E2E_LANE=single
```

The runner verifies that its linked go-qrl module matches
`GO_QRL_SOURCE_DIR` before starting a lane.

`e2e-run` provisions the lane's network profile, runs the lane, and removes the
network. Run every supported lane with:

```bash
make e2e-all
```

For iterative work, keep a network running:

```bash
make network-start DEVNET_PROFILE=single
make e2e E2E_LANE=single
make network-stop
```

List the registered lanes with `go run ./cmd/qrl-tests list`. Reports are
written under `reports/<lane>/`.

## Kubernetes

The same runner supports Kurtosis on Docker and Kubernetes. Kubernetes runs
must use registry-backed images and an active Kurtosis gateway:

```bash
DEVNET_BACKEND=kubernetes \
DEVNET_EXECUTION_IMAGE=registry.example/go-qrl:test \
DEVNET_CLEF_IMAGE=registry.example/go-qrl-clef:test \
make e2e-run E2E_LANE=single
```

Use distinct `DEVNET_ENCLAVE_NAME` and `E2E_REPORT_DIR` values for concurrent
networks. Backend-specific scenarios are selected from declared capabilities;
Docker-only partition tests are omitted on Kubernetes.

See [development network configuration](devnet/README.md), [suite ownership](endtoend/README.md),
[scenario coverage](endtoend/coverage/README.md), and [test ownership](docs/ownership.md).
