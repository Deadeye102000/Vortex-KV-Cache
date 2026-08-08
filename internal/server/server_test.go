package server

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"testing"

	"vortex-cache/internal/resp"
	"vortex-cache/internal/store"
)

func startTestServer(t *testing.T) (*Server, *store.Store, string) {
	cache, err := store.NewStore(store.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	srv := NewServer("127.0.0.1:0", cache)
	if err := srv.Listen(); err != nil {
		t.Fatalf("failed to listen on test socket: %v", err)
	}

	go func() {
		_ = srv.Serve()
	}()

	return srv, cache, srv.Addr().String()
}

func TestServer_CommandsIntegration(t *testing.T) {
	srv, cache, addr := startTestServer(t)
	defer func() {
		_ = srv.Close()
		_ = cache.Close()
	}()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("failed to dial test server: %v", err)
	}
	defer conn.Close()

	reader := resp.NewReader(conn)
	writer := resp.NewWriter(conn)

	// 1. PING
	_ = writer.WriteArray([]resp.Value{resp.NewBulkStringFromString("PING")})
	_ = writer.Flush()

	res, err := reader.ReadValue()
	if err != nil || res.Str != "PONG" {
		t.Fatalf("PING failed: res=%+v, err=%v", res, err)
	}

	// 2. SET key val EX 10
	_ = writer.WriteArray([]resp.Value{
		resp.NewBulkStringFromString("SET"),
		resp.NewBulkStringFromString("user:1"),
		resp.NewBulkStringFromString("alice"),
		resp.NewBulkStringFromString("EX"),
		resp.NewBulkStringFromString("10"),
	})
	_ = writer.Flush()

	res, err = reader.ReadValue()
	if err != nil || res.Str != "OK" {
		t.Fatalf("SET failed: res=%+v, err=%v", res, err)
	}

	// 3. GET key
	_ = writer.WriteArray([]resp.Value{
		resp.NewBulkStringFromString("GET"),
		resp.NewBulkStringFromString("user:1"),
	})
	_ = writer.Flush()

	res, err = reader.ReadValue()
	if err != nil || !bytes.Equal(res.Bulk, []byte("alice")) {
		t.Fatalf("GET failed: res=%+v, err=%v", res, err)
	}

	// 4. TTL key
	_ = writer.WriteArray([]resp.Value{
		resp.NewBulkStringFromString("TTL"),
		resp.NewBulkStringFromString("user:1"),
	})
	_ = writer.Flush()

	res, err = reader.ReadValue()
	if err != nil || res.Num <= 0 || res.Num > 10 {
		t.Fatalf("TTL failed: res=%+v, err=%v", res, err)
	}

	// 5. EXISTS key
	_ = writer.WriteArray([]resp.Value{
		resp.NewBulkStringFromString("EXISTS"),
		resp.NewBulkStringFromString("user:1"),
	})
	_ = writer.Flush()

	res, err = reader.ReadValue()
	if err != nil || res.Num != 1 {
		t.Fatalf("EXISTS failed: res=%+v, err=%v", res, err)
	}

	// 6. DEL key
	_ = writer.WriteArray([]resp.Value{
		resp.NewBulkStringFromString("DEL"),
		resp.NewBulkStringFromString("user:1"),
	})
	_ = writer.Flush()

	res, err = reader.ReadValue()
	if err != nil || res.Num != 1 {
		t.Fatalf("DEL failed: res=%+v, err=%v", res, err)
	}
}

func TestServer_Pipelining(t *testing.T) {
	srv, cache, addr := startTestServer(t)
	defer func() {
		_ = srv.Close()
		_ = cache.Close()
	}()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("failed to dial test server: %v", err)
	}
	defer conn.Close()

	reader := resp.NewReader(conn)
	writer := resp.NewWriter(conn)

	_ = writer.WriteArray([]resp.Value{resp.NewBulkStringFromString("SET"), resp.NewBulkStringFromString("k1"), resp.NewBulkStringFromString("v1")})
	_ = writer.WriteArray([]resp.Value{resp.NewBulkStringFromString("SET"), resp.NewBulkStringFromString("k2"), resp.NewBulkStringFromString("v2")})
	_ = writer.WriteArray([]resp.Value{resp.NewBulkStringFromString("GET"), resp.NewBulkStringFromString("k1")})
	_ = writer.Flush()

	r1, err1 := reader.ReadValue()
	r2, err2 := reader.ReadValue()
	r3, err3 := reader.ReadValue()

	if err1 != nil || r1.Str != "OK" || err2 != nil || r2.Str != "OK" || err3 != nil || string(r3.Bulk) != "v1" {
		t.Fatalf("pipelining failed: r1=%+v, r2=%+v, r3=%+v", r1, r2, r3)
	}
}

func TestServer_ConcurrentClients(t *testing.T) {
	srv, cache, addr := startTestServer(t)
	defer func() {
		_ = srv.Close()
		_ = cache.Close()
	}()

	const numClients = 16
	const opsPerClient = 100

	var wg sync.WaitGroup
	wg.Add(numClients)

	for c := 0; c < numClients; c++ {
		go func(clientID int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Errorf("client %d failed to connect: %v", clientID, err)
				return
			}
			defer conn.Close()

			r := resp.NewReader(conn)
			w := resp.NewWriter(conn)

			for i := 0; i < opsPerClient; i++ {
				key := fmt.Sprintf("client:%d:key:%d", clientID, i)
				val := fmt.Sprintf("val:%d", i)

				_ = w.WriteArray([]resp.Value{resp.NewBulkStringFromString("SET"), resp.NewBulkStringFromString(key), resp.NewBulkStringFromString(val)})
				_ = w.Flush()
				resSet, err := r.ReadValue()
				if err != nil || resSet.Str != "OK" {
					t.Errorf("client %d set error: %v", clientID, err)
					return
				}

				_ = w.WriteArray([]resp.Value{resp.NewBulkStringFromString("GET"), resp.NewBulkStringFromString(key)})
				_ = w.Flush()
				resGet, err := r.ReadValue()
				if err != nil || string(resGet.Bulk) != val {
					t.Errorf("client %d get error: %v", clientID, err)
					return
				}
			}
		}(c)
	}

	wg.Wait()
}
