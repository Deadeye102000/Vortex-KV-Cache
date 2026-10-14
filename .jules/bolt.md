## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2026-10-14 - Struct mutations under RLock
**Learning:** `vortex-cache` `get` queries mutated `entry.AccessedAt` while only holding `RLock` in the shard. In highly concurrent scenarios, this not only causes a data race, but forces cross-core CPU cache invalidations, hurting read throughput.
**Action:** Never mutate cache entry fields while holding only an `RLock`. Avoid adding timestamp mutations to read paths if they can be handled by separate structures (like the LRU tracking `Nodes`).
