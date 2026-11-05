## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-18 - Read-Lock Mutability Anti-Pattern
**Learning:** Mutating cache entry fields (like `AccessedAt`) while only holding an `RLock` in a highly concurrent map causes data races and triggers CPU cache invalidations, significantly degrading read throughput. In decoupled eviction designs where callbacks (`OnGet`) handle tracking (e.g., in LRU structures), the internal entry metadata is often redundant and harmful to performance.
**Action:** Never mutate cache entry fields while holding only an `RLock`. For access tracking or eviction, rely on explicitly synchronized separate data structures or callbacks rather than embedding mutable access metadata inside core cache entries.
