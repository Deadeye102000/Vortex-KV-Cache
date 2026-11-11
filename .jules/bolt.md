## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-11 - Remove AccessedAt mutation under RLock
**Learning:** In highly concurrent sharded caches using `sync.RWMutex`, mutating cache entry metadata (like `AccessedAt`) while holding only an `RLock` causes data races and CPU cache invalidations, hurting performance and correctness. The cache uses decoupled eviction policies which rely on explicit callbacks (`OnGet`), eliminating the need to store this state directly on entries.
**Action:** Removed `AccessedAt` from `Entry` to save memory, and removed the mutation under `RLock` to fix data races and CPU cache thrashing.
