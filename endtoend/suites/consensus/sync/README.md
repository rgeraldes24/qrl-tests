# Consensus sync suite

Run this suite against the built-in `sync` profile. The secondary beacon node
uses `--force-clear-db`, so restarting it exercises sync from an empty consensus
database. Its validator uses `--enable-doppelganger --force-clear-db`, allowing
the suite to prove through the beacon RPC counter and validator liveness that
recently active keys are refused after local signing history is removed.
