## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-10-22 - Remove unused AccessedAt mutation under RLock
**Learning:** Mutating the `AccessedAt` metadata field in the `Entry` struct during a read operation (while holding only `RLock`) is a data race. Moreover, it causes CPU cache invalidations, hurting read throughput on concurrent workloads. Since `AccessedAt` wasn't being used by LRU eviction (which uses explicit callbacks instead), it's unnecessary overhead.
**Action:** Removed `AccessedAt` from `Entry` and its assignment in `shard.get()` under `RLock`. This resulted in improved read benchmark speeds (from ~164 ns/op down to ~149 ns/op).
