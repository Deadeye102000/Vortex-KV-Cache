package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrInvalidShardCount = errors.New("shard count must be a positive power of 2")
	ErrStoreClosed       = errors.New("store is closed")
)

// Config configures the Store instance options.
type Config struct {
	// ShardCount must be a power of 2 (e.g. 1, 2, 4, 8, 16, 32, 64, 128). Default: 16.
	ShardCount int

	// MaxKeysPerShard defines maximum keys allowed per shard before eviction is triggered.
	// 0 indicates unlimited capacity.
	MaxKeysPerShard int

	// EvictionFactory creates an EvictionPolicy instance per shard.
	// If nil, no eviction is performed when capacity limit is reached.
	EvictionFactory EvictionPolicyFactory

	// ActiveSweepInterval is how often background active expiration runs. Default: 100ms.
	ActiveSweepInterval time.Duration

	// ActiveSweepSampleSize is max keys sampled per shard per cycle. Default: 20.
	ActiveSweepSampleSize int

	// ActiveSweepMaxCPU is maximum CPU execution duration per active sweep cycle. Default: 1ms.
	ActiveSweepMaxCPU time.Duration
}

// DefaultConfig returns optimal baseline configuration settings with no capacity limits.
func DefaultConfig() Config {
	return Config{
		ShardCount:            16,
		MaxKeysPerShard:       0, // Unlimited
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
	cfg        Config
	shards     []*shard
	shardMask  uint64
	ctx        context.Context
	cancel     context.CancelFunc
	sweeperWg  sync.WaitGroup
	closed     bool
	closedLock sync.RWMutex
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
		ctx:       ctx,
		cancel:    cancel,
	}

	// Launch Redis-style probabilistic active background sweeper
	s.sweeperWg.Add(1)
	go s.activeExpireLoop()

	return s, nil
}

// getShard retrieves the appropriate shard for a key via zero-allocation FNV-1a hash & bitwise mask.
func (s *Store) getShard(key string) *shard {
	hash := fnv1a64(key)
	return s.shards[hash&s.shardMask]
}

// Set stores a key-value pair with an optional TTL duration.
// If shard capacity is reached, triggers eviction policy.
func (s *Store) Set(key string, val []byte, ttl time.Duration) error {
	s.closedLock.RLock()
	defer s.closedLock.RUnlock()
	if s.closed {
		return ErrStoreClosed
	}

	now := time.Now().UnixNano()
	s.getShard(key).set(key, val, ttl, now)
	return nil
}

// Get retrieves a key's value. Performs lazy expiration if expired.
func (s *Store) Get(key string) ([]byte, bool) {
	s.closedLock.RLock()
	defer s.closedLock.RUnlock()
	if s.closed {
		return nil, false
	}

	now := time.Now().UnixNano()
	return s.getShard(key).get(key, now)
}

// Delete purges a key from the store.
func (s *Store) Delete(key string) bool {
	s.closedLock.RLock()
	defer s.closedLock.RUnlock()
	if s.closed {
		return false
	}

	return s.getShard(key).delete(key)
}

// Exists checks if a non-expired key is present.
func (s *Store) Exists(key string) bool {
	s.closedLock.RLock()
	defer s.closedLock.RUnlock()
	if s.closed {
		return false
	}

	now := time.Now().UnixNano()
	return s.getShard(key).exists(key, now)
}

// TTL returns remaining duration and existence flag.
// Returns -1 for non-expiring keys, -2 if key does not exist.
func (s *Store) TTL(key string) (time.Duration, bool) {
	s.closedLock.RLock()
	defer s.closedLock.RUnlock()
	if s.closed {
		return -2, false
	}

	now := time.Now().UnixNano()
	return s.getShard(key).ttl(key, now)
}

// Expire sets or modifies the TTL on an existing key.
func (s *Store) Expire(key string, ttl time.Duration) bool {
	s.closedLock.RLock()
	defer s.closedLock.RUnlock()
	if s.closed {
		return false
	}

	now := time.Now().UnixNano()
	return s.getShard(key).expire(key, ttl, now)
}

// Len returns the current valid key count across all shards.
func (s *Store) Len() int64 {
	s.closedLock.RLock()
	defer s.closedLock.RUnlock()
	if s.closed {
		return 0
	}

	now := time.Now().UnixNano()
	var total int64
	for _, shard := range s.shards {
		total += shard.len(now)
	}
	return total
}

// Close gracefully stops the background active sweeper and shuts down the store.
func (s *Store) Close() error {
	s.closedLock.Lock()
	if s.closed {
		s.closedLock.Unlock()
		return nil
	}
	s.closed = true
	s.closedLock.Unlock()

	s.cancel()
	s.sweeperWg.Wait()
	return nil
}

// activeExpireLoop runs the Redis-style background probabilistic active expiration algorithm.
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

// runActiveSweepCycle executes a single active sweep cycle across shards within CPU deadline.
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

// Stats returns a summary of the store configuration and shard count.
func (s *Store) Stats() string {
	return fmt.Sprintf("Shards: %d, MaxKeysPerShard: %d, Keys: %d", s.cfg.ShardCount, s.cfg.MaxKeysPerShard, s.Len())
}
