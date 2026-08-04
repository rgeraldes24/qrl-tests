# Engine and cross-layer suite

This suite authenticates directly with each execution client's Engine API and
compares a live beacon execution payload with the corresponding execution
block and Engine payload body. Hash and range body retrieval must return the
same exact payload body. It does not deploy an RPC proxy or observer.
