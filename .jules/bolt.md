## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2026-10-09 - Never mutate struct fields during RLock
**Learning:** In highly concurrent Go applications, mutating struct fields (like `AccessedAt`) while holding only an `RLock` causes severe data races and invalidates CPU cache lines across cores. This crushes read throughput.
**Action:** Ensure that operations protected by `RLock` are strictly read-only. Remove state tracking (like access times) from read paths if it's not absolutely critical.
