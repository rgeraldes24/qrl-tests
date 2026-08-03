# Execution Sync

This profile starts a second execution client without discovery or consensus
payload input. The suite creates a transaction, contract, full-width storage
value, log, and proof on the primary client, connects the secondary explicitly,
and verifies exact state before and after restarting the secondary on its
existing database.
