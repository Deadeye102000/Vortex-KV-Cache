package store

import (
	"sync"
	"sync/atomic"
	"time"
)

// shard holds a subset of the key-value store guarded by its own RWMutex lock.
type shard struct {
	mu       sync.RWMutex
	items    map[string]*Entry
	maxKeys  int
	eviction EvictionPolicy
}

func newShard(maxKeys int, factory EvictionPolicyFactory) *shard {
	var evict EvictionPolicy
	if factory != nil {
		evict = factory()
	}

	return &shard{
		items:    make(map[string]*Entry),
		maxKeys:  maxKeys,
		eviction: evict,
	}
}

// set stores or updates an entry in the shard.
// Triggers eviction if maxKeys capacity is exceeded.
func (s *shard) set(key string, val []byte, ttl time.Duration, now int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.items[key]
	if exists {
		s.items[key] = NewEntry(val, ttl, now)
		if s.eviction != nil {
			s.eviction.OnSet(key)
		}
		return
	}

	// Capacity check & eviction trigger for new keys
	if s.maxKeys > 0 && len(s.items) >= s.maxKeys && s.eviction != nil {
		if evictKey, ok := s.eviction.SelectEvict(); ok {
			delete(s.items, evictKey)
			s.eviction.OnDelete(evictKey)
		}
	}

	s.items[key] = NewEntry(val, ttl, now)
	if s.eviction != nil {
		s.eviction.OnSet(key)
	}
}

// get retrieves a value from the shard.
// Performs lazy expiration: if key is expired, purges it under write lock.
func (s *shard) get(key string, now int64) ([]byte, bool) {
	s.mu.RLock()
	entry, exists := s.items[key]
	if !exists {
		s.mu.RUnlock()
		return nil, false
	}

	if entry.IsExpired(now) {
		s.mu.RUnlock()
		s.mu.Lock()
		if e, ok := s.items[key]; ok && e.IsExpired(now) {
			delete(s.items, key)
			if s.eviction != nil {
				s.eviction.OnDelete(key)
			}
		}
		s.mu.Unlock()
		return nil, false
	}

	atomic.StoreInt64(&entry.AccessedAt, now)
	valCopy := entry.Value

	s.mu.RUnlock()

	// Notify eviction policy under write lock or upgrade lock safely
	if s.eviction != nil {
		s.mu.Lock()
		if _, ok := s.items[key]; ok {
			s.eviction.OnGet(key)
		}
		s.mu.Unlock()
	}

	return valCopy, true
}

// delete removes a key from the shard. Returns true if the key was present.
func (s *shard) delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.items[key]; exists {
		delete(s.items, key)
		if s.eviction != nil {
			s.eviction.OnDelete(key)
		}
		return true
	}
	return false
}

// exists checks if a non-expired key exists in the shard.
func (s *shard) exists(key string, now int64) bool {
	s.mu.RLock()
	entry, found := s.items[key]
	if !found {
		s.mu.RUnlock()
		return false
	}

	if entry.IsExpired(now) {
		s.mu.RUnlock()
		s.mu.Lock()
		if e, ok := s.items[key]; ok && e.IsExpired(now) {
			delete(s.items, key)
			if s.eviction != nil {
				s.eviction.OnDelete(key)
			}
		}
		s.mu.Unlock()
		return false
	}
	s.mu.RUnlock()

	if s.eviction != nil {
		s.mu.Lock()
		if _, ok := s.items[key]; ok {
			s.eviction.OnGet(key)
		}
		s.mu.Unlock()
	}

	return true
}

// ttl returns remaining duration of a key.
func (s *shard) ttl(key string, now int64) (time.Duration, bool) {
	s.mu.RLock()
	entry, found := s.items[key]
	if !found {
		s.mu.RUnlock()
		return -2, false
	}

	if entry.IsExpired(now) {
		s.mu.RUnlock()
		s.mu.Lock()
		if e, ok := s.items[key]; ok && e.IsExpired(now) {
			delete(s.items, key)
			if s.eviction != nil {
				s.eviction.OnDelete(key)
			}
		}
		s.mu.Unlock()
		return -2, false
	}

	remaining := entry.RemainingTTL(now)
	s.mu.RUnlock()
	return remaining, true
}

// expire updates or sets the TTL on an existing key.
func (s *shard) expire(key string, ttl time.Duration, now int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, found := s.items[key]
	if !found || entry.IsExpired(now) {
		if found {
			delete(s.items, key)
			if s.eviction != nil {
				s.eviction.OnDelete(key)
			}
		}
		return false
	}

	if ttl > 0 {
		entry.ExpiresAt = now + ttl.Nanoseconds()
	} else {
		entry.ExpiresAt = 0
	}

	if s.eviction != nil {
		s.eviction.OnSet(key)
	}

	return true
}

// len returns count of non-expired keys in shard.
func (s *shard) len(now int64) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, entry := range s.items {
		if !entry.IsExpired(now) {
			count++
		}
	}
	return count
}

// activeSweep samples up to sampleSize keys in this shard.
func (s *shard) activeSweep(sampleSize int, now int64) (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.items) == 0 {
		return 0, 0
	}

	sampled := 0
	expired := 0

	for key, entry := range s.items {
		if sampled >= sampleSize {
			break
		}
		sampled++
		if entry.IsExpired(now) {
			delete(s.items, key)
			if s.eviction != nil {
				s.eviction.OnDelete(key)
			}
			expired++
		}
	}

	return expired, sampled
}
