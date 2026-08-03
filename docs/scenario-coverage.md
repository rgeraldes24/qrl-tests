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

[`scenarios.yaml`](../endtoend/coverage/scenarios.yaml) is the machine-checked
contract for every source scenario. It records
individual covered, missing, failing, and unsupported behaviors and ties each
implemented behavior to an executable Ginkgo label.

## Excluded Task Classes

The QRL suites intentionally do not expose general-purpose shell, JavaScript,
remote-task download, or Spamoor tasks. They are orchestration escape hatches
rather than protocol assertions. Equivalent supported behavior is implemented
as reviewable Go specs.
