package store

import (
	"context"
	"errors"
	"fmt"
	"hash/maphash"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrInvalidShardCount = errors.New("shard count must be a positive power of 2")
	ErrStoreClosed       = errors.New("store is closed")
)

// SnapshotEntry represents a key-value item captured during full-state resync snapshotting.
type SnapshotEntry struct {
	Key   string
	Value []byte
	TTL   time.Duration
}

// WriteListener callback function invoked whenever a mutation operation occurs.
type WriteListener func(cmdName string, key string, val []byte, ttl time.Duration)

// Config configures the Store instance options.
type Config struct {
	ShardCount            int
	MaxKeysPerShard       int
	EvictionFactory       EvictionPolicyFactory
	ActiveSweepInterval   time.Duration
	ActiveSweepSampleSize int
	ActiveSweepMaxCPU     time.Duration
}

// DefaultConfig returns optimal baseline configuration settings with no capacity limits.
func DefaultConfig() Config {
	return Config{
		ShardCount:            16,
		MaxKeysPerShard:       0,
		EvictionFactory:       nil,
		ActiveSweepInterval:   100 * time.Millisecond,
		ActiveSweepSampleSize: 20,
		ActiveSweepMaxCPU:     1 * time.Millisecond,
	}
}

// WithLRUEviction returns a Store configuration configured for LRU eviction.
func WithLRUEviction(shardCount, maxKeysPerShard int) Config {
	cfg := DefaultConfig()
	cfg.ShardCount = shardCount
	cfg.MaxKeysPerShard = maxKeysPerShard
	cfg.EvictionFactory = NewLRUPolicy
	return cfg
}

// Store is the high-performance sharded concurrent cache store.
type Store struct {
	cfg          Config
	shards       []*shard
	shardMask    uint64
	seed         maphash.Seed
	ctx          context.Context
	cancel       context.CancelFunc
	sweeperWg    sync.WaitGroup
	// Optimization: Using atomic.Bool instead of sync.RWMutex prevents lock contention on high-throughput hot paths.
	closed       atomic.Bool

	listenersMu  sync.RWMutex
	onWriteHooks []WriteListener
}

// NewStore initializes a new Store with the given configuration options.
func NewStore(cfg Config) (*Store, error) {
	if cfg.ShardCount <= 0 || (cfg.ShardCount&(cfg.ShardCount-1)) != 0 {
		return nil, ErrInvalidShardCount
	}

	if cfg.ActiveSweepInterval <= 0 {
		cfg.ActiveSweepInterval = 100 * time.Millisecond
	}
	if cfg.ActiveSweepSampleSize <= 0 {
		cfg.ActiveSweepSampleSize = 20
	}
	if cfg.ActiveSweepMaxCPU <= 0 {
		cfg.ActiveSweepMaxCPU = 1 * time.Millisecond
	}

	shards := make([]*shard, cfg.ShardCount)
	for i := 0; i < cfg.ShardCount; i++ {
		shards[i] = newShard(cfg.MaxKeysPerShard, cfg.EvictionFactory)
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Store{
		cfg:       cfg,
		shards:    shards,
		shardMask: uint64(cfg.ShardCount - 1),
		seed:      maphash.MakeSeed(),
		ctx:       ctx,
		cancel:    cancel,
	}

	s.sweeperWg.Add(1)
	go s.activeExpireLoop()

	return s, nil
}

// OnWrite registers a write listener callback function for replication broadcasting.
func (s *Store) OnWrite(listener WriteListener) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	s.onWriteHooks = append(s.onWriteHooks, listener)
}

func (s *Store) notifyWrite(cmdName string, key string, val []byte, ttl time.Duration) {
	s.listenersMu.RLock()
	hooks := s.onWriteHooks
	s.listenersMu.RUnlock()

	for _, hook := range hooks {
		hook(cmdName, key, val, ttl)
	}
}

// getShard retrieves the appropriate shard for a key.
func (s *Store) getShard(key string) *shard {
	// Optimization: replaced custom FNV-1a hashing with Go's built-in maphash.
	// Maphash is significantly faster (using hardware AES-NI instructions where available)
	// and provides randomized seeds preventing hash collision contention on shards.
	hash := maphash.String(s.seed, key)
	return s.shards[hash&s.shardMask]
}

