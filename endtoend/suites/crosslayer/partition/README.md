# Partition suite

Runs two-way split scenarios with native qrl-tests code. An
ephemeral network-tool container installs packet-drop rules in the existing EL
and CL network namespaces; no fault-injection service is deployed. Cleanup
removes every rule after each spec.

The execution scenarios verify canonical receipt agreement, removal of the
losing receipt, and reinjection of independent transactions mined on an
orphaned branch.

Run with `DEVNET_PROFILE=chaos`.
