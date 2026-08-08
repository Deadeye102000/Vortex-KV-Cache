# Vortex Cache — Architectural & Technical Design Document

## 1. Overview & System Design Philosophy

**Vortex Cache** is a concurrent, high-throughput, low-latency in-memory key-value cache built in Go. It is designed to emulate key operational semantics of Redis (such as RESP protocol compliance, dual-tier key expiration, pluggable eviction policies, and master-replica replication) while leveraging Go's native concurrency primitives and memory management.

### Key Goals:
- **Low Lock Contention:** Minimize lock contention under heavy multi-threaded read/write workloads.
- **Zero-Allocation Key Routing:** Fast, lock-free key-to-shard mapping without memory heap allocations.
- **RESP2 Wire Compatibility:** Native compatibility with standard `redis-cli` and client SDKs without third-party dependencies.
- **Deterministic & Probabilistic Expiration:** Combine instant lazy expiration on read with Redis-style active background sweeping to prevent memory accumulation.
- **Pluggable $O(1)$ Eviction Policies:** Abstract `EvictionPolicy` contract with zero-lock-contention per-shard LRU eviction.

---

## 2. High-Performance Sharded Cache Store (`internal/store`)

### 2.1 Sharded Concurrency Model

Standard Go `sync.Map` is optimized for read-heavy workloads with stable key sets but degrades when keys are frequently created, mutated, or deleted. A single `sync.RWMutex` over a `map[string]*Entry` creates a single-lock bottleneck that chokes under multi-core parallelism.

**Vortex Architecture:**
- The key space is partitioned across $N$ independent **shards** (where $N$ is a power of 2, defaulting to `16` or `64`).
- Each shard contains its own `sync.RWMutex` guarding an independent `map[string]*Entry`.
- Operations on key $K_1$ in Shard $A$ operate completely independently of operations on key $K_2$ in Shard $B$.

```
                      +-----------------------------+
                      |   Client Request (Key: "k") |
                      +--------------+--------------+
                                     |
                          FNV-1a Hash (Zero-Alloc)
                                     |
                        Shard Index = Hash & (N - 1)
                                     |
       +-----------------------------+-----------------------------+
       |                             |                             |
  +----+----+                   +----+----+                   +----+----+
  | Shard 0 |                   | Shard i |                   |Shard N-1|
  | RWMutex |                   | RWMutex |                   | RWMutex |
  |   Map   |                   |   Map   |                   |   Map   |
  +---------+                   +---------+                   +---------+
```

### 2.2 Hash Function & Bitwise Routing

- **Hash Algorithm:** 64-bit FNV-1a (`hash/fnv` logic optimized for inline zero-allocation execution over string bytes).
- **Power-of-2 Shard Indexing:** When $N$ is a power of 2 (e.g., 16, 32, 64), the shard index calculation uses bitwise AND:
  $$\text{ShardIndex} = \text{Hash}(key) \ \& \ (N - 1)$$
  This avoids CPU integer division (`% N`), saving clock cycles on every cache operation.

---

## 3. RESP Wire Protocol & TCP Server (`internal/resp` & `internal/server`)

### 3.1 Custom RESP2 Streaming Parser

Implemented from scratch using `bufio.Reader` for streaming TCP reads:

- **Simple String (`+`):** `+OK\r\n`
- **Error (`-`):** `-ERR unknown command\r\n`
- **Integer (`:`):** `:100\r\n`
- **Bulk String (`$`):** `$5\r\nhello\r\n` or `$-1\r\n` (Null)
- **Array (`*`):** `*2\r\n$3\r\nGET\r\n$4\r\nkey1\r\n` or `*-1\r\n` (Null Array)

### 3.2 TCP Network Loop & Pipelining

```
Client Connection --> bufio.Reader --> resp.ReadValue() --> Handler.Dispatch() --> resp.WriteValue() --> bufio.Writer.Flush()
```

- **Goroutine-Per-Connection:** Accept loop spawns a dedicated goroutine per client TCP connection.
- **Implicit Pipelining:** The reader loop processes consecutive commands in a single TCP read buffer without waiting for network ACKs between commands, flushing buffered replies immediately.

---

## 4. Empirical Benchmarks & Tail Latency Analysis

### Benchmark Results (`redis-benchmark -c 50 -n 100000`)

- **`GET` Throughput:** **212,314 requests/sec**
  - **p50:** 0.135 ms
  - **p95:** 0.191 ms
  - **p99:** 0.239 ms
  - **Max:** 0.679 ms
- **`SET` Throughput:** **156,986 requests/sec**
  - **p50:** 0.151 ms
  - **p95:** 0.375 ms
  - **p99:** 0.551 ms
  - **Max:** 4.519 ms

### Hot-Key Contention Analysis
- Under **500 concurrent clients** targeting spread keys, throughput reaches **201,029 ops/sec** with p999 of **9.67 ms**.
- Under **500 concurrent clients** contending on a single hot-key writer, write lock contention on the single target shard inflates p999 latency to **11.61 ms**.
