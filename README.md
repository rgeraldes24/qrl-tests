# QRL Tests

`qrl-tests` provisions pinned Kurtosis networks and runs end-to-end tests across
the QRL execution and consensus stack.

## Run

```bash
go test ./...
make e2e-run
```

The configured client images must already be available to the selected Kurtosis
backend.

The default lane includes `execution-console`, which requires Linux, the local
Docker backend, and built-in parameters. Other runner modes can select
`E2E_SUITE=execution-abi`.

`e2e-run` provisions a network, runs the selected E2E lane, writes reports under
`reports/`, and removes the network:

```
reports/
├── run-manifest.json         # provenance and replay metadata for the run
├── summary.json              # spec counts and classified failures
├── summary.md                # Markdown result summary
└── lanes/<lane>/
    ├── report.json           # Ginkgo test report
    ├── output.log            # test command output
    ├── manifest.json         # lane, network environment, and console image
    └── diagnostics/          # created only when the lane fails
        ├── diagnostics.json   # capture status and errors
        ├── inspect.txt        # Kurtosis enclave inspection
        └── services/
            └── <service>.log  # log for each discovered service
```

Diagnostics are collected for post-creation start failures and for test or
harness failures after a network is acquired. Runner-owned networks are
diagnosed before cleanup, while attached networks are left running. For
cleanup-only failures, diagnostics are collected after the failed cleanup
attempt. Diagnostic and cleanup errors do not replace the original failure.

For iterative ABI development against an existing network:

```bash
make network-start
E2E_SUITE=execution-abi make e2e
make network-stop
```

The console suite runs `gqrl attach` directly from the configured execution
image. Run it with `E2E_SUITE=execution-console make e2e-run` on Linux with the
Docker backend and built-in parameters.

List registered lanes and suites with:

```bash
go run ./cmd/qrltest list
```

Network provisioning supports Docker and Kubernetes. Kubernetes requires
registry-backed images and an active Kurtosis gateway. The `execution-console`
suite requires a runner-provisioned Linux Docker network using built-in
parameters.

See [development network configuration](devnet/README.md) and the
[end-to-end suites](e2e/README.md).
