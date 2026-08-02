# Network Scenario Coverage

This repository implements QRL-native Ginkgo scenarios for network health,
transaction workloads, validator lifecycle operations, failure recovery, and
VM execution. The source inventories are pinned at commits
`89366be6cb211522249a6d9424adb47631864432` and
`374b6e40298bd4236ea9ed062141853f1ccd89fb`.

Statuses mean:

- **Full**: every applicable source behavior is implemented.
- **Equivalent**: the complete QRL-native behavior is implemented, with a
  protocol-specific source operation replaced or excluded explicitly.
- **Partial**: useful behavior is executable, but at least one applicable source
  behavior is still missing.
- **Failing**: the scenario is implemented and currently exposes a reproducible
  client or topology failure.
- **Unsupported**: the scenario depends entirely on a protocol feature or
  execution model QRL does not support.

[`behavior-contracts.json`](../internal/scenarios/testdata/behavior-contracts.json)
is the machine-checked contract for every non-unsupported scenario. It records
individual covered, missing, failing, and unsupported behaviors and ties each
implemented behavior to an executable Ginkgo label.

## Native Ginkgo Coverage

| Ginkgo suite | Status | Source behavior |
| --- | --- | --- |
| `network` | Implemented | Client health, EL/CL synchronization, finality, block proposals, slot waits, validator attestations, and head progression |
| `vm` | Implemented | QRVM opcodes, VM64 boundaries, calls, creation, logs, and precompiles |
| `api` | Implemented | JSON-RPC, GraphQL, WebSocket, debug, filters, transaction lookup, and txpool behavior |
| `transactions` | Implemented | Deterministic wallet funding and large-calldata transaction inclusion |
| `abi` | Implemented | VM64 ABI and generated bindings |
| `console` | Implemented | Embedded web3 and console behavior |
| `clef` | Implemented | Clef account, signing, rules, and persistence behavior |
| `externalsigner` | Implemented | go-qrl to Clef integration |
| `engine` | Implemented | Engine authentication, capabilities, payload bodies, and CL/EL payload consistency |
| `resilience` | Implemented | Native client stop, restart, outage, and catch-up behavior |
| `validator` | Implemented | Deposit, top-up, activation, voluntary exit, execution withdrawal, and slashings |
| `partition` | Implemented | Partition, finality stall, competing heads, healing, convergence, and renewed finality |
| `beaconapi` | Implemented | Beacon node, state, configuration, pool, and signature-verification APIs |
| `validatorapi` | Implemented | Attester, proposer, sync-committee duty, and liveness APIs |
| `protocol` | Implemented | Genesis, peer, metrics, execution-data, fee-recipient, sync participation, and signature invariants |
| `sync` | Implemented | Fresh database sync and doppelganger protection |
| `coldstate` | Implemented | Historical validator assignments after archival |
| `optimistic` | Implemented | Native optimistic import and execution-validation recovery |

## Source Scenario Inventory

