## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2025-02-12 - Remove AccessedAt mutation under RLock
**Learning:** Mutating cache entry fields (like `AccessedAt`) while holding only an `RLock` causes a data race in highly concurrent environments and frequently invalidates CPU caches, degrading read throughput. `vortex-cache` eviction policies decouple access tracking via callbacks (e.g., `OnGet`) rather than relying on internal access timestamps on the entries themselves.
**Action:** Never mutate cache entry fields under a shared read lock (`sync.RWMutex.RLock`). Remove unnecessary timestamp updates from read operations.
