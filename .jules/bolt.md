## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-10-16 - Prevent Mutation on Read under RLock
**Learning:** In `vortex-cache`, mutating cache entries on read (e.g., updating an `AccessedAt` field) while holding only an `RLock` causes data races and triggers CPU cache line invalidations, severely reducing concurrent read performance. The LRU eviction policy correctly relies on explicit factory callbacks (`OnGet`) rather than embedding access metadata into the entry itself, rendering `AccessedAt` redundant.
**Action:** Remove `AccessedAt` from the core `Entry` struct and avoid mutating state during read (`get`) operations to enable lock-free/concurrent-safe reads without memory contention.
