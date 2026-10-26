# End-to-end suites

The suites run against a [development network](../devnet/README.md). The
runner passes a generated manifest containing the participant
endpoints; suites do not provision infrastructure.

## Lanes

| Lane | Profile | Coverage |
| --- | --- | --- |
| `execution` | `single` | Execution client: contract ABI, gqrl console, and transaction pool |
| `consensus-staking-automated` | `single` | Staking through contract deposits and the keymanager API |
| `consensus-staking-operator` | `single` | Staking through the deposit and validator CLIs |
| `consensus-remote-signer` | `remote-signer` | Validator duties signed by go-remotesigner: its keys and chain, finality, its metrics, and refused slashable requests; needs locally built images |
| `resilience-network-restart` | `anchored` | Recovery from a full outage: proposing and finalizing after every node stops at once |
| `sync` | `single` | A new full-mode node syncs from genesis, then catches up after a stop |

Run one lane with a fresh network:

```bash
make e2e-run
```

Run a lane against an existing matching network:

```bash
make network-start
make e2e
make network-stop
```

`resilience-network-restart` stops and restarts every service, so it runs only
on a network the runner provisions (`make e2e-run` or `run-all`), without a
parameters file.

## Remote signer lane

`consensus-remote-signer` runs a validator client that holds no keys: every
duty is signed by go-remotesigner, which loads the validator client's
keystores, over TLS with a bearer token. The suite checks that the signer
serves exactly the validator client's keys and is bound to its chain, that the
validator client has no keystore, that the chain finalizes, that the signer's
metrics show every duty type signed, a RANDAO reveal for every block and no
refusal or fault, that it refuses a double vote and a double proposal sent to
it directly, and that the chain keeps finalizing afterwards. When a spec
fails, the signer's metrics, healthcheck and capabilities, and the validator
client's remote signer metrics, are saved under
`reports/lanes/consensus-remote-signer/remote-signer/<spec>/`.

Its images are not published yet, so CI does not run the lane, `run-all`
leaves it out, and `qrltest list` marks it. Build them from local checkouts, the validator
client and beacon node from one qrysm commit with the remote signer client:

```bash
REMOTESIGNER_SOURCE_DIR=../go-remotesigner make remote-signer-image
QRYSM_SOURCE_DIR=../qrysm QRYSM_REF=<commit> make qrysm-images
```

The scripts build the working tree of the checkout, or the commit
`REMOTESIGNER_REF` or `QRYSM_REF` names. The signer image is built with
go-remotesigner's own Dockerfile and needs `go-licenses` v2; the qrysm images
are static `go build`s, without Bazel. Then run the lane with them:

```bash
DEVNET_CONSENSUS_IMAGE=local/qrysm:remote-signer-beacon-chain \
DEVNET_VALIDATOR_IMAGE=local/qrysm:remote-signer-validator \
E2E_LANE=consensus-remote-signer make e2e-run
```

The lane also needs a qrl-package that runs go-remotesigner. Until the pinned
package does, start the network from a qrl-package checkout with the
parameters of the `remote-signer` profile, and run the lane against it:

```bash
go run ./cmd/qrltest network params --profile remote-signer \
	--consensus-image local/qrysm:remote-signer-beacon-chain \
	--validator-image local/qrysm:remote-signer-validator > remote-signer.json
kurtosis run --enclave go-qrl-devnet ../qrl-package --args-file remote-signer.json
E2E_LANE=consensus-remote-signer make e2e
make network-stop
```

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
