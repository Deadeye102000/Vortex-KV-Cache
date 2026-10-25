## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2026-10-25 - Prevent Data Races and CPU Cache Invalidations on Reads
**Learning:** In highly concurrent read-heavy operations, modifying fields (like `AccessedAt`) on cache entries while holding only an `RLock` causes a data race. Additionally, mutating entries during a read invalidates CPU cache lines across cores, negating the performance benefits of shared memory. In `vortex-cache`, standard data race tests might miss this on hot keys; it requires `go test -race -bench=. ./...` for rigorous detection. Also, eviction policies here are decoupled and rely on callbacks (e.g., `OnGet`), meaning `AccessedAt` metadata inside the `Entry` struct was completely unused.
**Action:** Never mutate cache entry fields while holding only an `RLock`. Remove unused tracking metadata from core struct objects if they aren't actively utilized by the eviction policies to improve read throughput and safety.
