## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-15 - Mutating struct fields under RLock
**Learning:** Mutating cache entry metadata (like `AccessedAt`) while holding only an `RLock` not only causes a severe data race, but also triggers CPU cache line invalidations across cores, significantly degrading read throughput in high-concurrency environments. Eviction tracking should rely on decoupled callback mechanisms, not by storing access timestamps in the hot-path entry struct.
**Action:** Never mutate cache entry fields in read operations (e.g., `Get`). Ensure all read-heavy structs remain read-only when accessed under an `RLock`.
