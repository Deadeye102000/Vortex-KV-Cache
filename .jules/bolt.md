## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-10-15 - Mutating under RLock causes data races
**Learning:** Mutating the `AccessedAt` field of cache entries while only holding an `RLock` during `GET` operations caused severe data races and CPU cache invalidations under high concurrency.
**Action:** Removed the unused `AccessedAt` field from `Entry` to maintain purely read-only semantics during `GET` operations, eliminating the data race and saving 8 bytes per entry.
