## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-10-27 - Remove RLock mutations in cache entries
**Learning:** Mutating cache entry metadata (`AccessedAt`) while holding only an `RLock` causes data races and CPU cache invalidations, significantly degrading concurrent read performance. The cache core's decoupled eviction policies (using `OnGet` callbacks) mean this metadata within `Entry` itself is actually unused and redundant.
**Action:** Never mutate structs protected by an `RLock`. For high-throughput caching, ensure eviction metadata and access tracking are cleanly separated from core reading logic to preserve RCU (Read-Copy-Update) semantics where possible.
