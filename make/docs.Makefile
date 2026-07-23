BIN_DIR ?= $(CURDIR)/bin
APP_NAME ?= replicators
BUILD_TAGS ?=
COVERAGE_FILE ?= $(CURDIR)/.tmp/coverage.out
DOCS_DIR ?= $(CURDIR)/docs
DOCSYNC ?= $(BIN_DIR)/docsync

include make/common.mk

.PHONY: docs
docs: ## Build docs
	docker run --rm \
		-p 8000:8000 \
	  	-v "$$PWD:/docs" \
	  	squidfunk/mkdocs-material:9

.PHONY: sync
sync: $(DOCSYNC) ## Sync EXAMPLE markers in all markdown files
	$(DOCSYNC) --root "$(CURDIR)"

.PHONY: check
check: $(DOCSYNC) ## Verify EXAMPLE markers are in sync in all markdown files
	$(DOCSYNC) --root "$(CURDIR)" --check

$(DOCSYNC):
	mkdir -p "$(BIN_DIR)"
	GOBIN="$(BIN_DIR)" go -C tools install ./cmd/docsync

.PHONY: help
help: ## Show available targets
	$(call print-help)
