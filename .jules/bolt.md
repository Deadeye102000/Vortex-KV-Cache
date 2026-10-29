## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-06-03 - Remove AccessedAt mutation under RLock
**Learning:** Mutating cache entry fields (like `AccessedAt`) under an `RLock` causes data races and CPU cache invalidations. Eviction policies are decoupled and rely on explicit callbacks (`OnGet`, `OnSet`, `OnDelete`), so `AccessedAt` is redundant and unnecessary.
**Action:** Remove `AccessedAt` from the `Entry` struct and avoid mutating it during read operations to maximize read throughput.
