## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-11 - Remove AccessedAt mutation on read path
**Learning:** Mutating an `AccessedAt` field inside an `RLock` section causes data races and triggers CPU cache invalidation, which significantly degrades concurrent read throughput. In `vortex-cache`, eviction policies are decoupled from the core `Entry` and use explicit callbacks, so tracking access inside `Entry` is unnecessary.
**Action:** Remove `AccessedAt` from cache entries to prevent write-on-read data races and ensure zero-mutation read paths for maximum concurrency.
