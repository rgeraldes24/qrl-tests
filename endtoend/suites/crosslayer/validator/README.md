# Validator suite

This suite drives the supported QRL validator lifecycle directly through the
deposit contract and consensus REST APIs. It covers deposit, top-up,
activation, voluntary exit, execution withdrawal, proposer slashing, and
attester slashing. Ethereum BLS credential-change and EL-triggered request
playbooks remain explicit protocol exclusions.

The `profile-operations` lane uses a fresh network with 512 genesis validators
across four active participants and 300 initially inactive validators assigned
to a fifth participant. It runs a mixed lifecycle with partial and full
withdrawals, exactly 64 voluntary exits, 50 proposer and 50 attester slashings,
and a 300-validator deposit/finality-recovery workload. Generated operation and
deposit signatures are verified before submission, and live block signatures
are verified after production.
