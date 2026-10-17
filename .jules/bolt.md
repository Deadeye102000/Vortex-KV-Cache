## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-11 - Remove AccessedAt tracking on read
**Learning:** Mutating an entry's `AccessedAt` field while holding only an `RLock` (Read Lock) causes a data race. Beyond the data race, mutating memory on read paths invalidates CPU cache lines across cores, causing severe contention in high-throughput concurrent workloads. `vortex-cache` eviction policies (like LRU) are decoupled and handle their own access tracking via `OnGet`/`OnSet` callbacks, making the `AccessedAt` field on the cache `Entry` entirely redundant.
**Action:** Remove `AccessedAt` from the `Entry` struct and avoid any mutation under read locks to improve read throughput and eliminate CPU cache invalidation overhead.
