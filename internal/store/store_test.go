package store

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStore_BasicOperations(t *testing.T) {
	s, err := NewStore(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	key := "user:1001"
	val := []byte("alice")

	// Set & Get
	if err := s.Set(key, val, 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, found := s.Get(key)
	if !found {
		t.Fatalf("expected key %s to be found", key)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("got %s, want %s", string(got), string(val))
	}

	// Exists
	if !s.Exists(key) {
		t.Fatalf("expected key %s to exist", key)
	}

	// Delete
	if !s.Delete(key) {
		t.Fatalf("expected key %s to be deleted", key)
	}

	if s.Exists(key) {
		t.Fatalf("key %s should not exist after deletion", key)
	}
}

func TestStore_LazyExpiration(t *testing.T) {
	s, err := NewStore(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	key := "temp:session"
	val := []byte("secret_token")
	ttl := 50 * time.Millisecond

	if err := s.Set(key, val, ttl); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Immediate check: should exist
	if _, found := s.Get(key); !found {
		t.Fatalf("key should be found before expiration")
	}

	// Wait for TTL to pass
	time.Sleep(70 * time.Millisecond)

	// Lazy expiration check: Get should return not found and purge the key
	if _, found := s.Get(key); found {
		t.Fatalf("key should have expired and returned not found")
	}

	if s.Exists(key) {
		t.Fatalf("key should not exist after lazy expiration")
	}
}

func TestStore_ActiveExpirationSweep(t *testing.T) {
	cfg := Config{
		ShardCount:            4,
		ActiveSweepInterval:   20 * time.Millisecond,
		ActiveSweepSampleSize: 50,
		ActiveSweepMaxCPU:     5 * time.Millisecond,
	}

	s, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	// Insert 100 short-lived keys with 30ms TTL
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("sweep:key:%d", i)
		val := []byte(fmt.Sprintf("val:%d", i))
		_ = s.Set(key, val, 30*time.Millisecond)
	}

	if initialLen := s.Len(); initialLen != 100 {
		t.Fatalf("expected 100 initial keys, got %d", initialLen)
	}

	// Wait for TTL + multiple active sweep cycles to run
	time.Sleep(120 * time.Millisecond)

	// Active sweeper should have purged the expired keys without explicit Get calls
	if finalLen := s.Len(); finalLen != 0 {
		t.Fatalf("expected active sweeper to purge expired keys, remaining: %d", finalLen)
	}
}

func TestStore_TTLAndExpire(t *testing.T) {
	s, err := NewStore(DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	key := "cache:item"
	val := []byte("data")

	_ = s.Set(key, val, 200*time.Millisecond)

	remaining, ok := s.TTL(key)
	if !ok || remaining <= 0 || remaining > 200*time.Millisecond {
		t.Fatalf("invalid initial TTL: %v", remaining)
	}

	if !s.Expire(key, 500*time.Millisecond) {
		t.Fatalf("failed to update Expire on key")
	}

	updatedRemaining, ok := s.TTL(key)
	if !ok || updatedRemaining <= 200*time.Millisecond {
		t.Fatalf("expected updated TTL > 200ms, got %v", updatedRemaining)
	}
}

func TestStore_LRUEvictionCorrectness(t *testing.T) {
	// Single shard with fixed max capacity of 3 keys
	cfg := WithLRUEviction(1, 3)
	s, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	_ = s.Set("K1", []byte("val1"), 0)
	_ = s.Set("K2", []byte("val2"), 0)
	_ = s.Set("K3", []byte("val3"), 0)

	if s.Len() != 3 {
		t.Fatalf("expected 3 items, got %d", s.Len())
	}

	// Access K1 to promote it to MRU (LRU order becomes: K2 -> K3 -> K1)
	_, found := s.Get("K1")
	if !found {
		t.Fatalf("expected K1 to be found")
	}

	// Insert K4. Capacity is 3, so K2 (the LRU key) must be evicted!
	_ = s.Set("K4", []byte("val4"), 0)

	if s.Len() != 3 {
		t.Fatalf("expected store length to remain 3 after eviction, got %d", s.Len())
	}

	// K2 should be evicted (not found)
	if _, found := s.Get("K2"); found {
		t.Fatalf("expected K2 to be evicted via LRU policy")
	}

	// K1, K3, K4 should all still exist
	for _, k := range []string{"K1", "K3", "K4"} {
		if _, found := s.Get(k); !found {
			t.Fatalf("expected key %s to exist in store", k)
		}
	}
}

// mockEvictionPolicy tracks eviction calls to verify custom pluggable policy contract.
type mockEvictionPolicy struct {
	evictCount atomic.Int64
	keys       []string
}

func (m *mockEvictionPolicy) OnGet(key string)    {}
func (m *mockEvictionPolicy) OnSet(key string)    { m.keys = append(m.keys, key) }
func (m *mockEvictionPolicy) OnDelete(key string) {}
func (m *mockEvictionPolicy) SelectEvict() (string, bool) {
	m.evictCount.Add(1)
	if len(m.keys) == 0 {
		return "", false
	}
	evict := m.keys[0]
	m.keys = m.keys[1:]
	return evict, true
}
func (m *mockEvictionPolicy) Clear() { m.keys = nil }

func TestStore_PluggableCustomPolicy(t *testing.T) {
	mock := &mockEvictionPolicy{}
	cfg := DefaultConfig()
	cfg.ShardCount = 1
	cfg.MaxKeysPerShard = 2
	cfg.EvictionFactory = func() EvictionPolicy {
		return mock
	}

	s, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	_ = s.Set("A", []byte("1"), 0)
	_ = s.Set("B", []byte("2"), 0)
	_ = s.Set("C", []byte("3"), 0) // Triggers eviction call to mock policy

	if mock.evictCount.Load() == 0 {
		t.Fatalf("expected custom pluggable policy to be called on eviction")
	}
}

func TestStore_ConcurrentEvictionRace(t *testing.T) {
	// Fixed capacity per shard with LRU eviction under heavy parallel reads and writes
	cfg := WithLRUEviction(8, 20)
	s, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	const numGoroutines = 32
	const opsPerGoroutine = 300

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := fmt.Sprintf("race:key:%d:%d", goroutineID, i%50)
				val := []byte(fmt.Sprintf("val:%d", i))

				_ = s.Set(key, val, 50*time.Millisecond)
				_, _ = s.Get(key)
				_ = s.Exists(key)
				if i%4 == 0 {
					_ = s.Delete(key)
				}
			}
		}(g)
	}

	wg.Wait()
}

func TestStore_ConcurrentStress(t *testing.T) {
	s, err := NewStore(Config{
		ShardCount:            16,
		ActiveSweepInterval:   10 * time.Millisecond,
		ActiveSweepSampleSize: 20,
		ActiveSweepMaxCPU:     1 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	const numGoroutines = 32
	const opsPerGoroutine = 500

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := fmt.Sprintf("key:%d:%d", goroutineID, i%50)
				val := []byte(fmt.Sprintf("val:%d", i))

				_ = s.Set(key, val, 100*time.Millisecond)
				_, _ = s.Get(key)
				_ = s.Exists(key)
				if i%5 == 0 {
					_ = s.Delete(key)
				}
				if i%7 == 0 {
					_ = s.Expire(key, 200*time.Millisecond)
				}
			}
		}(g)
	}

	wg.Wait()
}

func TestStore_ShardUniformity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShardCount = 16
	s, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	const numKeys = 16000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key:uniformity:%d", i)
		_ = s.Set(key, []byte("x"), 0)
	}

	for i, shd := range s.shards {
		count := shd.len(time.Now().UnixNano())
		if count == 0 {
			t.Fatalf("shard %d has 0 keys; poor hash distribution", i)
		}
	}
}
