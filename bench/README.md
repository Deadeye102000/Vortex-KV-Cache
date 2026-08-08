# Vortex Cache Benchmarks

This directory contains standalone benchmark utilities and performance comparison harnesses.

## Running Store Engine Benchmarks
To execute the in-memory store benchmarks across 1, 16, and 64 shard configurations:

```bash
go test -bench=. -benchmem ./internal/store/...
```

### Benchmark Metrics Tracked:
- **`ns/op`**: Nanoseconds per cache operation (throughput & latency).
- **`B/op`**: Bytes allocated per operation (memory overhead).
- **`allocs/op`**: Number of memory allocations per operation.
