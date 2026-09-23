## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).
## 2024-05-11 - Replace strconv.Itoa/FormatInt with strconv.AppendInt
**Learning:** `strconv.Itoa` and `strconv.FormatInt` perform dynamic heap allocations for their return strings, increasing garbage collection (GC) pressure for high-throughput operations. `strconv.AppendInt` can format directly into an existing byte slice, achieving zero-allocation integer formatting.
**Action:** Replace `strconv.Itoa` and `strconv.FormatInt` with `strconv.AppendInt` targeting a pre-allocated scratch buffer in `Writer` struct inside `internal/resp/writer.go` for `WriteInteger`, `WriteBulkString` length, and `WriteArray` size formatting to eliminate allocations and improve performance.
