package store

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkStore_Reads(b *testing.B) {
	for _, shardCount := range []int{1, 16, 64} {
		b.Run(fmt.Sprintf("Shards-%d", shardCount), func(b *testing.B) {
			s, _ := NewStore(Config{ShardCount: shardCount})
			defer s.Close()

			key := "bench:key"
			val := []byte("bench:value:data")
			_ = s.Set(key, val, 0)

			b.ResetTimer()
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, _ = s.Get(key)
				}
			})
		})
	}
}

func BenchmarkStore_Writes(b *testing.B) {
	for _, shardCount := range []int{1, 16, 64} {
		b.Run(fmt.Sprintf("Shards-%d", shardCount), func(b *testing.B) {
			s, _ := NewStore(Config{ShardCount: shardCount})
			defer s.Close()

			val := []byte("bench:value:data")

			b.ResetTimer()
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					i++
					key := fmt.Sprintf("key:%d", i%1000)
					_ = s.Set(key, val, 0)
				}
			})
		})
	}
}

func BenchmarkStore_Mixed90Read10Write(b *testing.B) {
	for _, shardCount := range []int{1, 16, 64} {
		b.Run(fmt.Sprintf("Shards-%d", shardCount), func(b *testing.B) {
			s, _ := NewStore(Config{ShardCount: shardCount})
			defer s.Close()

			val := []byte("bench:value:data")
			for i := 0; i < 1000; i++ {
				_ = s.Set(fmt.Sprintf("key:%d", i), val, 0)
			}

			b.ResetTimer()
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					i++
					key := fmt.Sprintf("key:%d", i%1000)
					if i%10 == 0 {
						_ = s.Set(key, val, 0)
					} else {
						_, _ = s.Get(key)
					}
				}
			})
		})
	}
}

func BenchmarkStore_WithTTL(b *testing.B) {
	s, _ := NewStore(Config{ShardCount: 16})
	defer s.Close()

	val := []byte("bench:value:data")
	ttl := 10 * time.Minute

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			key := fmt.Sprintf("ttlkey:%d", i%500)
			_ = s.Set(key, val, ttl)
			_, _ = s.Get(key)
		}
	})
}