| Source scenario | Disposition | QRL replacement or reason |
| --- | --- | --- |
| `api-compatibility/gloas-api-check.yaml` | Unsupported | Gloas/ePBS API surface is not supported |
| `dev/block-proposal-with-blobs-check.yaml` | Unsupported | Blob transactions are not supported |
| `dev/dev-deposits.yaml` | Full | `endtoend/suites/crosslayer/validator` submits the configured two distinct deposits and verifies their consensus balances |
| `dev/execution-spec-tests-dependencies.yaml` | Unsupported | Ethereum execution-spec tooling assumes EVM32/Ethereum protocol behavior |
| `dev/execution-spec-tests-execute.yaml` | Unsupported | Ethereum execution-spec tooling assumes EVM32/Ethereum protocol behavior |
| `dev/fund-wallet.yaml` | Equivalent | `endtoend/suites/crosslayer/transactions` verifies a deterministic transfer, receipt, and resulting balance |
| `dev/generate-attestations.yaml` | Equivalent | `endtoend/suites/crosslayer/network` verifies every active validator participates across three epochs; source fault injection is unsupported |
| `dev/shell-test.yaml` | Unsupported | Arbitrary shell execution is not a protocol scenario |
| `dev/synchronized-check.yaml` | Full | `endtoend/suites/crosslayer/network` verifies every execution and consensus client is synchronized |
| `dev/two-way-network-split-non-finality.yaml` | Full | `endtoend/suites/crosslayer/partition` stalls and restores finality across a balanced split |
| `dev/two-way-network-split-reorg-trigger.yaml` | Full | Competing heads form, then every node converges on the same newer finalized checkpoint after healing |
| `dev/validator-lifecycle-test.yaml` | Equivalent | Supported lifecycle operations, 300-validator deposit churn, partial and full withdrawals, proposer matrices, and finality recovery are covered; BLS credential changes are not part of QRL |
| `dev/validator-proposer-slashing-test.yaml` | Equivalent | The operations lane submits 50 proposer slashings through every consensus client and observes inclusion by every proposer pair |
| `dev/validator-slashing-single.yaml` | Full | `endtoend/suites/crosslayer/validator` submits one proposer slashing and verifies inclusion and state transition |
| `dev/validator-withdrawal-test-v2.yaml` | Unsupported | QRL withdrawal credentials are direct 64-byte Q-addresses; the source scenario specifically tests BLS-to-execution credential changes |
| `dev/wait-for-slot.yaml` | Full | `endtoend/suites/crosslayer/network` verifies slot and execution-block progression |
| `fusaka-dev/kurtosis/analyze-cgc.yaml` | Unsupported | Fusaka CGC behavior is not supported |
| `fusaka-dev/kurtosis/cgc-validation-test.yaml` | Unsupported | Fusaka CGC behavior is not supported |
| `fusaka-dev/kurtosis/ethconfig-test-with-rpc-call.yaml` | Unsupported | EIP-7910/Fusaka behavior is not supported |
| `glamsterdam-dev/execution-sec-tests-sequential.yaml` | Unsupported | Glamsterdam execution-spec behavior is not supported |
| `gloas-dev/builder-deposit-spam.yaml` | Unsupported | Gloas builder deposits are not supported |
| `gloas-dev/builder-deposit.yaml` | Unsupported | Gloas builder deposits are not supported |
| `gloas-dev/builder-lifecycle.yaml` | Unsupported | Gloas builder lifecycle is not supported |
| `gloas-dev/builder-prefork-onboard.yaml` | Unsupported | Gloas builder onboarding is not supported |
| `gloas-dev/builder-prefork-queuefill.yaml` | Unsupported | Gloas builder queues are not supported |
| `gloas-dev/deploy-eip8282-contracts.yaml` | Unsupported | EIP-8282 is not supported |
| `gloas-dev/exit-builders.yaml` | Unsupported | Gloas builder exits are not supported |
| `gloas-dev/exit-conflict.yaml` | Unsupported | Gloas builder/validator conflict behavior is not supported |
| `gloas-dev/prefork-queue-fill-public.yaml` | Unsupported | Gloas pending queues are not supported |
| `gloas-dev/prefork-queue-fill.yaml` | Unsupported | Gloas pending queues are not supported |
| `gloas-dev/slash-active-validators.yaml` | Unsupported | This variant depends on Gloas builder state |
| `gloas-dev/slash-validators.yaml` | Unsupported | This variant depends on Gloas builder state |
| `gloas-dev/slashing-exit-conflict.yaml` | Unsupported | This variant depends on Gloas builder state |
| `gloas-dev/worst-case-block.yaml` | Unsupported | Gloas operation mix is not supported |
| `pectra-dev/blockhash-test-with-rpc-call.yaml` | Unsupported | EIP-2935/Pectra behavior is not supported |
| `pectra-dev/eip7002-all.yaml` | Unsupported | EL-triggered EIP-7002 requests are not supported |
| `pectra-dev/eip7251-all.yaml` | Unsupported | EIP-7251 consolidations are not supported |
| `pectra-dev/execution-spec-tests-sequential.yaml` | Unsupported | Pectra execution-spec behavior is not supported |
| `pectra-dev/execution-spec-tests.yaml` | Unsupported | Pectra execution-spec behavior is not supported |
| `pectra-dev/kurtosis/33eth-deposit.yaml` | Unsupported | Pectra-specific deposit semantics are not supported |
| `pectra-dev/kurtosis/64-el-triggered-consolidation.yaml` | Unsupported | EIP-7251 consolidations are not supported |
| `pectra-dev/kurtosis/all.yaml` | Unsupported | Aggregate Pectra feature suite is not supported |
| `pectra-dev/kurtosis/blockhash-test.yaml` | Unsupported | EIP-2935/Pectra behavior is not supported |
| `pectra-dev/kurtosis/bls-changes.yaml` | Unsupported | QRL withdrawal credentials are direct 64-byte Q-addresses and do not use BLS credential-change operations |
| `pectra-dev/kurtosis/consolidion-overflow-tests.yaml` | Unsupported | EIP-7251 consolidation queues are not supported |
| `pectra-dev/kurtosis/eip6110-double-deposit.yaml` | Unsupported | Pectra/EIP-6110-specific deposit semantics are not supported |
| `pectra-dev/kurtosis/eip7002-lh-bug.yaml` | Unsupported | EIP-7002 is not supported |
| `pectra-dev/kurtosis/eip7002-mass-withdrawals.yaml` | Unsupported | EIP-7002 is not supported |
| `pectra-dev/kurtosis/eip7251-mass-consolidations.yaml` | Unsupported | EIP-7251 is not supported |
| `pectra-dev/kurtosis/eip7702-test.yaml` | Unsupported | EIP-7702 transactions are not supported |
| `pectra-dev/kurtosis/eip7702-txpool-invalidation.yaml` | Unsupported | EIP-7702 transactions are not supported |
| `pectra-dev/kurtosis/el-triggered-consolidation.yaml` | Unsupported | EIP-7251 is not supported |
| `pectra-dev/kurtosis/el-triggered-consolidations-of-consolidations.yaml` | Unsupported | EIP-7251 is not supported |
| `pectra-dev/kurtosis/el-triggered-exit.yaml` | Unsupported | EIP-7002 is not supported |
| `pectra-dev/kurtosis/el-triggered-withdrawal.yaml` | Unsupported | EIP-7002 is not supported |
| `pectra-dev/kurtosis/fillup-all-el-queues-valid.yaml` | Unsupported | Pectra EL request queues are not supported |
| `pectra-dev/kurtosis/fillup-all-el-queues.yaml` | Unsupported | Pectra EL request queues are not supported |
| `pectra-dev/kurtosis/fillup-consolidation-queue.yaml` | Unsupported | EIP-7251 is not supported |
| `pectra-dev/kurtosis/fillup-deposit-queue.yaml` | Unsupported | Pectra deposit queue behavior is not supported |
| `pectra-dev/kurtosis/fillup-withdrawal-queue.yaml` | Unsupported | EIP-7002 is not supported |
| `pectra-dev/kurtosis/massive-deposit-0x02.yaml` | Unsupported | Pectra 0x02 credential behavior is not supported |
| `pectra-dev/kurtosis/massive-deposit.yaml` | Unsupported | Pectra-specific deposit behavior is not supported |
| `pectra-dev/kurtosis/topup-deposits.yaml` | Equivalent | `endtoend/suites/crosslayer/validator` deposits twice for the same validator and verifies balance and activation |
| `pectra-dev/kurtosis/voluntary-exits.yaml` | Full | The operations lane signs, submits, includes, and applies exactly 64 voluntary exits |
| `pectra-dev/validator-lifecycle-test-v3.yaml` | Unsupported | Pectra lifecycle and request queues are not supported |
| `stable/all-opcodes-test.yaml` | Equivalent | The QRL-native VM suite covers QRVM/VM64 opcodes and precompiles, plus mined deployment, state-changing execution, and terminal receipt status |
| `stable/big-calldata-tx-test.yaml` | Full | The full lane submits 1000 transactions with 1000-byte calldata at no more than 10 workload transactions per block and verifies finality |
| `stable/blob-transactions-test.yaml` | Unsupported | Blob transactions are not supported |
| `stable/block-proposal-check.yaml` | Full | `endtoend/suites/crosslayer/network` observes a proposal from every validator client pair |
| `stable/dencun-opcodes-test.yaml` | Unsupported | Dencun opcodes are not supported |
| `stable/eoa-transactions-test.yaml` | Equivalent | The full lane sustains 10 dynamic-fee transactions per block through every execution client until every validator pair proposes a transaction-bearing block; legacy transactions are unsupported |
| `stable/kurtosis/validator-exit-test.yaml` | Equivalent | The operations lane distributes 64 exits through every consensus client and observes inclusion by every proposer pair |
| `stable/kurtosis/validator-slashing-test.yaml` | Equivalent | The operations lane submits 50 proposer and 50 attester slashings through every consensus client and observes both types from every proposer pair |
| `stable/kurtosis/validator-withdrawal-test.yaml` | Equivalent | `endtoend/suites/crosslayer/validator` verifies QRL direct-address withdrawal and execution balance transfer |
| `stable/mev-block-proposal-check.yaml` | Unsupported | MEV builder/relay flow is not part of the current QRL network |
| `stable/stability-check.yaml` | Full | The network lane enforces synchronization, finality, target/head participation, reorg, and fork budgets |
| `stable/validator-lifecycle-test-v2.yaml` | Equivalent | The operations lane runs the ten-validator mixed lifecycle with QRL direct-address withdrawals; BLS credential changes are not part of QRL |
| `verkle-dev/aave-deployment.yaml` | Unsupported | Verkle state and Ethereum AAVE deployment are not supported |
| `verkle-dev/verkle-conversion-check.yaml` | Unsupported | Verkle state conversion is not supported |

## Excluded Task Classes

The QRL suites intentionally do not expose general-purpose shell, JavaScript,
remote-task download, or Spamoor tasks. They are orchestration escape hatches
rather than protocol assertions. Equivalent supported behavior is implemented
as reviewable Go specs.
