## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-10 - Do not mutate fields under RLock
**Learning:** In `vortex-cache`'s `internal/store`, core cache operations use sharded `sync.RWMutex`. Mutating cache entry fields (like `AccessedAt`) while holding only an `RLock` causes data races and CPU cache invalidations, hurting performance and correctness.
**Action:** Never mutate cache entry fields while holding only an `RLock`. Rely on explicit callbacks (e.g., `OnGet`) under write locks for tracking access metadata, rather than tracking access metadata within the cache entries themselves.
