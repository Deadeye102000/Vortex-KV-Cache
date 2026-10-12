## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-10-12 - Remove AccessedAt mutation under RLock
**Learning:** In highly concurrent systems using `sync.RWMutex`, mutating a field (like `AccessedAt`) on a shared struct while holding only an `RLock` causes severe data races. Even if data races were ignored, mutating data under an `RLock` forces CPU cache line invalidations across cores, significantly degrading read throughput.
**Action:** Remove `AccessedAt` from `Entry` or avoid mutating it during read operations to ensure true lock-free scaling for read-heavy workloads.
