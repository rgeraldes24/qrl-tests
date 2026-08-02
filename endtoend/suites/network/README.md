# Network suite

This read-only suite covers QRL health scenarios with
native Ginkgo specs:

- execution and consensus synchronization;
- consensus block proposals and slot progression;
- finalized consensus state;
- active validator and attestation production;
- monotonic execution head, consensus head, and finality progression.

Run it against the separately started development network:

```bash
make e2e-test E2E_PACKAGES=./endtoend/suites/network
```
