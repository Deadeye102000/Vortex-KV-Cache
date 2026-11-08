## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-11-08 - Remove unused AccessedAt to fix data race and cache invalidation
**Learning:** Mutating an entry's field (like `AccessedAt`) under an `RLock` causes a data race. Even if it were safe, it triggers CPU cache line invalidations across cores, significantly degrading read-heavy throughput. In this architecture, eviction relies on callbacks (e.g., `OnGet`) instead of embedded access metadata, making `AccessedAt` entirely redundant.
**Action:** Remove `AccessedAt` from cache entries to eliminate data races and prevent false sharing/cache invalidation during highly concurrent reads. Never mutate struct fields while holding only a read lock.
