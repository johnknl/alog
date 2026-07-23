COVERAGE_FILE ?= $(CURDIR)/.tmp/coverage.out
DOCS_DIR ?= $(CURDIR)/docs
MOCKERY ?= $(BIN_DIR)/mockery
MOCK_FILE ?= $(CURDIR)/internal/storage/mocks/File.go
MOCK_FILESYSTEM_FILE ?= $(CURDIR)/internal/storage/mocks/FileSystem.go
UNIT_PACKAGES := ./...
COVERAGE_PACKAGES := $(shell go list ./... | grep -v 'github.com/johnknl/alog/internal/storage/mocks$$')

include make/common.mk

.PHONY: default
default: unit

.PHONY: test
test: unit fuzz mut bench ## Run all test suites and benchmarks

.PHONY: mocks
mocks: tools ## Generate mocks with mockery
	$(MOCKERY)

.PHONY: ensure-mocks
ensure-mocks: ## Generate mocks only when missing
	@if [ ! -f "$(MOCK_FILE)" ] || [ ! -f "$(MOCK_FILESYSTEM_FILE)" ]; then \
		$(MAKE) -sf make/Makefile tools; \
		$(MOCKERY); \
	fi

.PHONY: unit
unit: ensure-mocks ## Run all untagged tests 10x with -race and -shuffle=on
	go test -timeout=1m -count=10 -race -shuffle=on ./...

.PHONY: bench
bench: ## Run benchmarks and sync BENCH markers in markdown docs
	@$(MAKE) -sf make/bench.Makefile

.PHONY: fuzz
fuzz: ## Run fuzz tests (see make/fuzz.Makefile)
	@$(MAKE) -sf make/fuzz.Makefile

.PHONY: mut
mut: ## Run mutation tests (see make/mut.Makefile)
	@$(MAKE) -sf make/mut.Makefile

.PHONY: coverage
coverage: ensure-mocks ## Run unit tests with coverage profile
	mkdir -p "$(dir $(COVERAGE_FILE))"
	go test -v -race -covermode=atomic -coverprofile="$(COVERAGE_FILE)" $(COVERAGE_PACKAGES)
	go tool cover -func="$(COVERAGE_FILE)"

.PHONY: help
help: ## Show available targets
	$(call print-help)
