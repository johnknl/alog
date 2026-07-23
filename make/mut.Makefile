MUT_BIN ?= $(CURDIR)/bin/gremlins
MUT_CONFIG ?= $(CURDIR)/.gremlins.yaml
MUT_RESULTS_DIR ?= $(CURDIR)/.tmp/mutation
MUT_RESULTS_FILE ?= $(MUT_RESULTS_DIR)/gremlins.json
MUT_WORKERS ?= 2
MUT_TEST_CPU ?= 1
MUT_TIMEOUT_COEFFICIENT ?= 4
MUT_BASE_BRANCH ?= main

MUT_GREMLINS_COMMON_ARGS := \
	--config "$(MUT_CONFIG)" \
	--coverpkg "./..." \
	--workers "$(MUT_WORKERS)" \
	--test-cpu "$(MUT_TEST_CPU)" \
	--timeout-coefficient "$(MUT_TIMEOUT_COEFFICIENT)" \
	--exclude-files "^internal/storage/mocks/" \
	--exclude-files "^tools/cmd/docsync/" \
	--output "$(MUT_RESULTS_FILE)"

include make/common.mk

.PHONY: run
run: ## Run gremlins mutation testing
	mkdir -p "$(MUT_RESULTS_DIR)"
	$(MUT_BIN) unleash "." --integration $(MUT_GREMLINS_COMMON_ARGS)

.PHONY: dry-run
dry-run: ## Analyze mutation candidates without executing tests
	mkdir -p "$(MUT_RESULTS_DIR)"
	$(MUT_BIN) unleash "." --dry-run $(MUT_GREMLINS_COMMON_ARGS)

.PHONY: run-diff
run-diff: ## Run mutation testing only on diff from base branch
	mkdir -p "$(MUT_RESULTS_DIR)"
	$(MUT_BIN) unleash "." --integration --diff "$(MUT_BASE_BRANCH)" $(MUT_GREMLINS_COMMON_ARGS)

.PHONY: help
help: ## Show available targets
	$(call print-help)
