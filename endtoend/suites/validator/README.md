# Validator suite

This suite drives the supported QRL validator lifecycle directly through the
deposit contract and consensus REST APIs. It covers deposit, top-up,
activation, voluntary exit, execution withdrawal, proposer slashing, and
attester slashing. Ethereum BLS credential-change and EL-triggered request
playbooks remain explicit protocol exclusions.

The `profile-operations` lane uses a fresh 512-validator, four-participant
network. It runs a ten-validator mixed lifecycle, exactly 64 voluntary exits,
and 50 proposer plus 50 attester slashings while verifying operation submission
and inclusion across every client pair.
