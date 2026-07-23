HELP_WIDTH ?= 24
BIN_DIR ?= $(CURDIR)/bin
BENCH_STORE_DIR ?= $(CURDIR)/bench/results
BENCH_CURRENT_FILE ?= $(BENCH_DIR)/current.txt
BUILD_TAGS ?= tools

define print-help
	@awk -v width="$(HELP_WIDTH)" 'BEGIN {FS = ":.*## "; printf "Available targets:\n"} /^[a-zA-Z0-9_.%\/-]+:.*## / {printf "  make %-*s %s\n", width, $$1, $$2}' $(MAKEFILE_LIST)
endef
