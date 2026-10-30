## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-14 - RWMutex Data Races and CPU Cache Invalidation
**Learning:** In `vortex-cache`'s `internal/store`, the `get` method in shards mutated the `AccessedAt` field of an entry while only holding an `RLock` (Read Lock). This created a severe data race when highly concurrent clients accessed the same key. Furthermore, mutating fields while holding read locks triggers CPU cache line invalidations, severely reducing read throughput across CPU cores. Since eviction policies like LRU rely on callbacks (e.g., `OnGet`) instead of tracking access on the entry itself, `AccessedAt` was completely unused.
**Action:** Never mutate cache entry fields while holding only an `RLock`. Remove unused metadata fields from structs to prevent data races and reduce memory footprint.
