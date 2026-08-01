# Test Ownership

`qrl-tests` owns black-box network testing across QRL execution and consensus
clients. Client repositories retain tests that require implementation details.

| Repository | Test ownership |
| --- | --- |
| `qrl-tests` | Network lifecycle, cross-client assertions, public RPC/REST/GraphQL/WebSocket behavior, Clef integration, contract fixtures, and topology scenarios |
| `go-qrl` | Unit and package integration tests, QRVM and ABI implementation details, plus production APIs required by external suites |
| `qrysm` | Consensus unit and package integration tests, state-transition internals, plus production APIs required by external suites |

A test that runs against published client images through public interfaces
belongs here. A test that imports a client `internal` package remains with that
client.

The API, ABI, console, Clef, external-signer, VM/precompile, network-health,
and transaction suites are maintained in `endtoend/suites`. Future consensus
lifecycle and disruptive topology scenarios should use the same Ginkgo model;
they should not introduce another orchestration language.
