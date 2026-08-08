#!/bin/bash
# Applies Linux kernel networking tunables for high-connection-count
# workloads, then builds and runs the key-value-cache container with host
# networking. Intended for bare-metal/VM Linux hosts; skip this on
# constrained platforms (shared k8s nodes, WSL, CI) where you don't control
# the host network stack -- use `make docker-run` there instead.
set -euo pipefail

IMAGE="${IMAGE:-key-value-cache}"
PORT="${PORT:-7171}"

sysctl_set() {
	local key="$1" val="$2"
	if sudo sysctl -w "${key}=${val}" >/dev/null 2>&1; then
		echo "  ok   ${key}=${val}"
	else
		echo "  skip ${key} (not available on this kernel)"
	fi
}

echo "Applying kernel optimizations for high-performance networking..."

sysctl_set net.core.somaxconn 131072
sysctl_set net.core.netdev_max_backlog 500000
sysctl_set net.ipv4.tcp_tw_reuse 1
sysctl_set net.ipv4.tcp_fin_timeout 10
sysctl_set net.ipv4.tcp_max_syn_backlog 262144
sysctl_set net.ipv4.tcp_mem "786432 1048576 1572864"
sysctl_set net.ipv4.tcp_rmem "4096 87380 6291456"
sysctl_set net.ipv4.tcp_wmem "4096 87380 6291456"
sysctl_set net.ipv4.ip_local_port_range "1024 65535"
sysctl_set net.ipv4.tcp_fastopen 3
sysctl_set fs.file-max 2097152
sysctl_set net.ipv4.tcp_keepalive_time 60
sysctl_set net.ipv4.tcp_keepalive_intvl 10
sysctl_set net.ipv4.tcp_keepalive_probes 5

# net.ipv4.tcp_syncookies is intentionally left untouched: disabling it
# (the old default here) removes SYN flood protection and isn't needed for
# legitimate high-throughput traffic.
# net.ipv4.tcp_tw_recycle was removed from the kernel in 4.12+ and is not set.

ulimit -n 1048576 2>/dev/null || echo "  skip ulimit -n 1048576 (raise the hard limit in /etc/security/limits.conf if needed)"

echo "Kernel optimizations applied."
echo "Building ${IMAGE}..."
docker build -t "${IMAGE}" .

echo "Starting ${IMAGE} on host network, port ${PORT}..."
docker run --rm -p "${PORT}:${PORT}" \
	--ulimit nofile=1048576:1048576 \
	--network=host \
	-e PORT="${PORT}" \
	"${IMAGE}"