// Set stores a key-value pair with an optional TTL duration.
func (s *Store) Set(key string, val []byte, ttl time.Duration) error {
	if s.closed.Load() {
		return ErrStoreClosed
	}

	now := time.Now().UnixNano()
	s.getShard(key).set(key, val, ttl, now)
	s.notifyWrite("SET", key, val, ttl)
	return nil
}

// Get retrieves a key's value. Performs lazy expiration if expired.
func (s *Store) Get(key string) ([]byte, bool) {
	if s.closed.Load() {
		return nil, false
	}

	now := time.Now().UnixNano()
	return s.getShard(key).get(key, now)
}

// Delete purges a key from the store.
func (s *Store) Delete(key string) bool {
	if s.closed.Load() {
		return false
	}

	deleted := s.getShard(key).delete(key)
	if deleted {
		s.notifyWrite("DEL", key, nil, 0)
	}
	return deleted
}

// Exists checks if a non-expired key is present.
func (s *Store) Exists(key string) bool {
	if s.closed.Load() {
		return false
	}

	now := time.Now().UnixNano()
	return s.getShard(key).exists(key, now)
}

// TTL returns remaining duration and existence flag.
func (s *Store) TTL(key string) (time.Duration, bool) {
	if s.closed.Load() {
		return -2, false
	}

	now := time.Now().UnixNano()
	return s.getShard(key).ttl(key, now)
}

// Expire sets or modifies the TTL on an existing key.
func (s *Store) Expire(key string, ttl time.Duration) bool {
	if s.closed.Load() {
		return false
	}

	now := time.Now().UnixNano()
	updated := s.getShard(key).expire(key, ttl, now)
	if updated {
		s.notifyWrite("EXPIRE", key, nil, ttl)
	}
	return updated
}

// Len returns current valid key count across all shards.
func (s *Store) Len() int64 {
	if s.closed.Load() {
		return 0
	}

	now := time.Now().UnixNano()
	var total int64
	for _, shard := range s.shards {
		total += shard.len(now)
	}
	return total
}

// Snapshot returns a thread-safe point-in-time slice of all non-expired entries across shards.
func (s *Store) Snapshot() []SnapshotEntry {
	if s.closed.Load() {
		return nil
	}

	now := time.Now().UnixNano()
	var entries []SnapshotEntry

	for _, shd := range s.shards {
		shd.mu.RLock()
		for key, item := range shd.items {
			if !item.IsExpired(now) {
				remTTL := item.RemainingTTL(now)
				valCopy := make([]byte, len(item.Value))
				copy(valCopy, item.Value)

				entries = append(entries, SnapshotEntry{
					Key:   key,
					Value: valCopy,
					TTL:   remTTL,
				})
			}
		}
		shd.mu.RUnlock()
	}

	return entries
}

// Close gracefully stops the background active sweeper.
func (s *Store) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	s.cancel()
	s.sweeperWg.Wait()
	return nil
}

func (s *Store) activeExpireLoop() {
	defer s.sweeperWg.Done()

	ticker := time.NewTicker(s.cfg.ActiveSweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runActiveSweepCycle()
		}
	}
}

func (s *Store) runActiveSweepCycle() {
	deadline := time.Now().Add(s.cfg.ActiveSweepMaxCPU)

	for _, shd := range s.shards {
		if time.Now().After(deadline) {
			break
		}

		for {
			now := time.Now().UnixNano()
			expired, sampled := shd.activeSweep(s.cfg.ActiveSweepSampleSize, now)

			if sampled == 0 || (expired*100/sampled) < 25 {
				break
			}

			if time.Now().After(deadline) {
				break
			}
		}
	}
}

func (s *Store) Stats() string {
	return fmt.Sprintf("Shards: %d, MaxKeysPerShard: %d, Keys: %d", s.cfg.ShardCount, s.cfg.MaxKeysPerShard, s.Len())
}
