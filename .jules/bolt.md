## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-15 - Remove AccessedAt to fix data race under RLock
**Learning:** Mutating an entry's state (like `AccessedAt`) during a cache `GET` operation while only holding a read lock (`RLock`) causes a data race in highly concurrent environments. It also adds unnecessary memory contention and overhead when the field is unused.
**Action:** Avoid mutating cache entry state under read locks. If access tracking is strictly needed, consider using atomic operations, lock-free structures, or segregating read/write states.
