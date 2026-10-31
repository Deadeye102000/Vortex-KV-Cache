## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-10-31 - Remove RLock mutations in cache entries
**Learning:** In `vortex-cache`'s `internal/store`, core cache operations use sharded `sync.RWMutex`. Mutating cache entry fields (like `AccessedAt`) while holding only an `RLock` causes data races and CPU cache invalidations, hurting performance and safety. Eviction policies (like LRU) are decoupled and handle access metadata separately via explicit callbacks.
**Action:** Never mutate cache entry fields while holding only an `RLock`. Remove access time updates from `RLock` critical sections.
