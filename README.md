# Key-Value-Cache - High-Performance Key-Value Store

## Overview
Key-Value-Cache is a high-performance in-memory key-value store built with Go and [fasthttp](https://github.com/valyala/fasthttp). It is designed for low-latency, high-throughput workloads and scales its internals automatically to whatever CPU quota it's actually given — a laptop, a bare-metal box, or a 2-vCPU container — instead of assuming a fixed core count.

## Features
- **Sharded LRU cache** — true least-recently-used eviction per shard (`container/list` + map), not "delete an arbitrary key."
- **CPU-agnostic scaling** — shard count and `GOMAXPROCS` derive from the CPU quota actually granted to the process (via [`go.uber.org/automaxprocs`](https://github.com/uber-go/automaxprocs)), so the binary is tuned correctly whether it's on a 4-core laptop or a `--cpus=2` container on a 64-core host.
- **DoS-resistant hashing** — shard routing uses `hash/maphash` (randomly seeded per process) instead of a fixed hash like djb2, so an attacker can't craft keys that all collide onto one shard to create a lock-contention denial of service.
- **Allocation-light hot path** — pooled request structs (`sync.Pool`) and hand-written JSON response encoding avoid `encoding/json`'s reflection overhead on `/put` and `/get`.
- **Fast HTTP API** — powered by `fasthttp` for ultra-low latency, with graceful shutdown on `SIGINT`/`SIGTERM`.
- **Configurable via environment variables** — no rebuild needed to change capacity, shard count, timeouts, or port.
- **Operable** — `/health` and `/stats` endpoints, structured startup logging, Docker `HEALTHCHECK`.

## Architecture & Design Choices

### Sharded cache with real LRU eviction
Each shard is a `map[string]*list.Element` backed by a doubly linked list. `Get` and `Put` both move the touched entry to the front of the list; when a shard is full, the back of the list (the actual least-recently-used entry) is evicted in O(1). This replaces the previous behavior of deleting whatever key a map iteration happened to land on first.

This does mean `Get` takes the shard's mutex exclusively (to update recency), instead of a `sync.RWMutex` read-lock as before. That's a deliberate trade-off: pure-read microbenchmarks on a single shard are slightly slower per-op, but it buys correct LRU semantics, and the shard-count change below more than compensates for it in realistic mixed read/write traffic (see [Benchmarks](#benchmarks)).

### Shard count scales with the real CPU quota, not `runtime.NumCPU()`
The original code hardcoded 4 shards regardless of machine size, and separately called `runtime.GOMAXPROCS(runtime.NumCPU())`, which reads the *host's* core count — wrong inside a CPU-limited container. This version:
1. Calls `automaxprocs.Set()` first, which detects a cgroup CPU quota (if any) and sets `GOMAXPROCS` to match it.
2. Derives shard count as `nextPow2(clamp(GOMAXPROCS * 4, 16, 1024))`.

So a `docker run --cpus=2` container gets `GOMAXPROCS=2` and 16 shards; a bare-metal 64-core box gets `GOMAXPROCS=64` and 256 shards — both proportional to actual available parallelism, not a guess.

### Capacity is a real, configurable budget
`CACHE_CAPACITY` (default `2,800,000`, matching the original's implicit `4 shards × 700,000`) is divided evenly across shards at startup, so total memory use has a predictable ceiling regardless of how many shards that gets split into.

### Why not fasthttp Prefork / SO_REUSEPORT
fasthttp's `Prefork` mode forks multiple OS processes for more accept-loop parallelism, but each process gets its own memory — for a stateful in-memory cache that would silently partition the keyspace (a `PUT` could land in a process a later `GET` never reaches). It's intentionally not used here.

## Configuration
All settings are environment variables with safe defaults — no code changes needed:

| Variable | Default | Description |
|---|---|---|
| `HOST` | `` (all interfaces) | Bind address |
| `PORT` | `7171` | Bind port |
| `SHARD_COUNT` | `0` (auto) | Force a shard count (rounded up to a power of 2); `0` = derive from `GOMAXPROCS` |
| `CACHE_CAPACITY` | `2800000` | Total items across all shards |
| `MAX_KV_SIZE` | `256` | Max bytes for a key or value |
| `READ_TIMEOUT_SECONDS` | `5` | Per-connection read timeout |
| `WRITE_TIMEOUT_SECONDS` | `5` | Per-connection write timeout |
| `IDLE_TIMEOUT_SECONDS` | `60` | Max keep-alive idle duration |
| `SHUTDOWN_TIMEOUT_SECONDS` | `10` | Grace period for in-flight requests on shutdown |
| `GOMEMLIMIT` | unset | Recommended in containers: set to ~90% of the memory limit so Go's GC stays ahead of an OOM kill |

## Installation & Setup

### Build and run locally
```sh
make build     # compiles ./key-value-cache
make run       # build + run
```
Requires Go 1.25+.

### Building & Running the Docker Image
```sh
make docker-build
make docker-run          # runs with --memory=1g --cpus=2 to demonstrate cgroup-aware scaling
```
or directly:
```sh
docker build -t key-value-cache .
docker run -p 7171:7171 --ulimit nofile=1048576:1048576 --network=host key-value-cache
```

### Optimized Kernel Parameters (bare-metal/VM Linux hosts)
`install.sh` applies kernel networking tunables for high connection-count workloads (backlog sizes, TCP buffer sizes, port range, etc.), then builds and runs the container on host networking:
```sh
./install.sh
```
Each `sysctl` is applied individually and skipped (not failed) if unsupported by the running kernel — safe to run in containers/CI where `/proc/sys` may be read-only. Skip this script on shared infrastructure you don't control (k8s nodes, CI runners) — use `make docker-run` there instead.

## API Endpoints

### `POST /put`
Stores a key-value pair.
```sh
curl -X POST "http://localhost:7171/put" -H "Content-Type: application/json" -d '{"key":"name", "value":"KeyValueCache"}'
# {"status":"OK","message":"Key inserted/updated successfully."}
```

### `GET /get?key={key}`
Retrieves a value by key.
```sh
curl "http://localhost:7171/get?key=name"
# {"status":"OK","key":"name","value":"KeyValueCache"}
```
A miss returns `{"status":"ERROR","message":"Key not found."}` with HTTP 200, unchanged from the original API contract.

### `GET /health`
Liveness probe for Docker/Kubernetes. Returns `{"status":"OK"}`.

### `GET /stats`
Basic runtime introspection: `{"status":"OK","shards":64,"keys":1,"goroutines":6,"gomaxprocs":16}`.

All error responses (`400`, `404`, `405`) are now structured JSON (`{"status":"ERROR","message":"..."}`) instead of the original plain-text fasthttp default.

## Testing
```sh
make test    # unit tests (cache correctness, LRU eviction, HTTP handlers)
make race    # same, with the race detector
make cover   # with coverage
```

## Benchmarks

### How to reproduce
```sh
# 1. Pure cache-logic benchmarks (no network — isolates lock/hash/eviction cost)
make bench

# 2. End-to-end HTTP benchmarks
make build && ./key-value-cache &
go install github.com/rakyll/hey@latest
hey -n 300000 -c 300 -m GET "http://localhost:7171/get?key=benchkey"
```
`ab` (Apache Bench) is single-threaded and becomes the bottleneck before this server does on any modern multi-core machine — use a concurrent client like `hey`, `wrk`, or `k6` for meaningful numbers. Even then, on a same-machine test the load-generator competes with the server for CPU; treat single-box HTTP numbers as a sanity check, not a capacity ceiling. The Go benchmarks in `make bench` isolate server-side cost from client-side cost and are the more trustworthy signal.

### Results (AMD Ryzen 7 5800HS, 16 threads, measured 2026-08-08)

**Cache-level (`go test -bench`, no network, 16 parallel goroutines):**

| Benchmark | Before (4 fixed shards) | After (64 auto-scaled shards, LRU) |
|---|---|---|
| Mixed Put+Get | 278.4 ns/op, 1 alloc/op | **41.0 ns/op (6.8x faster), 2 allocs/op** |
| Get-only | 24.4 ns/op, 0 allocs/op | 32.5 ns/op, 0 allocs/op |

The mixed workload — the realistic case — is ~7x faster because contention drops sharply once shard count scales with CPU count instead of being fixed at 4. Pure get-only throughput on a single, uncontended shard is modestly lower per-op because `Get` now takes an exclusive lock to maintain true LRU order (see [Architecture](#architecture--design-choices)); under real contention this is outweighed by the 16x increase in shard count.

**HTTP end-to-end (`hey`, n=300000, c=300, same machine, client-bound — see caveat above):**

| | Before | After |
|---|---|---|
| GET req/s | ~149,000 | ~152,000 |
| PUT req/s | ~96,000 (ab) | ~142,000 |
| p99 latency | 13.2ms | 13.1ms |

These are within noise of each other because the load generator saturates first on this hardware; they're included to show no regression at the HTTP layer, not as the primary evidence of improvement — the cache-level benchmarks above are.

### Correctness under capacity pressure
Verified via `/stats`: with `CACHE_CAPACITY=1000`, inserting 5,000 keys leaves exactly 1,000 resident, with the most recently written/read keys surviving and older ones evicted — confirming LRU (not arbitrary) eviction.
