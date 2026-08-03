# Soak

The soak lane runs repeated transaction batches while alternating participant
restarts and balanced network partitions. Every cycle must return all execution
and consensus clients to one finalized state. Set `E2E_SOAK_DURATION` to control
the workload duration; the default is 30 minutes.
