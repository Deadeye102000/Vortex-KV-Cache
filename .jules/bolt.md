## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-11-04 - Remove struct field mutation under RLock
**Learning:** Mutating an `AccessedAt` timestamp on a cache entry struct while only holding a `sync.RWMutex` `RLock` causes a data race on highly concurrent reads to the same key, and triggers CPU cache invalidation, reducing read throughput. `vortex-cache` decoupled eviction policies track access separately, meaning access time on the entry struct is unnecessary and causes contention.
**Action:** Never mutate cache entry fields while holding only an `RLock`. Remove unnecessary tracking fields from core structs when they are handled independently by other mechanisms (like eviction policies).
