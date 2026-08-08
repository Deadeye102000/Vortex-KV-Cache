package store

// EvictionPolicy defines the pluggable contract for cache key eviction strategies.
// Implementations (e.g. LRU, LFU, FIFO) maintain access/frequency metadata for keys in a shard.
type EvictionPolicy interface {
	// OnGet is invoked when a key is accessed via Get or Exists.
	OnGet(key string)

	// OnSet is invoked when a key is inserted or updated via Set.
	OnSet(key string)

	// OnDelete is invoked when a key is removed via Delete, Lazy Expiration, or Active Sweeping.
	OnDelete(key string)

	// SelectEvict recommends the candidate key to evict when capacity is exceeded.
	// Returns candidate key and true, or empty string and false if the policy holds no keys.
	SelectEvict() (string, bool)

	// Clear resets the eviction policy state.
	Clear()
}

// EvictionPolicyFactory is a factory function creating an EvictionPolicy instance per shard.
type EvictionPolicyFactory func() EvictionPolicy

// EvictionType specifies pre-packaged eviction strategies.
type EvictionType string

const (
	EvictionNone EvictionType = "none"
	EvictionLRU  EvictionType = "lru"
)

// NewEvictionPolicy returns a factory function for the specified eviction type.
func NewEvictionPolicy(eType EvictionType) EvictionPolicyFactory {
	switch eType {
	case EvictionLRU:
		return NewLRUPolicy
	case EvictionNone:
		return nil
	default:
		return nil
	}
}
