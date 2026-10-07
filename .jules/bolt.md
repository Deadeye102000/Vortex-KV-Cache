## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2026-10-07 - Data Race on Read Operations due to Unused Field Mutation
**Learning:** During high-concurrency read operations (`Get`), updating a struct field like `AccessedAt` under a read lock (`RLock`) causes a critical data race because multiple threads will try to write to it simultaneously. In Vortex-Cache, avoiding `GC` pointer chasing is great, but we must ensure we don't accidentally mutate values during purely read operations.
**Action:** When working on concurrent data structures with reader-writer locks, avoid writing to fields on read paths unless using `sync/atomic` primitives, or if the field is genuinely unused (as was the case here), delete it altogether to remove both the race condition and the CPU overhead.
