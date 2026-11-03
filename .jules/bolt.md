## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).
## 2024-05-11 - Remove Access Mutation under RLock
**Learning:** Mutating cache entries (e.g. updating an `AccessedAt` field) while holding an `RLock` leads to data races and CPU cache invalidations, hurting read throughput. Eviction access metadata should be explicitly tracked via decoupling into eviction policy structures instead of directly on cache entries.
**Action:** When tracking access metrics for eviction algorithms under concurrent setups, do not mutate state within `RLock` scopes. Rely on external structures or upgrade to write locks where mutation is necessary.
