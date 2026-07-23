#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

VM_NAME="${VM_NAME:-alog-fuzz}"
VM_BASE_IMAGE="${VM_BASE_IMAGE:-https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img}"
VM_CPUS="${VM_CPUS:-4}"
VM_MEMORY_MIB="${VM_MEMORY_MIB:-8192}"
VM_DISK_GIB="${VM_DISK_GIB:-32}"
VM_WORKDIR="${VM_WORKDIR:-/home/alog/alog}"
VM_STORAGE_DIR="${VM_STORAGE_DIR:-/var/lib/libvirt/images/${VM_NAME}}"
LIBVIRT_URI="${LIBVIRT_URI:-qemu:///system}"

FUZZ_COUNT="${FUZZ_COUNT:-1}"
FUZZ_TIME="${FUZZ_TIME:-2m}"
FUZZ_TARGET="${FUZZ_TARGET:-.}"
FUZZ_PARALLEL="${FUZZ_PARALLEL:-1}"
FUZZ_GOMAXPROCS="${FUZZ_GOMAXPROCS:-1}"
FUZZ_GOMEMLIMIT="${FUZZ_GOMEMLIMIT:-1024MiB}"
FUZZ_GOGC="${FUZZ_GOGC:-50}"

STATE_DIR="${ROOT_DIR}/.tmp/fuzz-vm"
RUNTIME_DIR="${STATE_DIR}/runtime"
BASE_IMG="${VM_STORAGE_DIR}/base.qcow2"
DISK_IMG="${VM_STORAGE_DIR}/${VM_NAME}.qcow2"
CLOUD_IMG="${VM_STORAGE_DIR}/${VM_NAME}-seed.iso"
SSH_KEY="${RUNTIME_DIR}/${VM_NAME}-id_ed25519"
SSH_PUB="${SSH_KEY}.pub"
MAC_FILE="${RUNTIME_DIR}/${VM_NAME}.mac"
KNOWN_HOSTS="${RUNTIME_DIR}/known_hosts"

if [[ -f "${MAC_FILE}" ]]; then
  VM_MAC="$(<"${MAC_FILE}")"
else
  VM_MAC=""
fi

