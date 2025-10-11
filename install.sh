#!/bin/bash

echo "Applying kernel optimizations for high-performance networking"

# Increase the maximum number of pending connections
sudo sysctl -w net.core.somaxconn=131072

# Increase the network backlog to prevent packet loss
sudo sysctl -w net.core.netdev_max_backlog=500000

# Enable fast reuse of closed connections
sudo sysctl -w net.ipv4.tcp_tw_reuse=1
sudo sysctl -w net.ipv4.tcp_tw_recycle=1

# Reduce TCP FIN timeout to free up sockets faster
sudo sysctl -w net.ipv4.tcp_fin_timeout=10

# Increase the max SYN backlog to handle more concurrent connections
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=262144

# Disable syncookies (only safe if no SYN flood attacks expected)
sudo sysctl -w net.ipv4.tcp_syncookies=0

# Optimize TCP memory buffers
sudo sysctl -w net.ipv4.tcp_mem="786432 1048576 1572864"
sudo sysctl -w net.ipv4.tcp_rmem="4096 87380 6291456"
sudo sysctl -w net.ipv4.tcp_wmem="4096 87380 6291456"

# Expand available local port range to prevent exhaustion
sudo sysctl -w net.ipv4.ip_local_port_range="1024 65535"

# Enable TCP Fast Open to reduce connection latency
sudo sysctl -w net.ipv4.tcp_fastopen=3

# Increase the maximum number of open files for high concurrency
sudo sysctl -w fs.file-max=2097152
ulimit -n 1048576

# Optimize TCP keepalive settings
sudo sysctl -w net.ipv4.tcp_keepalive_time=60
sudo sysctl -w net.ipv4.tcp_keepalive_intvl=10
sudo sysctl -w net.ipv4.tcp_keepalive_probes=5

echo "Kernel optimizations applied successfully!"
docker run -p 7171:7171 --ulimit nofile=1048576:1048576 --network=host rushhaabhhh/key-value-cache