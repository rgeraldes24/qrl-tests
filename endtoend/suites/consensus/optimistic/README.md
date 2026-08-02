# Optimistic Sync Suite

Run this suite with the `optimistic` profile. The secondary execution and
validator clients are stopped while its beacon node remains connected to the
network in Qrysm's startup-optimistic mode. The suite verifies optimistic block
import and subsequent execution validation after the EL returns.
