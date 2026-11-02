## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-11-02 - Remove unused field causing race condition
**Learning:** `vortex-cache`'s `internal/store` core cache operations use sharded `sync.RWMutex`. Mutating cache entry fields (like `AccessedAt`) while holding only an `RLock` causes a data race. In vortex-cache, eviction policies (like LRU) are decoupled from the core `Entry` struct and rely on explicit callbacks (e.g., `OnGet`) rather than tracking access metadata within the cache entries themselves.
**Action:** Removed unused `AccessedAt` field from `Entry` struct to eliminate data races during highly concurrent read operations and boost performance by preventing cache invalidations.
