BENCH_DIR ?= $(CURDIR)/.tmp/bench
BENCH_PATTERN ?= ^BenchmarkArchive/append$$
BENCH_COUNT ?= 10
BENCH_PKG ?= .
PROFILE_STEM ?= $(shell printf '%s' "$(BENCH_PATTERN)" | tr -c '[:alnum:]' '_')
CPU_PROFILE ?= $(BENCH_DIR)/cpu.$(PROFILE_STEM).$(BENCH_COUNT).pprof
MEM_PROFILE ?= $(BENCH_DIR)/mem.$(PROFILE_STEM).$(BENCH_COUNT).pprof
BLOCK_PROFILE ?= $(BENCH_DIR)/block.$(PROFILE_STEM).$(BENCH_COUNT).pprof
MUTEX_PROFILE ?= $(BENCH_DIR)/mutex.$(PROFILE_STEM).$(BENCH_COUNT).pprof
PPROF_LIST_FUNC ?= github.com/johnknl/alog.(*Archive).Range

include make/common.mk

.PHONY: profile
profile: $(CPU_PROFILE) $(MEM_PROFILE) ## Run benchmark and save CPU/memory profiles

$(CPU_PROFILE) $(MEM_PROFILE) &: 
	mkdir -p "$(BENCH_DIR)"
	go test -run ^$$ -bench "$(BENCH_PATTERN)" -benchmem -count "$(BENCH_COUNT)" -cpuprofile "$(CPU_PROFILE)" -memprofile "$(MEM_PROFILE)" "$(BENCH_PKG)"

.PHONY: profile-locks
profile-locks: $(BLOCK_PROFILE) $(MUTEX_PROFILE) ## Run benchmark and save block/mutex profiles

$(BLOCK_PROFILE) $(MUTEX_PROFILE) &: 
	mkdir -p "$(BENCH_DIR)"
	go test -run ^$$ -bench "$(BENCH_PATTERN)" -benchmem -count "$(BENCH_COUNT)" -blockprofile "$(BLOCK_PROFILE)" -mutexprofile "$(MUTEX_PROFILE)" "$(BENCH_PKG)"

.PHONY: top-mem
top-mem: $(MEM_PROFILE) ## Show top allocators
	go tool pprof -top "$(MEM_PROFILE)"

.PHONY: top-cpu
top-cpu: $(CPU_PROFILE) ## Show top CPU functions
	go tool pprof -top "$(CPU_PROFILE)"

.PHONY: top-block
top-block: $(BLOCK_PROFILE) ## Show top blocking functions
	go tool pprof -top "$(BLOCK_PROFILE)"

.PHONY: top-mutex
top-mutex: $(MUTEX_PROFILE) ## Show top mutex contention
	go tool pprof -top "$(MUTEX_PROFILE)"

.PHONY: list-mem
list-mem: $(MEM_PROFILE) ## Show alloc lines for PPROF_LIST_FUNC
	go tool pprof -list="$(PPROF_LIST_FUNC)" "$(MEM_PROFILE)"

.PHONY: list-cpu
list-cpu: $(CPU_PROFILE) ## Show CPU lines for PPROF_LIST_FUNC
	go tool pprof -list="$(PPROF_LIST_FUNC)" "$(CPU_PROFILE)"

.PHONY: escape
escape: ## Run benchmark with escape analysis
	go test -run ^$$ -bench "$(BENCH_PATTERN)" -gcflags='all=-m=2' "$(BENCH_PKG)"

.PHONY: help
help: ## Show available targets
	$(call print-help)
