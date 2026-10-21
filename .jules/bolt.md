## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-10 - Do not mutate fields under RLock in vortex-cache
**Learning:** Mutating cache entry fields (like `AccessedAt`) while holding only an `RLock` in `vortex-cache`'s `internal/store` core causes severe data races under high concurrency. Eviction policies here are decoupled and rely on explicit callbacks, so tracking access time inside the entry structure itself isn't needed.
**Action:** Removed the `AccessedAt` field from `Entry` to prevent data races and CPU cache invalidations, saving memory per entry in the process. Ensure read operations are strictly read-only within the cache's core map structures.
