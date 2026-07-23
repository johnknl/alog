FUZZ_COUNT ?= 1
FUZZ_TIME ?= 20s
FUZZ_PKG ?= ./...
FUZZ_TARGET ?= .
FUZZ_PARALLEL ?= 1
FUZZ_GOMAXPROCS ?= 1
FUZZ_GOMEMLIMIT ?= 1024MiB
FUZZ_GOGC ?= 50

VM_NAME ?= alog-fuzz
VM_BASE_IMAGE ?= https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
VM_CPUS ?= 16
VM_MEMORY_MIB ?= 32768
VM_DISK_GIB ?= 32
VM_WORKDIR ?= /home/alog/alog
VM_STORAGE_DIR ?= /var/lib/libvirt/images/$(VM_NAME)
LIBVIRT_URI ?= qemu:///system
VM_FUZZ_TIME ?= 90s
VM_FUZZ_COUNT ?= 1
VM_FUZZ_TARGET ?= .
VM_FUZZ_PARALLEL ?= 1
VM_FUZZ_GOMAXPROCS ?= 1
VM_FUZZ_GOMEMLIMIT ?= 1536MiB
VM_FUZZ_GOGC ?= 100

include make/common.mk

.PHONY: run
run: ## Run fuzz tests (auto-discovers targets)
	@set -eu; \
	export GOMAXPROCS="$(FUZZ_GOMAXPROCS)"; \
	export GOMEMLIMIT="$(FUZZ_GOMEMLIMIT)"; \
	export GOGC="$(FUZZ_GOGC)"; \
	if [ "$(FUZZ_TARGET)" != "." ]; then \
		set -- $$(go list $(FUZZ_PKG)); \
		if [ "$$#" -ne 1 ]; then \
			echo "FUZZ_TARGET requires FUZZ_PKG to resolve to exactly one package"; \
			exit 1; \
		fi; \
		go test -timeout=2m -count=$(FUZZ_COUNT) -parallel=$(FUZZ_PARALLEL) -run=^$$ -fuzz "$(FUZZ_TARGET)" -fuzztime $(FUZZ_TIME) "$$1"; \
		exit 0; \
	fi; \
	for pkg in $$(go list $(FUZZ_PKG)); do \
		has_target=0; \
		for target in $$(go test -list '^Fuzz' "$$pkg"); do \
			case "$$target" in \
				Fuzz*) \
					has_target=1; \
					echo "Running: $$pkg:$$target"; \
					go test -timeout=2m -count=$(FUZZ_COUNT) -parallel=$(FUZZ_PARALLEL) -run=^$$ -fuzz "^$$target$$" -fuzztime $(FUZZ_TIME) "$$pkg"; \
				;; \
			esac; \
		done; \
		if [ "$$has_target" -eq 0 ]; then \
			echo "==> $$pkg: no fuzz targets"; \
		fi; \
	done

.PHONY: fuzz-sandbox-provision
fuzz-sandbox-provision: ## Create or update fuzz sandbox
	VM_NAME=$(VM_NAME) VM_BASE_IMAGE=$(VM_BASE_IMAGE) VM_CPUS=$(VM_CPUS) VM_MEMORY_MIB=$(VM_MEMORY_MIB) VM_DISK_GIB=$(VM_DISK_GIB) VM_WORKDIR=$(VM_WORKDIR) VM_STORAGE_DIR=$(VM_STORAGE_DIR) LIBVIRT_URI=$(LIBVIRT_URI) ./make/scripts/fuzz-vm.sh provision

.PHONY: fuzz-sandbox
fuzz-sandbox: ## Run fuzz tests in sandbox
	VM_NAME=$(VM_NAME) VM_BASE_IMAGE=$(VM_BASE_IMAGE) VM_CPUS=$(VM_CPUS) VM_MEMORY_MIB=$(VM_MEMORY_MIB) VM_DISK_GIB=$(VM_DISK_GIB) VM_WORKDIR=$(VM_WORKDIR) VM_STORAGE_DIR=$(VM_STORAGE_DIR) LIBVIRT_URI=$(LIBVIRT_URI) FUZZ_COUNT=$(VM_FUZZ_COUNT) FUZZ_TIME=$(VM_FUZZ_TIME) FUZZ_TARGET=$(VM_FUZZ_TARGET) FUZZ_PARALLEL=$(VM_FUZZ_PARALLEL) FUZZ_GOMAXPROCS=$(VM_FUZZ_GOMAXPROCS) FUZZ_GOMEMLIMIT=$(VM_FUZZ_GOMEMLIMIT) FUZZ_GOGC=$(VM_FUZZ_GOGC) ./make/scripts/fuzz-vm.sh run

.PHONY: fuzz-sandbox-ssh
fuzz-sandbox-ssh: ## Open shell in fuzz sandbox
	VM_NAME=$(VM_NAME) VM_WORKDIR=$(VM_WORKDIR) VM_STORAGE_DIR=$(VM_STORAGE_DIR) LIBVIRT_URI=$(LIBVIRT_URI) ./make/scripts/fuzz-vm.sh ssh

.PHONY: fuzz-sandbox-stop
fuzz-sandbox-stop: ## Stop fuzz sandbox
	VM_NAME=$(VM_NAME) VM_STORAGE_DIR=$(VM_STORAGE_DIR) LIBVIRT_URI=$(LIBVIRT_URI) ./make/scripts/fuzz-vm.sh stop

.PHONY: fuzz-sandbox-destroy
fuzz-sandbox-destroy: ## Destroy fuzz sandbox and local artifacts
	VM_NAME=$(VM_NAME) VM_STORAGE_DIR=$(VM_STORAGE_DIR) LIBVIRT_URI=$(LIBVIRT_URI) ./make/scripts/fuzz-vm.sh destroy

.PHONY: help
help: ## Show available targets
	$(call print-help)
