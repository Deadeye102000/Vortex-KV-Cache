package store

import (
	"time"
)

// Entry represents an individual cache item stored in memory.
// Timestamps are stored as Unix nanoseconds (int64) to reduce memory allocations
// and eliminate Go Garbage Collector pointer-chasing overhead.
type Entry struct {
	Value      []byte
	CreatedAt  int64 // Unix nanoseconds
	ExpiresAt  int64 // Unix nanoseconds (0 indicates no expiration)
}

// NewEntry constructs a new cache entry.
// If ttl > 0, ExpiresAt is computed relative to now (in Unix nanoseconds).
func NewEntry(val []byte, ttl time.Duration, now int64) *Entry {
	var expiresAt int64
	if ttl > 0 {
		expiresAt = now + ttl.Nanoseconds()
	}

	// Copy value slice to ensure immutability and prevent external mutation
	valCopy := make([]byte, len(val))
	copy(valCopy, val)

	return &Entry{
		Value:      valCopy,
		CreatedAt:  now,
		ExpiresAt:  expiresAt,
	}
}

// IsExpired checks if the entry has passed its expiration timestamp relative to now (Unix nanoseconds).
func (e *Entry) IsExpired(now int64) bool {
	if e.ExpiresAt == 0 {
		return false
	}
	return now >= e.ExpiresAt
}

// RemainingTTL calculates the remaining duration for this entry relative to now (Unix nanoseconds).
// Returns -1 if the entry has no expiration.
// Returns 0 if expired.
func (e *Entry) RemainingTTL(now int64) time.Duration {
	if e.ExpiresAt == 0 {
		return -1
	}
	if now >= e.ExpiresAt {
		return 0
	}
	return time.Duration(e.ExpiresAt - now)
}
