## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-15 - Mutating Cache Entries Under RLock
**Learning:** Mutating the `AccessedAt` field on cache entries inside `shard.get()` under an `RLock` causes data races and severe read scalability degradation due to CPU cache line invalidation. Since `vortex-cache` decouples eviction policies and relies on callbacks (`OnGet`, etc.), the access metadata within the cache entries themselves is unused and updating it causes unnecessary writes.
**Action:** Never mutate cache entry fields while holding only an `RLock`. Remove unnecessary access metadata fields from entries if they are handled by a separate decoupled eviction policy.
