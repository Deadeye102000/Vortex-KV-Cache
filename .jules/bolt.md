## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).
## 2024-10-08 - CPU Cache Invalidation & Data Races on Read
**Learning:** Mutating cache entry struct fields (like `AccessedAt`) while only holding a read lock (`RLock`) in `vortex-cache` not only causes a severe Go data race but also triggers CPU cache line invalidations across cores, significantly degrading read throughput.
**Action:** When inspecting highly concurrent read paths guarded by `RWMutex`, ensure strictly zero memory mutations occur on shared data structures. Remove redundant state fields that encourage such mutations if they are tracked safely elsewhere (like in LRU).
