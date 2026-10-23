## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).
## 2026-10-23 - Remove Entry Mutation under RLock
**Learning:** In `vortex-cache`, mutating struct fields (like `AccessedAt`) while only holding a read lock (`s.mu.RLock()`) causes data races and triggers CPU cache invalidations across cores, significantly degrading read throughput. Since eviction policies in `vortex-cache` are decoupled and rely on callbacks (e.g., `OnGet`), tracking access time directly on the `Entry` struct was unnecessary.
**Action:** Remove struct mutations under `RLock` during `Get()` operations to ensure thread safety and maximize concurrent read performance.
