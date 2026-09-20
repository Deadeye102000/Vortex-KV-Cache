## 2024-05-10 - Replace FNV-1a with maphash
**Learning:** `vortex-cache` uses a custom FNV-1a hash implementation for routing keys to shards. Go's built-in `hash/maphash` is significantly faster (especially on architectures with AES-NI instructions) because it's implemented in assembly and uses a fast hash algorithm (based on AES/Wyhash).
**Action:** Replace custom FNV-1a hashing with `hash/maphash` to speed up key routing across all operations (`GET`, `SET`, `DEL`, etc.).
## 2024-05-10 - Replace RWMutex with atomic.Bool for store closed state
**Learning:** Checking a sync.RWMutex on every single cache operation (`Get`, `Set`, `Delete`, etc.) just to check if the store is closed introduces unnecessary lock contention on high throughput hot paths. Using an `atomic.Bool` completely removes this contention and measurably improves read throughput.
**Action:** Replace `s.closed bool` and `s.closedLock sync.RWMutex` with `s.closed atomic.Bool` for simple state flags that are read frequently but written rarely (like a shutdown flag).
