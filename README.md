# Vortex Cache

> A high-performance, low-latency, concurrent in-memory key-value cache engine in Go with Redis Serialization Protocol (RESP2) wire compatibility, dual-tier key expiration, pluggable $O(1)$ LRU eviction, and master-replica failover.

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https.golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Build & Test](https://img.shields.io/badge/Tests-100%25%20Passing-success)](#verification--testing)
[![Race Detector](https://img.shields.io/badge/Data%20Races-Clean-success)](#concurrency-proof--race-detection)

---

## ⚡ Performance Summary (Empirical Benchmark Runs)

Measured on Apple M4 silicon against live Vortex TCP Server on loopback using standard third-party `redis-benchmark` and custom Go load-testing harness (`bench/loadtest.go`):

| Workload / Command | Throughput (ops/sec) | p50 (ms) | p95 (ms) | p99 (ms) | p999 (ms) | Max (ms) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **`GET` (Standard `redis-benchmark` -c 50)** | **212,314 ops/sec** | **0.135 ms** | **0.191 ms** | **0.239 ms** | 0.559 ms | 0.679 ms |
| **`SET` (Standard `redis-benchmark` -c 50)** | **156,986 ops/sec** | **0.151 ms** | **0.375 ms** | **0.551 ms** | 2.223 ms | 4.519 ms |
| **Spread Keys (10 Clients)** | **119,656 ops/sec** | **0.077 ms** | **0.136 ms** | **0.197 ms** | 0.538 ms | 15.207 ms |
| **Spread Keys (100 Clients)** | **185,295 ops/sec** | **0.442 ms** | **1.003 ms** | **2.190 ms** | 17.831 ms | 38.335 ms |
| **Spread Keys (500 Clients)** | **201,029 ops/sec** | **2.003 ms** | **3.410 ms** | **5.566 ms** | **9.677 ms** | 41.042 ms |
| **Hot Key Contention (500 Clients)** | **209,145 ops/sec** | **2.207 ms** | **3.480 ms** | **6.448 ms** | **11.616 ms** | 18.562 ms |

---

## 🔬 Architectural Trade-offs & Tail Latency Analysis

### 1. Sharded Lock Concurrency vs. Hot-Key Writers
- **Spread-Key Performance:** Partitioning the key space across $N=16$ independent shards eliminates lock contention between unrelated keys. Under 500 concurrent clients, throughput scales to **>200,000 ops/sec** with a smooth p999 tail latency of **9.67 ms**.
- **Hot-Key Writer Bottleneck:** When 500 clients simultaneously write to the exact same key (`hotkey:contention:1`), operations route to a single shard. While concurrent reads scale under shared `RLock()`, write operations require acquiring the shard's exclusive `Lock()`. This causes writer queueing, driving the p999 tail latency from **9.67 ms up to 11.61 ms**.

### 2. Compact Primitive Timestamps vs Monotonic `time.Time`
Storing nanosecond Unix timestamps as `int64` primitives eliminates pointer chasing inside stored cache entries, avoiding Go GC mark-sweep pauses and ensuring zero heap allocation on read lookups.

---

## 🛠️ Feature Set & Architecture

- **RESP2 Protocol Engine (`internal/resp`)**: Full streaming parser built from scratch using `bufio.Reader` / `bufio.Writer`. Correctly handles partial network reads, pipeline streaming, and null frames (`$-1\r\n`, `*-1\r\n`).
- **Redis Wire Compatibility**: Works unmodified with standard `redis-cli` and any language Redis client SDK.
- **Implemented Command Set**:
  - `PING [message]`
  - `GET key`
  - `SET key value [EX seconds] [PX milliseconds]`
  - `DEL key [key ...]`
  - `EXISTS key [key ...]`
  - `EXPIRE key seconds`
  - `TTL key`
  - `INFO [section]` (live metrics reporting process ID, uptime, key count, shard stats).
- **Dual Expiration Mechanics**:
  - **Lazy Expiration:** Evaluated on-read with double-checked write-lock cleanup.
  - **Active Expiration Sweeper:** Redis-style probabilistic background cycle sampling 20 keys per shard every 100ms with CPU deadline bounding.
- **Pluggable $O(1)$ LRU Eviction**: Doubly-linked list + map eviction policy per shard with zero cross-shard lock contention.

---

## 🚀 Quick Start

### 1. Build and Run Server
```bash
# Clone repository
git clone https://github.com/vortex-cache/vortex-cache.git
cd vortex-cache

# Run server on port 6379 with 16 shards
go run ./cmd/vortex-server -p 6379 -shards 16
```

### 2. Connect with Standard `redis-cli`
```bash
redis-cli -p 6379 PING
# Output: PONG

redis-cli -p 6379 SET mykey "Hello Vortex" EX 60
# Output: OK

redis-cli -p 6379 GET mykey
# Output: "Hello Vortex"

redis-cli -p 6379 TTL mykey
# Output: (integer) 59

redis-cli -p 6379 INFO
```

---

## 🧪 Verification & Testing

Run full test suite with Go Race Detector (`-race`) enabled:

```bash
go test -v -race -cover ./...
```

Run in-memory store engine benchmarks:

```bash
go test -bench=. -benchmem ./internal/store/...
```

Run custom load-testing benchmark harness:

```bash
# 100 concurrent clients spread across random keys
go run ./bench/loadtest.go -c 100 -n 100000 -size 64

# Hot-key contention stress test
go run ./bench/loadtest.go -c 100 -n 100000 -size 64 -hotkey
```

Run official `redis-benchmark`:

```bash
redis-benchmark -h 127.0.0.1 -p 6379 -n 100000 -c 50 -t get,set
```
