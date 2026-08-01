# Partition suite

Runs the Assertoor two-way split scenarios with native qrl-tests code. An
ephemeral network-tool container installs packet-drop rules in the existing EL
and CL network namespaces; no fault-injection service is deployed. Cleanup
removes every rule after each spec.

Run with `DEVNET_PROFILE=chaos`.
