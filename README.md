# QRL Tests

`qrl-tests` owns black-box end-to-end testing across QRL execution and
consensus clients. It starts a pinned Kurtosis development network and runs
QRL-native Ginkgo suites through public RPC, REST, GraphQL, WebSocket, console,
and signer interfaces.

The suites cover the supported intent from the Assertoor scenario catalog
without embedding Assertoor or maintaining a second YAML runner. Ethereum-only
features such as blobs, Pectra/Fusaka/Gloas/Verkle behavior, EIP-7702, MEV
builders, and Spamoor are explicitly excluded. See
[`docs/assertoor-compatibility.md`](docs/assertoor-compatibility.md).

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

Select suites with `E2E_PACKAGES`:

```bash
make e2e-test E2E_PACKAGES=./endtoend/suites/network
make e2e-test E2E_PACKAGES='./endtoend/suites/api ./endtoend/suites/console'
```

Network configuration is documented in [`devnet/README.md`](devnet/README.md).
Suite ownership and the client-repository boundary are documented in
[`docs/migration.md`](docs/migration.md).
