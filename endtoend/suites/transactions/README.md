# Transaction suite

This suite implements the supported Assertoor transaction scenarios directly
as Ginkgo specs:

- fund a deterministic address and verify its balance and receipt;
- submit 64 KiB of deterministic calldata and verify the included payload.

Run it against the separately started development network:

```bash
make e2e-test E2E_PACKAGES=./endtoend/suites/transactions
```
