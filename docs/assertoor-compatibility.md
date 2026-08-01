# Assertoor Scenario Coverage

This repository reimplements useful test intent from
[Assertoor](https://github.com/ethpandaops/assertoor) as QRL-native Ginkgo
specs. It does not depend on Assertoor and does not copy its GPL
implementation. This inventory was made against Assertoor commit
`89366be6cb211522249a6d9424adb47631864432`.

Statuses mean:

- **Covered**: implemented as a native Ginkgo spec.
- **Merged**: covered by another QRL scenario because the source variants test
  the same protocol behavior.
- **Excluded**: relies on a protocol feature or execution model QRL does not
  support, or is an arbitrary execution utility rather than a network test.

`Covered` means the QRL replacement is runnable, not that it copies every source
task. The Ginkgo spec is the coverage contract; Ethereum-specific
transaction types, opcodes, and client-matrix assumptions are replaced with
the corresponding supported QRL behavior.

## Native Ginkgo Coverage

| Ginkgo suite | Status | Source behavior |
| --- | --- | --- |
| `network` | Covered | Client health, EL/CL synchronization, finality, block proposals, slot waits, validator attestations, and head progression |
| `vm` | Covered | QRVM opcodes, VM64 boundaries, calls, creation, logs, and precompiles |
| `api` | Covered | JSON-RPC, GraphQL, WebSocket, debug, filters, transaction lookup, and txpool behavior |
| `transactions` | Covered | Deterministic wallet funding and large-calldata transaction inclusion |
| `abi` | Covered | VM64 ABI and generated bindings |
| `console` | Covered | Embedded web3 and console behavior |
| `clef` | Covered | Clef account, signing, rules, and persistence behavior |
| `externalsigner` | Covered | go-qrl to Clef integration |
| `engine` | Covered | Engine authentication, capabilities, payload bodies, and CL/EL payload consistency |
| `resilience` | Covered | Native client stop, restart, outage, and catch-up behavior |
| `validator` | Covered | Deposit, top-up, activation, voluntary exit, execution withdrawal, and slashings |
| `partition` | Covered | Partition, finality stall, competing heads, healing, convergence, and renewed finality |

## Source Scenario Inventory

| Assertoor scenario | Disposition | QRL replacement or reason |
| --- | --- | --- |
| `api-compatibility/gloas-api-check.yaml` | Excluded | Gloas/ePBS API surface is not supported |
| `dev/block-proposal-with-blobs-check.yaml` | Excluded | Blob transactions are not supported |
| `dev/dev-deposits.yaml` | Covered | `endtoend/suites/validator` submits deposits for two distinct validators and verifies their consensus balances |
| `dev/execution-spec-tests-dependencies.yaml` | Excluded | Ethereum execution-spec tooling assumes EVM32/Ethereum protocol behavior |
| `dev/execution-spec-tests-execute.yaml` | Excluded | Ethereum execution-spec tooling assumes EVM32/Ethereum protocol behavior |
| `dev/fund-wallet.yaml` | Covered | `endtoend/suites/transactions` |
| `dev/generate-attestations.yaml` | Covered | `endtoend/suites/network` verifies every active validator participates at least once across a complete three-epoch window; source fault-injection options are outside the default QRL profile |
| `dev/shell-test.yaml` | Excluded | Arbitrary shell execution is not a protocol scenario |
| `dev/synchronized-check.yaml` | Covered | `endtoend/suites/network` |
| `dev/two-way-network-split-non-finality.yaml` | Covered | `endtoend/suites/partition` stalls and restores finality across a balanced split |
| `dev/two-way-network-split-reorg-trigger.yaml` | Covered | `endtoend/suites/partition` creates competing heads and verifies convergence after healing |
| `dev/validator-lifecycle-test.yaml` | Merged | `endtoend/suites/validator` covers supported lifecycle operations and `endtoend/suites/partition` covers non-finality; BLS credential changes are not part of QRL |
| `dev/validator-proposer-slashing-test.yaml` | Merged | `endtoend/suites/validator` submits and verifies proposer and attester slashings |
| `dev/validator-slashing-single.yaml` | Merged | `endtoend/suites/validator` submits one proposer slashing and verifies inclusion |
| `dev/validator-withdrawal-test-v2.yaml` | Excluded | QRL withdrawal credentials are direct 64-byte Q-addresses; the source scenario specifically tests BLS-to-execution credential changes |
| `dev/wait-for-slot.yaml` | Covered | `endtoend/suites/network` |
| `fusaka-dev/kurtosis/analyze-cgc.yaml` | Excluded | Fusaka CGC behavior is not supported |
| `fusaka-dev/kurtosis/cgc-validation-test.yaml` | Excluded | Fusaka CGC behavior is not supported |
| `fusaka-dev/kurtosis/ethconfig-test-with-rpc-call.yaml` | Excluded | EIP-7910/Fusaka behavior is not supported |
| `glamsterdam-dev/execution-sec-tests-sequential.yaml` | Excluded | Glamsterdam execution-spec behavior is not supported |
| `gloas-dev/builder-deposit-spam.yaml` | Excluded | Gloas builder deposits are not supported |
| `gloas-dev/builder-deposit.yaml` | Excluded | Gloas builder deposits are not supported |
| `gloas-dev/builder-lifecycle.yaml` | Excluded | Gloas builder lifecycle is not supported |
| `gloas-dev/builder-prefork-onboard.yaml` | Excluded | Gloas builder onboarding is not supported |
| `gloas-dev/builder-prefork-queuefill.yaml` | Excluded | Gloas builder queues are not supported |
| `gloas-dev/deploy-eip8282-contracts.yaml` | Excluded | EIP-8282 is not supported |
| `gloas-dev/exit-builders.yaml` | Excluded | Gloas builder exits are not supported |
| `gloas-dev/exit-conflict.yaml` | Excluded | Gloas builder/validator conflict behavior is not supported |
| `gloas-dev/prefork-queue-fill-public.yaml` | Excluded | Gloas pending queues are not supported |
| `gloas-dev/prefork-queue-fill.yaml` | Excluded | Gloas pending queues are not supported |
| `gloas-dev/slash-active-validators.yaml` | Excluded | This variant depends on Gloas builder state |
| `gloas-dev/slash-validators.yaml` | Excluded | This variant depends on Gloas builder state |
| `gloas-dev/slashing-exit-conflict.yaml` | Excluded | This variant depends on Gloas builder state |
| `gloas-dev/worst-case-block.yaml` | Excluded | Gloas operation mix is not supported |
| `pectra-dev/blockhash-test-with-rpc-call.yaml` | Excluded | EIP-2935/Pectra behavior is not supported |
| `pectra-dev/eip7002-all.yaml` | Excluded | EL-triggered EIP-7002 requests are not supported |
| `pectra-dev/eip7251-all.yaml` | Excluded | EIP-7251 consolidations are not supported |
| `pectra-dev/execution-spec-tests-sequential.yaml` | Excluded | Pectra execution-spec behavior is not supported |
| `pectra-dev/execution-spec-tests.yaml` | Excluded | Pectra execution-spec behavior is not supported |
| `pectra-dev/kurtosis/33eth-deposit.yaml` | Excluded | Pectra-specific deposit semantics are not supported |
| `pectra-dev/kurtosis/64-el-triggered-consolidation.yaml` | Excluded | EIP-7251 consolidations are not supported |
| `pectra-dev/kurtosis/all.yaml` | Excluded | Aggregate Pectra feature suite is not supported |
| `pectra-dev/kurtosis/blockhash-test.yaml` | Excluded | EIP-2935/Pectra behavior is not supported |
| `pectra-dev/kurtosis/bls-changes.yaml` | Excluded | QRL withdrawal credentials are direct 64-byte Q-addresses and do not use BLS credential-change operations |
| `pectra-dev/kurtosis/consolidion-overflow-tests.yaml` | Excluded | EIP-7251 consolidation queues are not supported |
| `pectra-dev/kurtosis/eip6110-double-deposit.yaml` | Excluded | Pectra/EIP-6110-specific deposit semantics are not supported |
| `pectra-dev/kurtosis/eip7002-lh-bug.yaml` | Excluded | EIP-7002 is not supported |
| `pectra-dev/kurtosis/eip7002-mass-withdrawals.yaml` | Excluded | EIP-7002 is not supported |
| `pectra-dev/kurtosis/eip7251-mass-consolidations.yaml` | Excluded | EIP-7251 is not supported |
| `pectra-dev/kurtosis/eip7702-test.yaml` | Excluded | EIP-7702 transactions are not supported |
| `pectra-dev/kurtosis/eip7702-txpool-invalidation.yaml` | Excluded | EIP-7702 transactions are not supported |
| `pectra-dev/kurtosis/el-triggered-consolidation.yaml` | Excluded | EIP-7251 is not supported |
| `pectra-dev/kurtosis/el-triggered-consolidations-of-consolidations.yaml` | Excluded | EIP-7251 is not supported |
| `pectra-dev/kurtosis/el-triggered-exit.yaml` | Excluded | EIP-7002 is not supported |
| `pectra-dev/kurtosis/el-triggered-withdrawal.yaml` | Excluded | EIP-7002 is not supported |
| `pectra-dev/kurtosis/fillup-all-el-queues-valid.yaml` | Excluded | Pectra EL request queues are not supported |
| `pectra-dev/kurtosis/fillup-all-el-queues.yaml` | Excluded | Pectra EL request queues are not supported |
| `pectra-dev/kurtosis/fillup-consolidation-queue.yaml` | Excluded | EIP-7251 is not supported |
| `pectra-dev/kurtosis/fillup-deposit-queue.yaml` | Excluded | Pectra deposit queue behavior is not supported |
| `pectra-dev/kurtosis/fillup-withdrawal-queue.yaml` | Excluded | EIP-7002 is not supported |
| `pectra-dev/kurtosis/massive-deposit-0x02.yaml` | Excluded | Pectra 0x02 credential behavior is not supported |
| `pectra-dev/kurtosis/massive-deposit.yaml` | Excluded | Pectra-specific deposit behavior is not supported |
| `pectra-dev/kurtosis/topup-deposits.yaml` | Merged | `endtoend/suites/validator` deposits twice for the same validator and verifies the resulting balance and activation |
| `pectra-dev/kurtosis/voluntary-exits.yaml` | Merged | `endtoend/suites/validator` submits a signed voluntary exit and verifies inclusion |
| `pectra-dev/validator-lifecycle-test-v3.yaml` | Excluded | Pectra lifecycle and request queues are not supported |
| `stable/all-opcodes-test.yaml` | Covered | `endtoend/suites/vm`, rewritten for QRVM/VM64 |
| `stable/big-calldata-tx-test.yaml` | Covered | `endtoend/suites/transactions` submits a sustained deterministic calldata workload, retains a 64 KiB boundary transaction, and verifies finality |
| `stable/blob-transactions-test.yaml` | Excluded | Blob transactions are not supported |
| `stable/block-proposal-check.yaml` | Covered | `endtoend/suites/network` |
| `stable/dencun-opcodes-test.yaml` | Excluded | Dencun opcodes are not supported |
| `stable/eoa-transactions-test.yaml` | Covered | `endtoend/suites/transactions` submits QRL dynamic-fee transactions through every execution client and verifies network-wide inclusion; QRL has no legacy transaction type |
| `stable/kurtosis/validator-exit-test.yaml` | Covered | `endtoend/suites/validator` submits a signed exit and verifies the state transition |
| `stable/kurtosis/validator-slashing-test.yaml` | Covered | `endtoend/suites/validator` covers proposer and attester slashings |
| `stable/kurtosis/validator-withdrawal-test.yaml` | Covered | `endtoend/suites/validator` verifies the consensus withdrawal and execution balance transfer |
| `stable/mev-block-proposal-check.yaml` | Excluded | MEV builder/relay flow is not part of the current QRL network |
| `stable/stability-check.yaml` | Covered | `endtoend/suites/network` |
| `stable/validator-lifecycle-test-v2.yaml` | Covered | `endtoend/suites/validator` covers deposit, top-up, activation, proposer and attester slashings, exit, and direct QRL-address withdrawal; BLS credential changes are not part of QRL |
| `verkle-dev/aave-deployment.yaml` | Excluded | Verkle state and Ethereum AAVE deployment are not supported |
| `verkle-dev/verkle-conversion-check.yaml` | Excluded | Verkle state conversion is not supported |

## Excluded Task Classes

The QRL suites intentionally do not expose general-purpose shell, JavaScript,
remote-task download, or Spamoor tasks. They are orchestration escape hatches
rather than protocol assertions. Equivalent supported behavior is implemented
as reviewable Go specs.
