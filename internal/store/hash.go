package store

const (
	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

// fnv1a64 computes a 64-bit FNV-1a hash of a string without memory allocations.
func fnv1a64(key string) uint64 {
	var hash uint64 = fnvOffset64
	for i := 0; i < len(key); i++ {
		hash ^= uint64(key[i])
		hash *= fnvPrime64
	}
	return hash
}
