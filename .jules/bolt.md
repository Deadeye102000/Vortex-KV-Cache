## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-10-01 - Remove unused fields `CreatedAt` and `AccessedAt` from `Entry` and data race fix
**Learning:** Found data races on `entry.AccessedAt` during highly concurrent reads. By inspecting the codebase, realized `CreatedAt` and `AccessedAt` fields in the core `Entry` struct are totally unused for any business logic (expiration relies on `ExpiresAt`, LRU uses a separate linked-list node). Modifying `AccessedAt` inside a read lock (`RLock`) was causing a severe data race, and keeping the fields added an extra 16 bytes per entry, increasing memory usage and GC overhead.
**Action:** Removed `CreatedAt` and `AccessedAt` from the `Entry` struct and deleted the assignment `entry.AccessedAt = now`. This prevents a concurrent data race during `Get`, saves 16 bytes per entry, and improved read throughput by ~15% in benchmarks.
