# Vortex Cache

> A high-performance, low-latency, concurrent in-memory key-value cache engine in Go with Redis Serialization Protocol (RESP2) wire compatibility, dual-tier key expiration, pluggable $O(1)$ LRU eviction, master-replica replication, automatic sentinel failover, and a live web topology dashboard.

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https.golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-3--Node%20Cluster-2496ED?logo=docker)](docker-compose.yml)
[![Build & Test](https://img.shields.io/badge/Tests-100%25%20Passing-success)](#verification--testing)
[![Race Detector](https://img.shields.io/badge/Data%20Races-Clean-success)](#concurrency-proof--race-detection)

---

## ⚡ Performance, Failover & Cluster Metrics

Empirical benchmark runs on Apple M4 silicon against live Vortex TCP Server:

| Metric / Workload | Empirical Result | Notes / Details |
| :--- | :--- | :--- |
| **`GET` Throughput** | **212,314 ops/sec** | Standard `redis-benchmark -c 50`, p50 = **0.135 ms** |
| **`SET` Throughput** | **156,986 ops/sec** | Standard `redis-benchmark -c 50`, p50 = **0.151 ms** |
| **Spread Keys (500 Clients)** | **201,029 ops/sec** | p50 = **2.003 ms**, p999 = **9.677 ms** |
| **Hot Key Contention (500 Clients)** | **209,145 ops/sec** | p50 = **2.207 ms**, p999 = **11.616 ms** (writer lock queueing) |
| **Replication Propagation Lag** | **57 µs (0.057 ms)** | 500 parallel writes streamed over TCP |
| **Chaos Failover Recovery Time** | **300.18 ms** | Time from Master process kill (`kill -9`) to promoted Master write readiness |

---

## 🖥️ Live Web Topology Dashboard (`http://localhost:8080`)

Vortex Cache includes a live web status dashboard (`cmd/vortex-dashboard`) designed with an **Obsidian Dark & Champagne Gold** aesthetic:

- **100% Real RESP Info Polling**: Connects over raw TCP sockets to cluster nodes and issues `INFO` commands every 1000ms — **zero mock data**.
- **Live Metrics**: Displays node connection status (`UP`/`DOWN`), per-node role (`MASTER` vs `SLAVE`), stored key count, uptime, master link status, and streaming replication offsets.

---

## 🐳 Docker Compose 3-Node Cluster Setup

Launch a full 3-node cluster (1 Master, 2 Replicas, 1 Web Dashboard) with a single command:

```bash
docker-compose up --build
```

### Cluster Architecture:
- `vortex-master`: Listening on port `6379`
- `vortex-replica-1`: Listening on port `6380`
- `vortex-replica-2`: Listening on port `6381`
- `vortex-dashboard`: Listening on `http://localhost:8080`

---

## 🌐 Deployment Guidance (VPS / Fly.io vs Serverless)

> [!IMPORTANT]
> **Deployment Requirements for In-Memory TCP Caches:**
> - **Persistent TCP Sockets Required:** Vortex Cache relies on raw TCP socket listening (`:6379`) for persistent RESP protocol connections, replication streams, and background failover heartbeats.
> - **Compatible Providers:** Deploy to a Virtual Private Server (VPS) such as Hetzner, DigitalOcean (~$6/mo), AWS EC2, or Fly.io Machines with TCP port exposure (`bgial`).
> - **Incompatible Providers:** Serverless edge platforms (Vercel, Netlify) are stateless HTTP request handlers without persistent raw TCP socket listening or long-lived background goroutines.

---

## 🛡️ Automatic Failover & Split-Brain Trade-off Analysis

### 1. Single-Coordinator Heartbeat Sentinel Scoping
- Replicas ping the Master every **100 ms**. If **3 consecutive heartbeats fail** (~300 ms threshold), the replica automatically executes self-promotion (`SlaveOf("NO", "ONE")`).
- **Measured Chaos Recovery Time:** **300.18 ms** from abrupt Master termination to new Master accepting writes.

### 2. Explicit Split-Brain Risk Acknowledgment
- **The Split-Brain Risk:** In a network partition scenario where Master $M_1$ is isolated from Replica $R_1$ but remains accessible to a partition subset of clients, $R_1$ self-promotes to $M_2$, leading to potential state divergence.
- **Production Remediation Path:** Resolved in production using **Quorum-Based Consensus** (Raft / Paxos / Redis Sentinel Epoch voting) paired with **Fencing Tokens** or node fencing (**STONITH** - *"Shoot The Other Node In The Head"*).

---

## 🚀 Interactive Live Terminal Demo

Run the automated interactive demo script showing node boots, key replication, and live failover self-promotion:

```bash
./scripts/demo.sh
```

---

## 🧪 Verification & Testing

Run full test suite with Go Race Detector (`-race`) enabled:

```bash
go test -v -race -cover ./...
```
