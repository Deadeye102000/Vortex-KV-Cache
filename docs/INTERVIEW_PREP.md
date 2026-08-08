# Vortex Cache — Principal Systems Engineer Interview Preparation Guide

This document is a comprehensive technical interview guide for **Vortex Cache**. It is structured to help you confidently explain the system architecture, low-level implementation details, trade-offs, concurrency guarantees, and empirical performance metrics in Senior / Staff / Principal Systems Engineering interviews.

---

## 🎯 1. The 30-Second Elevator Pitch

> *"Vortex Cache is an in-memory concurrent key-value cache built in Go from scratch with zero external dependencies. It implements Redis Serialization Protocol (RESP2) wire compatibility, allowing standard tools like `redis-cli` and language SDKs to connect seamlessly. To achieve low latency and high multi-core parallelism, it uses a 16-shard mutex-partitioned storage engine, zero-allocation 64-bit FNV-1a hash routing, a Redis-style dual key expiration engine, an $O(1)$ LRU eviction policy per shard, an asynchronous Master-Replica replication engine with **57 µs replication lag**, automatic sentinel failover with measured **300.18 ms chaos recovery time**, and a live web topology dashboard polling real RESP INFO metrics. Under standard `redis-benchmark` testing, it sustains **>212,000 GET ops/sec** and **>156,000 SET ops/sec** with a p50 latency of **0.135 ms** and clean Go race detector validation."*

---

## 🏗️ 2. Core Architectural Deep Dives

### Deep Dive 1: Automatic Failover Engine & Split-Brain Analysis
- **Heartbeat Sentinel:** Replicas ping Master every 100 ms. 3 missed heartbeats (~300 ms) triggers automatic self-promotion (`SlaveOf("NO", "ONE")`).
- **Chaos Recovery Time:** **300.18 ms** measured recovery time from abrupt Master termination to new Master write readiness.
- **Split-Brain Risk:** Network partitions isolating an old master could lead to state divergence. Production systems resolve this using Quorum-Based Consensus (Raft / Sentinel Epoch Voting) paired with Fencing Tokens or STONITH node fencing.

---

### Deep Dive 2: Live Web Topology Dashboard & Real RESP Polling
- **Zero Mock Data:** The `vortex-dashboard` backend HTTP server dials cluster nodes over TCP, executes `INFO` commands via `resp.Writer`/`resp.Reader`, and streams live node status, replication lag, and key counts.
- **UI Aesthetic:** Built with an Obsidian Dark (`#090d16`) and Champagne Gold (`#f3c677`) glassmorphic card design.

---

### Deep Dive 3: Deployment Requirements (VPS / Fly.io vs Serverless)
- **Why TCP Socket Listening Matters:** In-memory key-value caches require persistent TCP socket connections (`:6379`) for RESP protocol handling, replication streams, and background heartbeat loops.
- **Stateless Serverless Limits:** Serverless platforms (Vercel / Netlify) run short-lived HTTP handlers that terminate after request completion, lacking raw TCP socket listening or persistent background goroutines.

---

## ❓ 3. Systems Interview Q&A on Containerization & Dashboard

### Q: Why can't Vortex Cache be deployed on Vercel or Netlify?
> **Answer:** Vortex Cache is a TCP wire-protocol server listening on raw sockets (`:6379`) with persistent background goroutines for active expiration, replication streams, and sentinel heartbeats. Serverless platforms like Vercel or Netlify are stateless HTTP request/response proxies that cannot expose persistent raw TCP ports or run background event loops. Deployment requires VPS instances (Hetzner / DigitalOcean) or containerized environments (Fly.io / AWS EC2) with raw TCP port exposure.
