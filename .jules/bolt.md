## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-23 - Remove redundant AccessedAt to fix data race and improve read throughput
**Learning:** In highly concurrent code, mutating cache entry fields (like `AccessedAt`) while holding only an `RLock` causes data races and CPU cache invalidations, significantly degrading performance. The `vortex-cache` architecture decouples eviction policies (which track accesses via callbacks), making `AccessedAt` within the core `Entry` struct redundant.
**Action:** Remove redundant access metadata from core cache structs if eviction relies on explicit callbacks. Never mutate fields while holding only an `RLock` to maximize read throughput and avoid data races.
