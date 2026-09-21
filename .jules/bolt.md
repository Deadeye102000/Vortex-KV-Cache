## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-05-11 - Zero-allocation RESP encoding
**Learning:** `vortex-cache` uses `strconv.FormatInt` and `strconv.Itoa` in `internal/resp/writer.go` which allocate strings on the heap for every integer and bulk string length. For a high-performance cache, these allocations cause significant GC pressure.
**Action:** Replace `strconv.FormatInt` and `strconv.Itoa` with `strconv.AppendInt` using a pre-allocated `[32]byte` scratch array inside the `Writer` struct to achieve zero-allocation integer formatting.
