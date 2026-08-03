.PHONY: test fmt e2e-compile network-image clef-image network-preflight network-start network-start-k8s network-start-configured network-stop e2e-test e2e-execution e2e-consensus e2e-crosslayer e2e-signer e2e-all e2e-all-k8s e2e-all-configured e2e-core e2e-validator e2e-validator-operations e2e-chaos e2e-scenarios e2e-sync e2e-execution-sync e2e-cold e2e-optimistic e2e-soak

GO ?= go
GO_QRL_SOURCE_DIR ?=
DEVNET_BACKEND ?= docker
DEVNET_EXECUTION_IMAGE ?= local/go-qrl:devnet
DEVNET_CLEF_IMAGE ?= local/go-qrl-clef:devnet
DEVNET_CONSENSUS_IMAGE ?=
DEVNET_VALIDATOR_IMAGE ?=
DEVNET_GENESIS_IMAGE ?=
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

network-preflight:
	@case "$(DEVNET_BACKEND)" in \
		docker) docker info >/dev/null 2>&1 || { echo "Docker is required and its daemon must be running" >&2; exit 1; } ;; \
		kubernetes) case "$(DEVNET_EXECUTION_IMAGE) $(DEVNET_CLEF_IMAGE) $(DEVNET_CONSENSUS_IMAGE) $(DEVNET_VALIDATOR_IMAGE) $(DEVNET_GENESIS_IMAGE)" in *local/*) echo "Kubernetes requires registry image references; override all local DEVNET_*_IMAGE values" >&2; exit 1 ;; esac ;; \
		*) echo "DEVNET_BACKEND must be docker or kubernetes" >&2; exit 2 ;; \
	esac
	@kurtosis version 2>/dev/null | grep -Eq '^CLI Version:[[:space:]]+1\.20\.' || { \
		echo "Kurtosis CLI 1.20.x is required (https://docs.kurtosis.com/upgrade)" >&2; \
		exit 1; \
	}
	kurtosis engine start

network-start: DEVNET_BACKEND=docker
network-start: network-image clef-image network-start-configured

network-start-k8s: DEVNET_BACKEND=kubernetes
network-start-k8s: network-start-configured

network-start-configured: network-preflight
	DEVNET_BACKEND="$(DEVNET_BACKEND)" \
	$(GO) run ./cmd/devnet start \
		--backend "$(DEVNET_BACKEND)" \
		--execution-image "$(DEVNET_EXECUTION_IMAGE)" \
		--clef-image "$(DEVNET_CLEF_IMAGE)" \
		$(if $(DEVNET_CONSENSUS_IMAGE),--consensus-image "$(DEVNET_CONSENSUS_IMAGE)") \
		$(if $(DEVNET_VALIDATOR_IMAGE),--validator-image "$(DEVNET_VALIDATOR_IMAGE)") \
		$(if $(DEVNET_GENESIS_IMAGE),--genesis-image "$(DEVNET_GENESIS_IMAGE)") \
		--profile "$(DEVNET_PROFILE)" \
		$(if $(DEVNET_ENCLAVE_NAME),--enclave-name "$(DEVNET_ENCLAVE_NAME)") \
		$(if $(DEVNET_START_TIMEOUT),--timeout "$(DEVNET_START_TIMEOUT)") \
		$(if $(DEVNET_PARAMS_FILE),--params-file "$(DEVNET_PARAMS_FILE)")

network-stop:
	$(GO) run ./cmd/devnet stop $(if $(DEVNET_ENCLAVE_NAME),--enclave-name "$(DEVNET_ENCLAVE_NAME)")

e2e-test:
	@test -n "$(strip $(GO_QRL_SOURCE_DIR))" || { echo "GO_QRL_SOURCE_DIR must point to a go-qrl checkout" >&2; exit 2; }
	@test -n "$(strip $(E2E_PACKAGES))" || { echo "E2E_PACKAGES must name at least one suite package" >&2; exit 2; }
	@mkdir -p "$(E2E_REPORT_DIR)"
	DEVNET_ENCLAVE_NAME="$(DEVNET_ENCLAVE_NAME)" \
	DEVNET_BACKEND="$(DEVNET_BACKEND)" \
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

e2e-execution: E2E_PACKAGES=./endtoend/suites/execution/abi ./endtoend/suites/execution/api ./endtoend/suites/execution/console ./endtoend/suites/execution/vm
e2e-execution: e2e-test

e2e-consensus: E2E_PACKAGES=./endtoend/suites/consensus/api ./endtoend/suites/consensus/protocol
e2e-consensus: e2e-test

e2e-crosslayer: E2E_PACKAGES=./endtoend/suites/crosslayer/...
e2e-crosslayer: E2E_SUITE_TIMEOUT=3h
e2e-crosslayer: e2e-test

e2e-signer: E2E_PACKAGES=./endtoend/suites/signer/...
e2e-signer: e2e-test

e2e-all: DEVNET_BACKEND=docker
e2e-all: network-image clef-image e2e-all-configured

e2e-all-k8s: DEVNET_BACKEND=kubernetes
e2e-all-k8s: e2e-all-configured

e2e-all-configured: network-preflight
	DEVNET_BACKEND="$(DEVNET_BACKEND)" \
	DEVNET_EXECUTION_IMAGE="$(DEVNET_EXECUTION_IMAGE)" \
	DEVNET_CLEF_IMAGE="$(DEVNET_CLEF_IMAGE)" \
	DEVNET_CONSENSUS_IMAGE="$(DEVNET_CONSENSUS_IMAGE)" \
	DEVNET_VALIDATOR_IMAGE="$(DEVNET_VALIDATOR_IMAGE)" \
	DEVNET_GENESIS_IMAGE="$(DEVNET_GENESIS_IMAGE)" \
	DEVNET_ENCLAVE_NAME="$(DEVNET_ENCLAVE_NAME)" \
	GO_QRL_SOURCE_DIR="$(GO_QRL_SOURCE_DIR)" \
	E2E_REPORT_DIR="$(E2E_REPORT_DIR)" \
	$(GO) run ./cmd/e2e run-all

e2e-core: E2E_PACKAGES=./endtoend/suites/execution/abi ./endtoend/suites/execution/api ./endtoend/suites/execution/console ./endtoend/suites/execution/vm ./endtoend/suites/signer/... ./endtoend/suites/crosslayer/engine ./endtoend/suites/crosslayer/network ./endtoend/suites/crosslayer/transactions
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

e2e-execution-sync: E2E_PACKAGES=./endtoend/suites/execution/sync
e2e-execution-sync: E2E_SUITE_TIMEOUT=45m
e2e-execution-sync: e2e-test

e2e-cold: E2E_PACKAGES=./endtoend/suites/consensus/coldstate
e2e-cold: E2E_SUITE_TIMEOUT=45m
e2e-cold: e2e-test

e2e-optimistic: E2E_PACKAGES=./endtoend/suites/consensus/optimistic
e2e-optimistic: E2E_SUITE_TIMEOUT=45m
e2e-optimistic: e2e-test

e2e-soak: E2E_PACKAGES=./endtoend/suites/system/soak
e2e-soak: E2E_SUITE_TIMEOUT=4h
e2e-soak: E2E_LABEL_FILTER=scenario-full
e2e-soak: e2e-test
