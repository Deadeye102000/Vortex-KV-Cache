# Vortex Cache — Principal Systems Engineer Interview Preparation Guide

This document is a comprehensive technical interview guide for **Vortex Cache**. It is structured to help you confidently explain the system architecture, low-level implementation details, trade-offs, concurrency guarantees, and empirical performance metrics in Senior / Staff / Principal Systems Engineering interviews.

---

## 🎯 1. The 30-Second Elevator Pitch

> *"Vortex Cache is an in-memory concurrent key-value cache built in Go from scratch with zero external dependencies. It implements Redis Serialization Protocol (RESP2) wire compatibility, allowing standard tools like `redis-cli` and language SDKs to connect seamlessly. To achieve low latency and high multi-core parallelism, it uses a 16-shard mutex-partitioned storage engine, zero-allocation 64-bit FNV-1a hash routing, a Redis-style dual key expiration engine (lazy + active probabilistic sweeping), and a pluggable $O(1)$ LRU eviction policy per shard. Under standard `redis-benchmark` testing, it sustains **>212,000 GET ops/sec** and **>156,000 SET ops/sec** with a p50 latency of **0.135 ms** and clean Go race detector validation."*

---

## 🏗️ 2. Core Architectural Deep Dives

### Deep Dive 1: Concurrency Model & Sharded Lock Design

#### The Problem with Naive Implementations:
- **Single `sync.RWMutex` over `map[string]*Entry`:** Under multi-core parallelism, every read and write competes for the exact same lock, creating a massive CPU bottleneck.
- **Go's `sync.Map`:** Optimized for read-heavy workloads with stable keys. Degrades under heavy insert/delete operations due to double-map copy and dirty-to-read promotions.

#### The Vortex Solution (Power-of-2 Sharding):
- Partition the key space across $N=16$ independent shards.
- Each shard maintains its own `sync.RWMutex` and `map[string]*Entry`.
- Operations on key $K_1$ in Shard 0 run concurrently with operations on key $K_2$ in Shard 1 without any lock contention.

```
Key String ---> 64-bit FNV-1a Hash ---> Shard Index = Hash & (N - 1) ---> Target Shard Mutex
```

#### Why Power-of-2 Shard Count ($N=16, 32, 64$)?
- Using $N = 2^k$ allows replacing expensive integer modulo operations (`hash % N`) with a single fast bitwise AND operation:
  $$\text{ShardIndex} = \text{Hash}(key) \ \& \ (N - 1)$$
- Bitwise AND executes in 1 CPU clock cycle versus ~15-20 cycles for integer division.

---

### Deep Dive 2: Memory Optimization & GC Footprint Reduction

#### Primitive `int64` Timestamps vs. Go `time.Time`:
- **Standard Approach (`time.Time`):** A Go `time.Time` struct contains internal pointers (`wall uint64`, `ext int64`, `loc *Location`). In a cache storing 10 million items, millions of pointers force the Go Garbage Collector (GC) to perform expensive pointer tracing during concurrent mark-sweep phases.
- **Vortex Design (`int64` Nanoseconds):**
  ```go
  type Entry struct {
      Value      []byte
      CreatedAt  int64 // Unix nanoseconds
      AccessedAt int64 // Unix nanoseconds
      ExpiresAt  int64 // Unix nanoseconds (0 = no expiration)
  }
  ```
- **Benefit:** Primitive `int64` fields contain zero heap pointers, dramatically reducing GC Stop-The-World (STW) pauses.

#### `[]byte` Slice Copying vs. Pointer Referencing:
- When a client calls `Set(key, val)` or `Get(key)`, Vortex creates an explicit byte slice copy (`make([]byte, len(val))`).
- **Why?** Returns value immutability safety. Prevents external caller code from mutating stored cache memory after retrieval or holding references that block GC collection.

---

### Deep Dive 3: Redis-Style Dual Key Expiration Engine

A common shortcut in toy caches is "fake lazy expiration" (checking TTL on read, but never sweeping). In production, keys that are set with a TTL but never read again would leak memory indefinitely. Vortex implements Redis' dual expiration strategy:

#### 1. Lazy Expiration (On-Read):
- Triggered during `Get`, `Exists`, and `TTL` lookups.
- **Double-Checked Lock Upgrade Pattern:**
  1. Acquire shard `RLock`.
  2. If `entry.ExpiresAt > 0 && now >= entry.ExpiresAt`, release `RLock`.
  3. Acquire shard `Lock` (write lock).
  4. **Re-verify expiration** (prevents race conditions if another goroutine mutated/deleted the key in the brief gap between `RUnlock` and `Lock`).
  5. Delete key from `map` and notify eviction policy.
  6. Return `nil, false`.

