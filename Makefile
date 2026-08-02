.PHONY: test fmt e2e-compile network-image clef-image network-start network-stop e2e-test e2e-execution e2e-consensus e2e-crosslayer e2e-signer e2e-all e2e-core e2e-validator e2e-validator-operations e2e-chaos e2e-scenarios e2e-sync e2e-cold e2e-optimistic

GO ?= go
GO_QRL_SOURCE_DIR ?=
DEVNET_EXECUTION_IMAGE ?= local/go-qrl:devnet
DEVNET_CLEF_IMAGE ?= local/go-qrl-clef:devnet
DEVNET_ENCLAVE_NAME ?=
DEVNET_PROFILE ?= single
DEVNET_START_TIMEOUT ?=
E2E_PACKAGES ?= ./endtoend/suites/...
E2E_SUITE_TIMEOUT ?= 45m
E2E_REPORT_DIR ?= reports
E2E_LABEL_FILTER ?= !scenario-full
override DEVNET_PARAMS_FILE := $(if $(strip $(DEVNET_PARAMS_FILE)),$(abspath $(DEVNET_PARAMS_FILE)))

test:
	$(GO) test ./...

fmt:
	gofmt -s -w $$(find . -name '*.go')

e2e-compile:
	$(GO) test -tags=e2e -run '^$$' ./endtoend/...

network-image:
	@test -n "$(strip $(GO_QRL_SOURCE_DIR))" || { echo "GO_QRL_SOURCE_DIR must point to a go-qrl checkout" >&2; exit 2; }
	docker build --tag "$(DEVNET_EXECUTION_IMAGE)" "$(GO_QRL_SOURCE_DIR)"

clef-image:
	@test -n "$(strip $(GO_QRL_SOURCE_DIR))" || { echo "GO_QRL_SOURCE_DIR must point to a go-qrl checkout" >&2; exit 2; }
	docker build \
		--file endtoend/Dockerfile.clef \
		--build-context go-qrl-source="$(GO_QRL_SOURCE_DIR)" \
		--tag "$(DEVNET_CLEF_IMAGE)" \
		.

network-start: network-image clef-image
	@docker info >/dev/null 2>&1 || { echo "Docker is required and its daemon must be running" >&2; exit 1; }
	@kurtosis version 2>/dev/null | grep -Eq '^CLI Version:[[:space:]]+1\.20\.' || { \
		echo "Kurtosis CLI 1.20.x is required (https://docs.kurtosis.com/upgrade)" >&2; \
		exit 1; \
	}
	kurtosis engine start
	$(GO) run ./devnet/cmd/devnet start \
		--execution-image "$(DEVNET_EXECUTION_IMAGE)" \
		--profile "$(DEVNET_PROFILE)" \
		$(if $(DEVNET_ENCLAVE_NAME),--enclave-name "$(DEVNET_ENCLAVE_NAME)") \
		$(if $(DEVNET_START_TIMEOUT),--timeout "$(DEVNET_START_TIMEOUT)") \
		$(if $(DEVNET_PARAMS_FILE),--params-file "$(DEVNET_PARAMS_FILE)")

network-stop:
	$(GO) run ./devnet/cmd/devnet stop $(if $(DEVNET_ENCLAVE_NAME),--enclave-name "$(DEVNET_ENCLAVE_NAME)")

e2e-test:
	@test -n "$(strip $(GO_QRL_SOURCE_DIR))" || { echo "GO_QRL_SOURCE_DIR must point to a go-qrl checkout" >&2; exit 2; }
	@test -n "$(strip $(E2E_PACKAGES))" || { echo "E2E_PACKAGES must name at least one suite package" >&2; exit 2; }
	@mkdir -p "$(E2E_REPORT_DIR)"
	DEVNET_ENCLAVE_NAME="$(DEVNET_ENCLAVE_NAME)" \
	DEVNET_PROFILE="$(DEVNET_PROFILE)" \
	GO_QRL_SOURCE_DIR="$(GO_QRL_SOURCE_DIR)" \
	$(GO) tool ginkgo \
		--tags=e2e \
		--procs=1 \
		--keep-going \
		--require-suite \
		--fail-on-empty \
		--fail-on-pending \
		--timeout="$(E2E_SUITE_TIMEOUT)" \
		--output-dir="$(abspath $(E2E_REPORT_DIR))" \
		--junit-report=junit.xml \
		--json-report=report.json \
		$(if $(strip $(E2E_LABEL_FILTER)),--label-filter='$(E2E_LABEL_FILTER)') \
		$(strip $(E2E_PACKAGES)) \
		-- -test.run='^TestE2E$$'

e2e-execution: E2E_PACKAGES=./endtoend/suites/execution/...
e2e-execution: e2e-test

e2e-consensus: E2E_PACKAGES=./endtoend/suites/consensus/beaconapi ./endtoend/suites/consensus/validatorapi ./endtoend/suites/consensus/protocol
e2e-consensus: e2e-test

e2e-crosslayer: E2E_PACKAGES=./endtoend/suites/crosslayer/...
e2e-crosslayer: E2E_SUITE_TIMEOUT=3h
e2e-crosslayer: e2e-test

e2e-signer: E2E_PACKAGES=./endtoend/suites/signer/...
e2e-signer: e2e-test

e2e-all: E2E_PACKAGES=./endtoend/suites/...
e2e-all: E2E_SUITE_TIMEOUT=4h
e2e-all: e2e-test

e2e-core: E2E_PACKAGES=./endtoend/suites/execution/... ./endtoend/suites/signer/... ./endtoend/suites/crosslayer/engine ./endtoend/suites/crosslayer/network ./endtoend/suites/crosslayer/transactions
e2e-core: e2e-test

e2e-validator: E2E_PACKAGES=./endtoend/suites/crosslayer/validator
e2e-validator: E2E_SUITE_TIMEOUT=90m
e2e-validator: e2e-test

e2e-validator-operations: E2E_PACKAGES=./endtoend/suites/crosslayer/validator
e2e-validator-operations: E2E_SUITE_TIMEOUT=4h
e2e-validator-operations: E2E_LABEL_FILTER=profile-operations
e2e-validator-operations: e2e-test

e2e-chaos: E2E_PACKAGES=./endtoend/suites/crosslayer/network ./endtoend/suites/crosslayer/resilience ./endtoend/suites/crosslayer/partition
e2e-chaos: E2E_SUITE_TIMEOUT=90m
e2e-chaos: e2e-test

e2e-scenarios: E2E_PACKAGES=./endtoend/suites/crosslayer/... ./endtoend/suites/execution/vm
e2e-scenarios: E2E_SUITE_TIMEOUT=3h
e2e-scenarios: E2E_LABEL_FILTER=scenario && !profile-operations
e2e-scenarios: e2e-test

e2e-sync: E2E_PACKAGES=./endtoend/suites/consensus/sync
e2e-sync: E2E_SUITE_TIMEOUT=45m
e2e-sync: e2e-test

e2e-cold: E2E_PACKAGES=./endtoend/suites/consensus/coldstate
e2e-cold: E2E_SUITE_TIMEOUT=45m
e2e-cold: e2e-test

e2e-optimistic: E2E_PACKAGES=./endtoend/suites/consensus/optimistic
e2e-optimistic: E2E_SUITE_TIMEOUT=45m
e2e-optimistic: e2e-test
