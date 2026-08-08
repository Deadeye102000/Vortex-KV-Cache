# Vortex Cache — Architectural & Technical Design Document

## 1. Overview & System Design Philosophy

**Vortex Cache** is a concurrent, high-throughput, low-latency in-memory key-value cache built in Go. It is designed to emulate key operational semantics of Redis (such as RESP protocol compliance, dual-tier key expiration, pluggable eviction policies, master-replica replication, automatic sentinel failover, and a live web status dashboard) while leveraging Go's native concurrency primitives and memory management.

---

## 2. Live Web Topology Dashboard (`cmd/vortex-dashboard`)

### 2.1 Real RESP Info Polling Engine

Unlike dashboards that rely on mock data or synthetic state, `vortex-dashboard` executes real TCP network polling against target cluster nodes:

1. **Periodic TCP Dialing:** Every 1000ms, the dashboard dials each target node address (`net.DialTimeout`).
2. **RESP INFO Dispatch:** Sends a standard `INFO` command over the TCP socket using `resp.Writer`.
3. **Metrics Parsing:** Parses raw Redis-compliant `INFO` string metrics (`role`, `master_host`, `master_link_status`, `slave_repl_offset`, `uptime_in_seconds`, `db0:keys`).
4. **JSON API Endpoint:** Exposes parsed metrics via `/api/topology` for the embedded single-page frontend.

---

## 3. Containerization & Docker Compose Topology

- **Multi-Stage Build (`Dockerfile`):** Compiles static Go 1.22 binaries for Alpine Linux.
- **Cluster Orchestration (`docker-compose.yml`):** Spawns 1 Master node (`vortex-master:6379`), 2 Replica nodes (`vortex-replica-1:6380`, `vortex-replica-2:6381`), and 1 Live Dashboard container (`vortex-dashboard:8080`).