#### 2. Active Expiration (Background Sweeper):
- Runs in a dedicated background goroutine (`activeExpireLoop`).
- **Interval:** Executes every 100ms.
- **Probabilistic Sampling Algorithm:**
  1. Iterates across all 16 shards.
  2. Samples up to $K=20$ keys in each shard.
  3. Purges any expired keys found.
  4. If **>25%** of sampled keys in a shard were expired, immediately repeats the sweep on that shard.
- **CPU Time Bounding:** The entire cycle is constrained by a maximum CPU deadline (default 1ms). If the deadline is reached, the sweeper yields control to prevent CPU starvation of client request threads.

---

### Deep Dive 4: Pluggable $O(1)$ Eviction Policy Architecture

Vortex decouples capacity eviction through an explicit interface:

```go
type EvictionPolicy interface {
    OnGet(key string)
    OnSet(key string)
    OnDelete(key string)
    SelectEvict() (string, bool)
    Clear()
}
```

#### Per-Shard Zero-Lock Contention LRU:
- Each shard maintains its own `EvictionPolicy` instance (default `lruPolicy`).
- Data structure: Inline doubly-linked list (`lruNode`) + hash map (`map[string]*lruNode`).
  - **Head:** Most Recently Used (MRU).
  - **Tail:** Least Recently Used (LRU -> Eviction Candidate).
- **Lock Advantage:** Because shard methods (`set`, `get`, `delete`) already hold the shard's own mutex lock, LRU updates occur inside the shard lock with **zero extra lock acquisitions** and **zero cross-shard lock contention**.

---

### Deep Dive 5: RESP Wire Protocol & Streaming TCP Server

#### RESP2 Protocol Parser (`internal/resp`):
- Built from scratch using standard library `bufio.Reader` and `bufio.Writer`.
- Supports Simple Strings (`+`), Errors (`-`), Integers (`:`), Bulk Strings (`$`), Arrays (`*`), and Null frames (`$-1\r\n`, `*-1\r\n`).

#### Handling TCP Packet Fragmentation:
- In real network environments, a RESP command string (e.g. `*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$5\r\nmyval\r\n`) can arrive fragmented across multiple TCP packets.
- `resp.Reader` uses `bufio.Reader.ReadByte()`, `readLine()`, and `io.ReadFull()` to wait for complete frame lengths before allocating buffers, guaranteeing stream framing correctness over TCP.

#### Implicit Pipelining:
- Clients (such as `redis-benchmark`) often send multiple commands in a single TCP payload before reading replies.
- The connection loop in `server.go` continuously reads RESP frames, executes commands, and writes to `bufio.Writer`, flushing buffered response bytes immediately to ensure sub-millisecond response latency.

---

## 📊 3. Empirical Benchmarks & Honest Latency Story

### Live `redis-benchmark` Stats (-c 50, -n 100,000):

| Command / Workload | Throughput | p50 Latency | p95 Latency | p99 Latency | p999 Latency |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`GET`** | **212,314 ops/sec** | **0.135 ms** | **0.191 ms** | **0.239 ms** | **0.559 ms** |
| **`SET`** | **156,986 ops/sec** | **0.151 ms** | **0.375 ms** | **0.551 ms** | **2.223 ms** |
| **Spread Keys (500 Clients)** | **201,029 ops/sec** | **2.003 ms** | **3.410 ms** | **5.566 ms** | **9.677 ms** |
| **Hot-Key Contention (500 Clients)** | **209,145 ops/sec** | **2.207 ms** | **3.480 ms** | **6.448 ms** | **11.616 ms** |

### The Honest Systems Engineering Story (Hot-Key Writer Bottleneck):
- **Why does p999 inflate from 9.67ms to 11.61ms under 500 hot-key writers?**
  When 500 clients write to the exact same key (`hotkey:contention:1`), all requests hash to the same shard. While reads execute concurrently under shared `RLock()`, writes require acquiring that shard's exclusive `Lock()`. The resulting write lock serialization creates queueing delay at high concurrency.
- **Interviewer Takeaway:** Explaining this honest tail-latency trade-off proves you understand real hardware and concurrency limits, rather than presenting sanitized benchmark numbers.

---

## ❓ 4. Top 10 Technical Interview Questions & Answers

### Q1: Why did you build a sharded map instead of using Go's `sync.Map`?
> **Answer:** `sync.Map` is optimized for append-only or read-heavy workloads with stable keys. When keys are frequently created, updated, or deleted (typical cache behavior), `sync.Map` experiences heavy contention and cache misses due to its internal read/dirty map promotion mechanism. Our 16-shard design partitions key storage across independent `sync.RWMutex` locks, guaranteeing that operations on different keys execute in parallel without lock contention.

