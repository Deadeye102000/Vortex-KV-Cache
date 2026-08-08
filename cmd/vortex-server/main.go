package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"vortex-cache/internal/server"
	"vortex-cache/internal/store"
)

func main() {
	port := flag.Int("p", 6379, "Port for Vortex Cache TCP server to listen on")
	shards := flag.Int("shards", 16, "Number of cache shards (must be power of 2)")
	maxKeysPerShard := flag.Int("max-keys-per-shard", 0, "Max keys per shard before LRU eviction (0 = unlimited)")
	flag.Parse()

	cfg := store.DefaultConfig()
	cfg.ShardCount = *shards
	cfg.MaxKeysPerShard = *maxKeysPerShard
	if *maxKeysPerShard > 0 {
		cfg.EvictionFactory = store.NewLRUPolicy
	}

	cache, err := store.NewStore(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize Vortex store: %v\n", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%d", *port)
	srv := server.NewServer(addr, cache)

	go func() {
		fmt.Printf("Vortex Cache TCP Server listening on %s (%s)\n", addr, cache.Stats())
		if err := srv.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down Vortex Cache...")
	_ = srv.Close()
	_ = cache.Close()
	fmt.Println("Vortex Cache stopped gracefully.")
}
