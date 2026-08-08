package replication_test

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"vortex-cache/internal/resp"
	"vortex-cache/internal/server"
	"vortex-cache/internal/store"
)

func startNode(t *testing.T) (*server.Server, *store.Store, string, int) {
	cache, err := store.NewStore(store.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	srv := server.NewServer("127.0.0.1:0", cache)
	if err := srv.Listen(); err != nil {
		t.Fatalf("failed to listen on socket: %v", err)
	}

	go func() {
		_ = srv.Serve()
	}()

	addrStr := srv.Addr().String()
	_, portStr, err := net.SplitHostPort(addrStr)
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	return srv, cache, addrStr, port
}

func TestReplication_FullResyncAndReadonly(t *testing.T) {
	masterSrv, masterCache, _, masterPort := startNode(t)
	defer func() {
		_ = masterSrv.Close()
		_ = masterCache.Close()
	}()

	replicaSrv, replicaCache, replicaAddr, _ := startNode(t)
	defer func() {
		_ = replicaSrv.Close()
		_ = replicaCache.Close()
	}()

	// 1. Seed Master with 5 initial keys
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("initial:key:%d", i)
		val := []byte(fmt.Sprintf("initial:val:%d", i))
		_ = masterCache.Set(key, val, 0)
	}

	// 2. Dial Replica and issue SLAVEOF 127.0.0.1 <masterPort>
	connRepl, err := net.Dial("tcp", replicaAddr)
	if err != nil {
		t.Fatalf("failed to connect to replica: %v", err)
	}
	defer connRepl.Close()

	rRepl := resp.NewReader(connRepl)
	wRepl := resp.NewWriter(connRepl)

	_ = wRepl.WriteArray([]resp.Value{
		resp.NewBulkStringFromString("SLAVEOF"),
		resp.NewBulkStringFromString("127.0.0.1"),
		resp.NewBulkStringFromString(fmt.Sprintf("%d", masterPort)),
	})
	_ = wRepl.Flush()

	resSlaveOf, err := rRepl.ReadValue()
	if err != nil || resSlaveOf.Str != "OK" {
		t.Fatalf("SLAVEOF failed: res=%+v, err=%v", resSlaveOf, err)
	}

	// 3. Wait briefly for initial snapshot resync to complete on Replica
	time.Sleep(100 * time.Millisecond)

	// Verify all 5 initial keys exist on Replica
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("initial:key:%d", i)
		expectedVal := fmt.Sprintf("initial:val:%d", i)
		val, found := replicaCache.Get(key)
		if !found {
			t.Fatalf("expected replica to hold snapshot key %s", key)
		}
		if string(val) != expectedVal {
			t.Fatalf("replica snapshot value mismatch: got %s, want %s", string(val), expectedVal)
		}
	}

	// 4. Verify READONLY enforcement on Replica
	_ = wRepl.WriteArray([]resp.Value{
		resp.NewBulkStringFromString("SET"),
		resp.NewBulkStringFromString("illegal:write"),
		resp.NewBulkStringFromString("data"),
	})
	_ = wRepl.Flush()

	resWrite, err := rRepl.ReadValue()
	if err != nil || resWrite.Type != resp.TypeError || !strings.Contains(resWrite.Str, "READONLY") {
		t.Fatalf("expected READONLY error on replica direct write, got: %+v", resWrite)
	}
}

func TestReplication_LiveStreamingAndMeasuredLag(t *testing.T) {
	masterSrv, masterCache, masterAddr, masterPort := startNode(t)
	defer func() {
		_ = masterSrv.Close()
		_ = masterCache.Close()
	}()

	replicaSrv, replicaCache, replicaAddr, _ := startNode(t)
	defer func() {
		_ = replicaSrv.Close()
		_ = replicaCache.Close()
	}()

	connRepl, err := net.Dial("tcp", replicaAddr)
	if err != nil {
		t.Fatalf("failed to dial replica: %v", err)
	}
	defer connRepl.Close()

	rRepl := resp.NewReader(connRepl)
	wRepl := resp.NewWriter(connRepl)

	_ = wRepl.WriteArray([]resp.Value{
		resp.NewBulkStringFromString("SLAVEOF"),
		resp.NewBulkStringFromString("127.0.0.1"),
		resp.NewBulkStringFromString(fmt.Sprintf("%d", masterPort)),
	})
	_ = wRepl.Flush()
	_, _ = rRepl.ReadValue()

	time.Sleep(50 * time.Millisecond)

	connMaster, err := net.Dial("tcp", masterAddr)
	if err != nil {
		t.Fatalf("failed to dial master: %v", err)
	}
	defer connMaster.Close()

	rMaster := resp.NewReader(connMaster)
	wMaster := resp.NewWriter(connMaster)

	const totalWrites = 500
	startTime := time.Now()

	for i := 0; i < totalWrites; i++ {
		key := fmt.Sprintf("stream:key:%d", i)
		val := fmt.Sprintf("stream:val:%d", i)
		_ = wMaster.WriteArray([]resp.Value{
			resp.NewBulkStringFromString("SET"),
			resp.NewBulkStringFromString(key),
			resp.NewBulkStringFromString(val),
		})
		_ = wMaster.Flush()
		_, _ = rMaster.ReadValue()
	}

	writeDoneTime := time.Now()

	var replicationLag time.Duration
	for {
		if replicaCache.Len() >= totalWrites {
			replicationLag = time.Since(writeDoneTime)
			break
		}
		if time.Since(writeDoneTime) > 3*time.Second {
			t.Fatalf("replication timeout: replica holds %d / %d keys", replicaCache.Len(), totalWrites)
		}
		time.Sleep(2 * time.Millisecond)
	}

	t.Logf("=== REPLICATION BENCHMARK METRICS ===")
	t.Logf("Total Keys Written to Master: %d", totalWrites)
	t.Logf("Master Write Execution Time:  %v", writeDoneTime.Sub(startTime))
	t.Logf("Measured Replication Lag:     %v (%.3f ms)", replicationLag, float64(replicationLag.Microseconds())/1000.0)

	for i := 0; i < totalWrites; i++ {
		key := fmt.Sprintf("stream:key:%d", i)
		expectedVal := fmt.Sprintf("stream:val:%d", i)
		val, found := replicaCache.Get(key)
		if !found || !bytes.Equal(val, []byte(expectedVal)) {
			t.Fatalf("replica key %s mismatch or missing: got %s, want %s", key, string(val), expectedVal)
		}
	}
}