### Q2: How do you map keys to shards without memory allocations?
> **Answer:** We compute a 64-bit FNV-1a hash over the string bytes directly. Because the shard count $N=16$ is a power of 2, we compute the target shard index using bitwise AND: `hash & (N - 1)`. This avoids heap allocations and eliminates CPU integer division (`% N`), executing in a single CPU cycle.

### Q3: How do you handle race conditions during lazy key expiration?
> **Answer:** Lazy expiration uses a double-checked locking pattern. We first read the key under a shared `RLock`. If expired, we release `RLock` and acquire exclusive `Lock`. Before deleting, we re-verify that the key is still present and expired to prevent race conditions in case another goroutine updated or deleted the key in between lock transitions.

### Q4: How does your active background sweeper prevent CPU starvation?
> **Answer:** The active sweeper runs every 100ms and samples up to 20 keys per shard. If >25% of sampled keys are expired, it repeats the sweep on that shard. To prevent CPU starvation under heavy expiration workloads, the sweeper evaluates a strict CPU deadline (default 1ms). If `time.Now().After(deadline)`, it yields execution until the next cycle.

### Q5: Why is your LRU eviction policy scoped per shard rather than globally?
> **Answer:** Global LRU structures require a single global lock over the access list, introducing a global bottleneck across all threads. By maintaining a dedicated $O(1)$ doubly-linked list per shard, LRU updates occur inside the shard's existing read/write lock with zero extra lock acquisitions and zero cross-shard lock contention.

### Q6: How does your RESP parser handle partial TCP packet reads?
> **Answer:** TCP is a streaming byte protocol that does not guarantee message boundary alignment. Our parser wraps socket connections in `bufio.Reader` and uses `io.ReadFull()` to read exact payload lengths parsed from RESP bulk header prefix bytes (`$length\r\n`). If a packet arrives fragmented, `ReadFull` pauses until remaining bytes arrive on the TCP socket.

### Q7: How does Vortex Cache support command pipelining?
> **Answer:** The TCP server connection loop reads RESP frames sequentially from the socket buffer without waiting for client ACK round-trips. It executes commands against the store and writes responses to a `bufio.Writer`, flushing buffered response bytes after processing incoming frames. This allows client batches of 100+ commands to execute in a single round-trip.

### Q8: How did you verify the absence of data races in your store?
> **Answer:** We wrote a multi-threaded stress test spawning 32 concurrent goroutines performing 16,000 mixed CRUD operations and continuous LRU evictions, validated using Go's data race detector: `go test -v -race -cover ./...`. All tests pass 100% clean with zero data race warnings.

### Q9: How would you scale Vortex Cache to a distributed cluster?
> **Answer:** To scale beyond a single node, we would implement **Consistent Hashing with Virtual Nodes** across a cluster topology (similar to Redis Cluster or Dynamo). Clients or a proxy layer (e.g., Envoy/Twemproxy) route keys to master nodes using CRC16 / MurmurHash3 modulo 16,384 hash slots.

### Q10: How would master-replica replication be implemented in Phase 5?
> **Answer:** Master nodes would maintain an asynchronous ring buffer replication log and offset sequence number (`master_repl_offset`). Replicas establish a TCP connection, issue a `PSYNC <run_id> <offset>` command, and stream incoming write command frames to replicate state in background goroutines.

---

## 🛠️ 5. Key File Cheat Sheet

- [README.md](file:///Users/Deadeye/Desktop/Projects/Vortex%20/README.md) — Main benchmark summary & quickstart.
- [ARCHITECTURE.md](file:///Users/Deadeye/Desktop/Projects/Vortex%20/docs/ARCHITECTURE.md) — Technical architecture design document.
- [store.go](file:///Users/Deadeye/Desktop/Projects/Vortex%20/internal/store/store.go) — Store manager & active sweeper.
- [shard.go](file:///Users/Deadeye/Desktop/Projects/Vortex%20/internal/store/shard.go) — Mutex-guarded shard implementation.
- [lru.go](file:///Users/Deadeye/Desktop/Projects/Vortex%20/internal/store/lru.go) — $O(1)$ doubly-linked list LRU eviction policy.
- [reader.go](file:///Users/Deadeye/Desktop/Projects/Vortex%20/internal/resp/reader.go) — Streaming RESP reader.
- [writer.go](file:///Users/Deadeye/Desktop/Projects/Vortex%20/internal/resp/writer.go) — Fast RESP encoder.
- [handler.go](file:///Users/Deadeye/Desktop/Projects/Vortex%20/internal/server/handler.go) — Redis command dispatcher.
- [server.go](file:///Users/Deadeye/Desktop/Projects/Vortex%20/internal/server/server.go) — Multi-threaded TCP server with pipelining.
- [loadtest.go](file:///Users/Deadeye/Desktop/Projects/Vortex%20/bench/loadtest.go) — Latency percentile benchmark harness.
