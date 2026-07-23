include make/common.mk

.PHONY: help
help: ## Show available targets
	@echo "Available targets:"
	@echo
	@awk -v width="$(HELP_WIDTH)" 'BEGIN {FS = ":.*## "; printf "make:\n"} /^[a-zA-Z0-9_.%\/-]+:.*## / {printf "  %-*s %s\n", width, $$1, $$2}' $(MAKEFILE_LIST)
	@for f in make/*.Makefile; do \
		echo; \
		awk -v width="$(HELP_WIDTH)" 'BEGIN {FS = ":.*## "; printf "make -f '"$$f"':\n"} /^[a-zA-Z0-9_.%\/-]+:.*## / {printf "  %-*s %s\n", width, $$1, $$2}' $$f; \
	done

.PHONY: tidy
tidy: ## Tidy and verify app module dependencies
	go mod tidy
	go mod verify

.PHONY: tools-tidy
tools-tidy: ## Tidy and verify tools module dependencies
	go -C tools mod tidy
	go -C tools mod verify

.PHONY: pretty
pretty: fmt ## Alias for fmt

# Use addlicense for header management because golangci-lint goheader has a
# known limitation with //go:build files where missing headers are not reliably
# reported/fixed. See:
# https://github.com/golangci/golangci-lint/blob/622ca0728e72a5bf1040a9213c9bb1f2a99987b8/pkg/golinters/goheader/goheader.go#L87
.PHONY: headers
headers: ## Add/update license headers on Go files
	@set -eu; \
	files=""; \
	for f in $$(git ls-files '*.go'; git ls-files --others --exclude-standard '*.go'); do \
		if [ -f "$$f" ]; then files="$$files $$f"; fi; \
	done; \
	files="$${files# }"; \
	if [ -n "$$files" ]; then \
		$(BIN_DIR)/addlicense -f LICENSE $$files; \
	fi

.PHONY: headers-check
headers-check: ## Verify license headers on Go files
	@set -eu; \
	files=""; \
	for f in $$(git ls-files '*.go'; git ls-files --others --exclude-standard '*.go'); do \
		if [ -f "$$f" ]; then files="$$files $$f"; fi; \
	done; \
	files="$${files# }"; \
	if [ -n "$$files" ]; then \
		$(BIN_DIR)/addlicense -check -f LICENSE $$files; \
	fi

.PHONY: fmt
fmt: headers ## Format Go code
	GOFLAGS="-tags=$(BUILD_TAGS)" go fmt ./...
	$(BIN_DIR)/golangci-lint run --fix

.PHONY: docs
docs: ## Start docs dev server (see make/docs.Makefile)
	@$(MAKE) -sf make/docs.Makefile

.PHONY: mocks
mocks: ## Generate mocks (see make/test.Makefile)
	@$(MAKE) -sf make/test.Makefile mocks

.PHONY: tools
tools: ## Install tools declared in tools/tools.go
	mkdir -p "$(BIN_DIR)"
	@for tool in $$(awk -F'"' '/^\t_ "[^"]+"/ {print $$2}' tools/tools.go); do \
		GOBIN="$(BIN_DIR)" go -C tools install $$tool; \
	done

.PHONY: lint
lint: headers-check ## Run golangci-lint (verify + lint)
	$(BIN_DIR)/golangci-lint config verify -c .golangci.yml
	$(BIN_DIR)/golangci-lint run

.PHONY: vuln
vuln: tools ## Run govulncheck
	$(BIN_DIR)/govulncheck ./...

.PHONY: test
test: ## Run unit tests (see make/test.Makefile)
	@$(MAKE) -sf make/test.Makefile

.PHONY: bench
bench: ## Run benchmarks (see make/bench.Makefile)
	@$(MAKE) -sf make/bench.Makefile

.PHONY: coverage
coverage: ## Run coverage suite (see make/test.Makefile)
	@$(MAKE) -sf make/test.Makefile coverage

.PHONY: ci
ci: coverage lint vuln ## Run CI checks
	@$(MAKE) -sf make/docs.Makefile check

.PHONY: upgrade-toolchain
upgrade-toolchain: ## Upgrade Go toolchain to the latest patch
	go get toolchain@patch

.PHONY: pkgsite-refresh
pkgsite-refresh:
	GOPROXY=https://proxy.golang.org go list -m github.com/johnknl/alog@latest

.PHONY: clean
clean: ## Clean temporary files
	@rm -rf "$(BIN_DIR)" .tmp bench/results/*.tmp; \
		find -name 'mock_*.go' -delete

.PHONY: release
release: ## Create a new release (tag and push)
	@set -eu; \
	current="$$(bin/svu current)"; \
	next="$$(bin/svu next)"; \
	if ! bin/gorelease -base="$$current" -version="$$next" >/tmp/gorelease.log; then \
		cat /tmp/gorelease.log >&2; \
		exit 1; \
	fi; \
	git tag "$$next" && \
	git push origin "$$next";
