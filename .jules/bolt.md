## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2026-11-06 - Avoid mutating cache entries under RLock
**Learning:** Mutating cache entry fields (like `AccessedAt`) while holding only an `RLock` causes data races and CPU cache invalidations, significantly degrading read throughput in high-concurrency environments.
**Action:** Never mutate cache entry fields under `RLock`. If eviction tracking is needed, delegate it to explicit eviction policies (like LRU) which can manage their own locking or use lock-free mechanisms.
