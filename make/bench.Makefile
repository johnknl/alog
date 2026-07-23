BENCH_DIR ?= $(CURDIR)/.tmp/bench
BENCH_PATTERN ?= .
BENCH_COUNT ?= 6
BENCH_REF ?= HEAD
BENCH_A ?= auto
BENCH_B ?= auto
BENCH_FILE_TAG ?=
BENCH_SHA_LEN ?= 12

include make/common.mk

.PHONY: bench
bench: ## Run benchmarks for BENCH_REF and store output at bench/results/<sha>[-dirty][--tag].txt
	@set -eu; \
	mkdir -p "$(BENCH_STORE_DIR)"; \
	file_tag="$(BENCH_FILE_TAG)"; \
	if [ -z "$$file_tag" ] && [ "$(BENCH_PATTERN)" != "." ]; then \
		file_tag="$$(printf '%s' "$(BENCH_PATTERN)" | tr -cs '[:alnum:]._-' '_')"; \
		file_tag="$${file_tag#_}"; \
		file_tag="$${file_tag%_}"; \
		if [ -z "$$file_tag" ]; then file_tag="pattern"; fi; \
	fi; \
	file_tag_suffix=""; \
	if [ -n "$$file_tag" ]; then file_tag_suffix="--$$file_tag"; fi; \
	resolved_ref=""; \
	if resolved_ref="$$(git rev-parse --verify --quiet "$(BENCH_REF)^{commit}")"; then :; else \
		git fetch --quiet origin "$(BENCH_REF)"; \
		resolved_ref="$$(git rev-parse --verify "FETCH_HEAD^{commit}")"; \
	fi; \
	short_ref="$$(git rev-parse --short=$(BENCH_SHA_LEN) "$$resolved_ref")"; \
	suffix=""; \
	run_dir="$(CURDIR)"; \
	if [ "$$resolved_ref" = "$$(git rev-parse --verify HEAD)" ]; then \
		if [ -n "$$(git status --porcelain --untracked-files=normal)" ]; then suffix="-dirty"; fi; \
	else \
		ref_worktree="$(BENCH_DIR)/.ref-worktree"; \
		rm -rf "$$ref_worktree"; \
		git worktree add -f --detach "$$ref_worktree" "$$resolved_ref"; \
		cd "$$ref_worktree"; make tools mocks; cd -; \
		cleanup() { git worktree remove --force "$$ref_worktree" >/dev/null 2>&1 || true; }; \
		trap cleanup EXIT INT TERM; \
		run_dir="$$ref_worktree"; \
	fi; \
	out_file="$(BENCH_STORE_DIR)/$$short_ref$$suffix$$file_tag_suffix.txt"; \
	tmp_file="$$out_file.tmp"; \
	GOWORK=off go -C "$$run_dir" test -run ^$$ -bench "$(BENCH_PATTERN)" -benchmem -count=$(BENCH_COUNT) ./... | tee "$$tmp_file"; \
	mv "$$tmp_file" "$$out_file"; \
	printf 'stored benchmark snapshot: %s\n' "$$out_file"

.PHONY: bench-compare
bench-compare: ## Compare snapshots (auto: previous commit vs current, or clean vs dirty)
	@set -eu; \
	file_tag="$(BENCH_FILE_TAG)"; \
	if [ -z "$$file_tag" ] && [ "$(BENCH_PATTERN)" != "." ]; then \
		file_tag="$$(printf '%s' "$(BENCH_PATTERN)" | tr -cs '[:alnum:]._-' '_')"; \
		file_tag="$${file_tag#_}"; \
		file_tag="$${file_tag%_}"; \
		if [ -z "$$file_tag" ]; then file_tag="pattern"; fi; \
	fi; \
	file_tag_suffix=""; \
	if [ -n "$$file_tag" ]; then file_tag_suffix="--$$file_tag"; fi; \
	resolve_ref() { \
		name="$$1"; \
		if resolved="$$(git rev-parse --verify --quiet "$$name^{commit}")"; then \
			printf '%s\n' "$$resolved"; \
			return 0; \
		fi; \
		git fetch --quiet origin "$$name"; \
		git rev-parse --verify "FETCH_HEAD^{commit}"; \
	}; \
	short_ref() { git rev-parse --short=$(BENCH_SHA_LEN) "$$1"; }; \
	clean_file_for() { \
		ref="$$1"; \
		short="$$(short_ref "$$ref")"; \
		if [ -n "$$file_tag_suffix" ]; then \
			file="$(BENCH_STORE_DIR)/$$short$$file_tag_suffix.txt"; \
		else \
			file="$(BENCH_STORE_DIR)/$$short.txt"; \
		fi; \
		if [ -f "$$file" ]; then \
			printf '%s\n' "$$file"; \
			return 0; \
		fi; \
		printf 'missing clean snapshot: %s\n' "$$file" >&2; \
		exit 1; \
	}; \
	dirty_file_for() { \
		ref="$$1"; \
		short="$$(short_ref "$$ref")"; \
		if [ -n "$$file_tag_suffix" ]; then \
			file="$(BENCH_STORE_DIR)/$$short-dirty$$file_tag_suffix.txt"; \
		else \
			file="$(BENCH_STORE_DIR)/$$short-dirty.txt"; \
		fi; \
		if [ -f "$$file" ]; then \
			printf '%s\n' "$$file"; \
			return 0; \
		fi; \
		printf 'missing dirty snapshot: %s\n' "$$file" >&2; \
		exit 1; \
	}; \
	a_name="$(BENCH_A)"; \
	b_name="$(BENCH_B)"; \
	head_ref="$$(git rev-parse --verify HEAD)"; \
	dirty=0; \
	if [ -n "$$(git status --porcelain --untracked-files=normal)" ]; then dirty=1; fi; \
	mode="clean"; \
	if [ "$$a_name" = "auto" ] && [ "$$b_name" = "auto" ]; then \
		if [ "$$dirty" = "1" ]; then \
			a_ref="$$head_ref"; \
			b_ref="$$head_ref"; \
			mode="dirty"; \
		else \
			a_ref="$$(resolve_ref HEAD~1)"; \
			b_ref="$$head_ref"; \
		fi; \
	else \
		if [ "$$a_name" = "auto" ]; then a_name="HEAD~1"; fi; \
		if [ "$$b_name" = "auto" ]; then b_name="HEAD"; fi; \
		a_ref="$$(resolve_ref "$$a_name")"; \
		b_ref="$$(resolve_ref "$$b_name")"; \
		if [ "$$b_name" = "HEAD" ] && [ "$$dirty" = "1" ] && [ "$$a_ref" = "$$head_ref" ]; then \
			mode="dirty"; \
		fi; \
	fi; \
	a_file="$$(clean_file_for "$$a_ref")"; \
	if [ "$$mode" = "dirty" ]; then \
		b_file="$$(dirty_file_for "$$b_ref")"; \
	else \
		b_file="$$(clean_file_for "$$b_ref")"; \
	fi; \
	printf 'comparing %s\n' "$$a_file"; \
	printf '      with %s\n' "$$b_file"; \
	$(BIN_DIR)/benchstat "$$a_file" "$$b_file"

.PHONY: bench-list
bench-list: ## List stored benchmark snapshots
	@set -eu; \
	mkdir -p "$(BENCH_STORE_DIR)"; \
	ls -1 "$(BENCH_STORE_DIR)"/*.txt 2>/dev/null || true

.PHONY: help
help: ## Show available targets
	$(call print-help)
