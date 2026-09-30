## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).

## 2024-09-30 - Zero-allocation byte-to-int parsing and formatting
**Learning:** `strconv.ParseInt(string(bytes), ...)` and `strconv.Atoi(string(bytes))` cause a heap allocation because they create a new string from the byte slice. We can avoid this allocation using `unsafe.String(unsafe.SliceData(bytes), len(bytes))`. Similarly, `strconv.FormatInt` and `strconv.Itoa` allocate strings, whereas `strconv.AppendInt` can write directly to a pre-allocated stack buffer (e.g. `[32]byte`), completely eliminating heap allocations in high-throughput hot paths like serialization and deserialization.
**Action:** Use `unsafe.String(unsafe.SliceData(bytes), len(bytes))` for converting `[]byte` to `string` specifically for `strconv` calls. Use `strconv.AppendInt(buf[:0], n, 10)` instead of `strconv.Itoa` or `strconv.FormatInt` to format integers without string allocations.
