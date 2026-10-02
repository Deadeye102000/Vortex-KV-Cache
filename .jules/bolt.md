## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-11 - RWMutex overhead vs atomic.Bool
**Learning:** Checking a store shutdown state (`s.closed bool`) via an `RWMutex` takes a severe toll on concurrent performance due to locking overhead (even read locks) under high contention.
**Action:** Use `atomic.Bool` instead for boolean state flags accessed across multiple hot paths. It provides lock-free concurrency, dropping overhead significantly per read/write cycle.
