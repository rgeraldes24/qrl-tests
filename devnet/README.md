# Development network

This directory provides a reusable package and CLI for a separately managed,
Kurtosis-backed QRL development network. It requires Docker and Kurtosis CLI
1.20.x.

## Run

```bash
make network-start
make network-stop
```

`network-start` builds the local go-qrl and Clef images, runs the pinned
qrl-package, and waits for readiness. It does not run the test suites.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `DEVNET_ENCLAVE_NAME` | `go-qrl-devnet` (CLI default) | Kurtosis enclave |
| `DEVNET_EXECUTION_IMAGE` | `local/go-qrl:devnet` | Tag for the locally built execution image |
| `DEVNET_PROFILE` | `single` | Built-in `single`, `multi`, `lifecycle`, `chaos`, `sync`, `execution-sync`, `operations`, `cold`, or `optimistic` profile |
| `DEVNET_START_TIMEOUT` | `30m` (CLI default) | Network startup budget |
| `DEVNET_PARAMS_FILE` | unset | Complete qrl-package JSON parameters |

`DEVNET_ENCLAVE_NAME` is optional. Without it, every command uses
`go-qrl-devnet`. Set it only to use another enclave name, and use the same value
for each command in that lifecycle:

```bash
DEVNET_ENCLAVE_NAME=my-devnet make network-start
DEVNET_ENCLAVE_NAME=my-devnet make network-stop
```

Kurtosis restricts enclave names to letters, digits, and dashes. Operations using the same name
must run serially. Concurrent networks need different names; concurrent builds
from different source trees also need different `DEVNET_EXECUTION_IMAGE` tags.

## Custom parameters

`DEVNET_PARAMS_FILE` replaces the selected built-in profile with a complete
qrl-package JSON argument object. Two exact JSON string tokens are substituted:

```text
__DEVNET_EXECUTION_IMAGE__
__DEVNET_WALLET_ADDRESS__
```

The first participant's `el_image` must use the image token.
`network_params.prefunded_accounts` must contain the wallet token as a key; the
wallet token may also be used as a value, such as `withdrawal_address`.

For example, save the following as `devnet-params.json`:

```json
{
  "participants": [
    {
      "el_image": "__DEVNET_EXECUTION_IMAGE__",
      "el_extra_params": ["--graphql", "--graphql.vhosts=*"],
      "cl_image": "qrledger/qrysm:beacon-chain-8b80fa0c3f5a",
      "cl_extra_params": ["--min-sync-peers=0", "--minimum-peers-per-subnet=0"],
      "vc_image": "qrledger/qrysm:validator-8b80fa0c3f5a"
    }
  ],
  "network_params": {
    "network_id": "1337",
    "seconds_per_slot": 5,
    "execution_follow_distance": 8,
    "prefunded_accounts": {
      "__DEVNET_WALLET_ADDRESS__": {
        "balance": "2000000QRL"
      }
    },
    "withdrawal_address": "__DEVNET_WALLET_ADDRESS__",
    "light_kdf_enabled": true
  },
  "qrl_genesis_generator_params": {
    "image": "qrledger/qrysm:qrl-genesis-generator-360410c72353-8b80fa0c3f5a"
  }
}
```

Start the network with the custom parameters:

```bash
DEVNET_PARAMS_FILE=devnet-params.json make network-start
```

The controller discovers every execution, consensus, and validator participant
from qrl-package service labels. Existing consumers can use the primary-node
endpoint aliases, while multi-node suites use `Environment.Participants`.
The reported GraphQL URL is live only if the profile enables GraphQL on the RPC
port. Readiness requires advancing blocks and a funded development wallet.

Most built-in profiles allocate 64 genesis validators. `multi` and `chaos`
split them across four client pairs, `sync` and `optimistic` split them across
two, and `lifecycle` and `cold` provide dedicated single-client lanes.
`execution-sync` starts a producing source client and an isolated empty
execution client that is connected during the test. The
destructive `operations` profile starts 512 validators across four active
client pairs and assigns 300 initially inactive keys to a fifth validator
client. This supports large deposit churn, proposer distribution, exits, and
slashings while preserving a recoverable network.

## Consumers

Go tooling can import `github.com/cyyber/qrl-tests/devnet` and call
`devnet.Inspect(ctx)` to discover the live execution RPC, GraphQL, WebSocket,
and consensus REST endpoints. The separately maintained
[end-to-end suites](../endtoend/README.md) are one consumer.

## Safety

The built-in profile funds a published development address used by readiness
checks and the migrated live suites. Its matching test seed is maintained by
the suite that signs transactions. Never fund or use this account outside
disposable local development networks.

After a failed start, run `make network-stop` with the same enclave name before
retrying.