usage() {
  cat <<'EOF'
Usage: make/scripts/fuzz-vm.sh <provision|run|ssh|stop|destroy>

Environment:
  LIBVIRT_URI   libvirt URI (default: qemu:///system)
  VM_STORAGE_DIR directory for hypervisor-attached disk/seed images
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

ensure_requirements() {
  require_cmd virsh
  require_cmd virt-install
  require_cmd qemu-img
  require_cmd sudo
  require_cmd ssh
  require_cmd ssh-keygen
  require_cmd curl
  require_cmd awk
  require_cmd getent
}

hypervisor_owner() {
  if getent passwd libvirt-qemu >/dev/null 2>&1; then
    printf '%s' "libvirt-qemu:$(id -gn libvirt-qemu)"
    return
  fi
  if getent passwd qemu >/dev/null 2>&1; then
    printf '%s' "qemu:$(id -gn qemu)"
    return
  fi
  printf '%s' ""
}

ensure_hypervisor_access() {
  local path="$1"
  local owner

  owner="$(hypervisor_owner)"
  if [[ -n "${owner}" ]]; then
    sudo chown "${owner}" "${path}"
  fi
  sudo chmod 0664 "${path}"
}

virsh_cmd() {
  sudo virsh -c "${LIBVIRT_URI}" "$@"
}

virt_install_cmd() {
  sudo virt-install --connect "${LIBVIRT_URI}" "$@"
}

domain_exists() {
  virsh_cmd dominfo "${VM_NAME}" >/dev/null 2>&1
}

ensure_state_dirs() {
  mkdir -p "${RUNTIME_DIR}"
  sudo mkdir -p "${VM_STORAGE_DIR}"
  sudo chmod 0775 "${VM_STORAGE_DIR}"
}

warn_if_domain_shape_mismatch() {
  if ! domain_exists; then
    return
  fi

  local current_mem_kib current_vcpus target_mem_kib
  current_mem_kib="$(virsh_cmd dominfo "${VM_NAME}" | awk -F: '/Max memory/ {gsub(/[^0-9]/, "", $2); print $2; exit}')"
  current_vcpus="$(virsh_cmd dominfo "${VM_NAME}" | awk -F: '/CPU\(s\)/ {gsub(/[^0-9]/, "", $2); print $2; exit}')"
  target_mem_kib="$((VM_MEMORY_MIB * 1024))"

  if [[ -n "${current_mem_kib}" && "${current_mem_kib}" != "${target_mem_kib}" ]]; then
    echo "warning: existing domain memory (${current_mem_kib} KiB) does not match requested (${target_mem_kib} KiB)." >&2
    echo "warning: run 'make -f make/fuzz.Makefile fuzz-sandbox-destroy' then reprovision to apply new VM size." >&2
  fi

  if [[ -n "${current_vcpus}" && "${current_vcpus}" != "${VM_CPUS}" ]]; then
    echo "warning: existing domain vCPUs (${current_vcpus}) do not match requested (${VM_CPUS})." >&2
    echo "warning: run 'make -f make/fuzz.Makefile fuzz-sandbox-destroy' then reprovision to apply new vCPU count." >&2
  fi
}

create_ssh_key() {
  if [[ -f "${SSH_KEY}" && -f "${SSH_PUB}" ]]; then
    return
  fi
  ssh-keygen -t ed25519 -N "" -f "${SSH_KEY}" >/dev/null
}

generate_mac() {
  local bytes
  bytes="$(od -An -N3 -tx1 /dev/urandom | tr -d ' \n')"
  VM_MAC="52:54:00:${bytes:0:2}:${bytes:2:2}:${bytes:4:2}"
  printf '%s' "${VM_MAC}" >"${MAC_FILE}"
}

ensure_mac() {
  if [[ -z "${VM_MAC}" ]]; then
    generate_mac
  fi
}

download_base_image() {
  if sudo test -f "${BASE_IMG}"; then
    return
  fi

  local tmp_img
  tmp_img="${RUNTIME_DIR}/${VM_NAME}-base-download.qcow2"
  curl -fsSL "${VM_BASE_IMAGE}" -o "${tmp_img}"
  sudo mv "${tmp_img}" "${BASE_IMG}"
  ensure_hypervisor_access "${BASE_IMG}"
}

ensure_disk_image() {
  if sudo test -f "${DISK_IMG}"; then
    return
  fi

  sudo qemu-img create -f qcow2 -F qcow2 -b "${BASE_IMG}" "${DISK_IMG}" "${VM_DISK_GIB}G" >/dev/null
  ensure_hypervisor_access "${DISK_IMG}"
}

render_cloud_init() {
  local user_data meta_data
  user_data="${RUNTIME_DIR}/${VM_NAME}-user-data.yaml"
  meta_data="${RUNTIME_DIR}/${VM_NAME}-meta-data.yaml"

  cat >"${user_data}" <<EOF
#cloud-config
users:
  - default
  - name: alog
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
    ssh_authorized_keys:
      - $(<"${SSH_PUB}")
package_update: true
packages:
  - git
  - make
  - golang-go
  - rsync
runcmd:
  - ["mkdir", "-p", "${VM_WORKDIR}"]
  - ["chown", "-R", "alog:alog", "/home/alog"]
EOF

  cat >"${meta_data}" <<EOF
instance-id: ${VM_NAME}
local-hostname: ${VM_NAME}
EOF

  if command -v cloud-localds >/dev/null 2>&1; then
    sudo cloud-localds "${CLOUD_IMG}" "${user_data}" "${meta_data}"
    ensure_hypervisor_access "${CLOUD_IMG}"
  fi
}

create_domain_cloud_localds() {
  virt_install_cmd \
    --name "${VM_NAME}" \
    --memory "${VM_MEMORY_MIB}" \
    --vcpus "${VM_CPUS}" \
    --import \
    --os-variant ubuntu24.04 \
    --disk "path=${DISK_IMG},format=qcow2,bus=virtio" \
    --disk "path=${CLOUD_IMG},device=cdrom" \
    --network "network=default,model=virtio,mac=${VM_MAC}" \
    --graphics none \
    --noautoconsole >/dev/null
}

create_domain_virt_install_cloud_init() {
  local user_data meta_data
  user_data="${RUNTIME_DIR}/${VM_NAME}-user-data.yaml"
  meta_data="${RUNTIME_DIR}/${VM_NAME}-meta-data.yaml"

  virt_install_cmd \
    --name "${VM_NAME}" \
    --memory "${VM_MEMORY_MIB}" \
    --vcpus "${VM_CPUS}" \
    --import \
    --os-variant ubuntu24.04 \
    --disk "path=${DISK_IMG},format=qcow2,bus=virtio" \
    --cloud-init "user-data=${user_data},meta-data=${meta_data}" \
    --network "network=default,model=virtio,mac=${VM_MAC}" \
    --graphics none \
    --noautoconsole >/dev/null
}

create_domain() {
  ensure_mac
  if command -v cloud-localds >/dev/null 2>&1; then
    create_domain_cloud_localds
    return
  fi

  create_domain_virt_install_cloud_init
}

start_domain() {
  if ! virsh_cmd domstate "${VM_NAME}" | grep -q running; then
    virsh_cmd start "${VM_NAME}" >/dev/null
  fi
}

stop_domain() {
  if virsh_cmd domstate "${VM_NAME}" | grep -q running; then
    virsh_cmd shutdown "${VM_NAME}" >/dev/null || true
    for _ in $(seq 1 30); do
      if ! virsh_cmd domstate "${VM_NAME}" | grep -q running; then
        return
      fi
      sleep 1
    done
    virsh_cmd destroy "${VM_NAME}" >/dev/null || true
  fi
}

lookup_ip() {
  local ip
  ip=""
  for _ in $(seq 1 60); do
    ip="$(virsh_cmd net-dhcp-leases default | awk -v mac="${VM_MAC}" '$0 ~ mac {split($5, a, "/"); print a[1]}')"
    if [[ -n "${ip}" ]]; then
      printf '%s' "${ip}"
      return
    fi
    sleep 2
  done
  return 1
}

ssh_opts() {
  cat <<EOF
-i ${SSH_KEY} -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=${KNOWN_HOSTS} -o ConnectTimeout=10
EOF
}

wait_for_ssh() {
  local ip="$1"
  local opts
  opts="$(ssh_opts)"
  for _ in $(seq 1 60); do
    if ssh ${opts} alog@"${ip}" true >/dev/null 2>&1; then
      return
    fi
    sleep 2
  done
  echo "ssh did not become ready for ${VM_NAME}" >&2
  exit 1
}

sync_repo() {
  local ip="$1"
  local opts
  opts="$(ssh_opts)"
  rsync -az --delete \
    --exclude '.git' \
    --exclude '.tmp' \
    --exclude 'bin' \
    -e "ssh ${opts}" \
    "${ROOT_DIR}/" "alog@${ip}:${VM_WORKDIR}/"
}

run_fuzz_in_vm() {
	local ip="$1"
	local opts
	opts="$(ssh_opts)"

	ssh ${opts} alog@"${ip}" \
		"set -euo pipefail; cd '${VM_WORKDIR}'; LOGICAL_CPUS=\$(nproc 2>/dev/null || echo 1); SAFE_PARALLEL='${FUZZ_PARALLEL}'; SAFE_PROCS='${FUZZ_GOMAXPROCS}'; if [ \"\${SAFE_PARALLEL}\" -gt \"\${LOGICAL_CPUS}\" ]; then SAFE_PARALLEL=\${LOGICAL_CPUS}; fi; if [ \"\${SAFE_PROCS}\" -gt \"\${LOGICAL_CPUS}\" ]; then SAFE_PROCS=\${LOGICAL_CPUS}; fi; if [ \"\${SAFE_PARALLEL}\" -lt 1 ]; then SAFE_PARALLEL=1; fi; if [ \"\${SAFE_PROCS}\" -lt 1 ]; then SAFE_PROCS=1; fi; FUZZ_COUNT='${FUZZ_COUNT}' FUZZ_TIME='${FUZZ_TIME}' FUZZ_TARGET='${FUZZ_TARGET}' FUZZ_PARALLEL=\"\${SAFE_PARALLEL}\" FUZZ_GOMAXPROCS=\"\${SAFE_PROCS}\" FUZZ_GOMEMLIMIT='${FUZZ_GOMEMLIMIT}' FUZZ_GOGC='${FUZZ_GOGC}' make -f make/fuzz.Makefile run"
}

cmd_provision() {
  ensure_requirements
  ensure_state_dirs
  create_ssh_key
  ensure_mac
  download_base_image
  ensure_disk_image
  render_cloud_init

  warn_if_domain_shape_mismatch

  if ! domain_exists; then
    create_domain
  fi

  start_domain
  local ip
  ip="$(lookup_ip)"
  wait_for_ssh "${ip}"
  echo "${VM_NAME} ready at ${ip}"
}

cmd_run() {
  cmd_provision
  local ip
  ip="$(lookup_ip)"
  sync_repo "${ip}"
  run_fuzz_in_vm "${ip}"
}

cmd_ssh() {
  ensure_requirements
  ensure_state_dirs
  ensure_mac
  start_domain
  local ip
  ip="$(lookup_ip)"
  wait_for_ssh "${ip}"
  local opts
  opts="$(ssh_opts)"
  ssh ${opts} alog@"${ip}"
}

cmd_stop() {
  ensure_requirements
  if domain_exists; then
    stop_domain
  fi
}

cmd_destroy() {
  ensure_requirements
  if domain_exists; then
    stop_domain
    virsh_cmd undefine "${VM_NAME}" --remove-all-storage --nvram >/dev/null 2>&1 || virsh_cmd undefine "${VM_NAME}" >/dev/null
  fi
  rm -rf "${STATE_DIR}"
}

main() {
  if [[ $# -ne 1 ]]; then
    usage
    exit 1
  fi

  case "$1" in
    provision) cmd_provision ;;
    run) cmd_run ;;
    ssh) cmd_ssh ;;
    stop) cmd_stop ;;
    destroy) cmd_destroy ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "$@"
