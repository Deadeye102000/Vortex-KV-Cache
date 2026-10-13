## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).
## 2024-05-15 - Fix Data Race on RLock
**Learning:** Mutating cache entry fields (like `AccessedAt`) while holding only an `RLock` causes a data race, leading to CPU cache invalidations and panics in concurrent operations.
**Action:** Used `atomic.StoreInt64` to safely update the accessed time without needing a full write lock, preserving high read throughput.
