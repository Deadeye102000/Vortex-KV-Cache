## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2026-10-26 - Avoid mutating cache entries under RLock
**Learning:** Mutating cache entry fields (like `AccessedAt`) while holding only an `RLock` causes data races and CPU cache invalidations, hurting performance and correctness.
**Action:** Remove `AccessedAt` if it's unused, or rely on explicit eviction callbacks instead of tracking access metadata directly within cache entries during reads.
