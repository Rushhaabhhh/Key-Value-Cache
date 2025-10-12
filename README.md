# Key-Value-Cache - High-Performance Key-Value Store

## Overview
Key-Value-Cache is a high-performance in-memory key-value store built with Golang and fasthttp. It is designed for low-latency, high-throughput workloads with WebSocket communication support.

## Features
- **Sharded Cache**: Uses multiple shards to improve concurrency.
- **Fast HTTP API**: Powered by `fasthttp` for ultra-low latency.
- **Optimized Kernel & Docker Settings**: Tuned for high traffic and maximum efficiency.
- **Efficient Memory Management**: Uses a hash-based approach with controlled eviction.

## Installation & Setup

### **Building & Running the Docker Image**
```sh
docker build -t key-value-cache .
docker run -p 7171:7171 --ulimit nofile=1048576:1048576 --network=host key-value-cache
```

### **Optimized Kernel Parameters**
To handle high connection loads efficiently, apply these system-level optimizations:
```sh
sudo sysctl -w net.core.somaxconn=131072
sudo sysctl -w net.core.netdev_max_backlog=500000
sudo sysctl -w net.ipv4.tcp_tw_reuse=1
sudo sysctl -w net.ipv4.tcp_tw_recycle=1
sudo sysctl -w net.ipv4.tcp_fin_timeout=10
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=262144
sudo sysctl -w net.ipv4.tcp_syncookies=0
sudo sysctl -w net.ipv4.tcp_mem="786432 1048576 1572864"
sudo sysctl -w net.ipv4.tcp_rmem="4096 87380 6291456"
sudo sysctl -w net.ipv4.tcp_wmem="4096 87380 6291456"
sudo sysctl -w net.ipv4.ip_local_port_range="1024 65535"
sudo sysctl -w net.ipv4.tcp_fastopen=3
sudo sysctl -w fs.file-max=2097152
sudo sysctl -w net.ipv4.tcp_keepalive_time=60
sudo sysctl -w net.ipv4.tcp_keepalive_intvl=10
sudo sysctl -w net.ipv4.tcp_keepalive_probes=5
```

### **Design Choices & Optimizations**
#### **1. Sharded Cache Design**
- Uses **consistent hashing (djb2)** to distribute keys across shards.
- Reduces contention by locking only at the shard level.

#### **2. Kernel-Level Optimizations**
- **Increased backlog & connection queue**: Prevents dropped requests under heavy traffic.
- **TCP optimizations**: Reduces `TIME_WAIT` sockets, improves port reuse, and speeds up connections.
- **Memory tuning**: Optimized TCP buffers for large-scale traffic.

#### **3. Docker Optimizations**
- **`--ulimit nofile=1048576:1048576`**: Avoids hitting file descriptor limits.
- **Host networking (`--network=host`)**: Reduces network overhead.

## API Endpoints
### **PUT /put**
Stores a key-value pair.
```sh
curl -X POST "http://localhost:7171/put" -H "Content-Type: application/json" -d '{"key":"name", "value":"KeyValueCache"}'
```

### **GET /get?key={key}**
Retrieves a value by key.
```sh
curl -X GET "http://localhost:7171/get?key=name"
```