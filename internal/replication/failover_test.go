package replication_test

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"vortex-cache/internal/resp"
)

func TestChaos_MasterTerminationFailoverRecovery(t *testing.T) {
	masterSrv, masterCache, masterAddr, masterPort := startNode(t)
	replicaSrv, replicaCache, replicaAddr, _ := startNode(t)

	defer func() {
		_ = replicaSrv.Close()
		_ = replicaCache.Close()
	}()

	// 1. Establish SLAVEOF replication link
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
	resSlaveOf, err := rRepl.ReadValue()
	if err != nil || resSlaveOf.Str != "OK" {
		t.Fatalf("SLAVEOF failed: res=%+v, err=%v", resSlaveOf, err)
	}

	time.Sleep(100 * time.Millisecond)

	// 2. Launch concurrent write workload on Master
	var stopWorkload sync.WaitGroup
	stopWorkload.Add(1)
	go func() {
		defer stopWorkload.Done()
		connM, err := net.Dial("tcp", masterAddr)
		if err != nil {
			return
		}
		defer connM.Close()

		wM := resp.NewWriter(connM)
		rM := resp.NewReader(connM)

		for i := 0; i < 500; i++ {
			_ = wM.WriteArray([]resp.Value{
				resp.NewBulkStringFromString("SET"),
				resp.NewBulkStringFromString(fmt.Sprintf("chaos:key:%d", i)),
				resp.NewBulkStringFromString("data"),
			})
			_ = wM.Flush()
			_, err := rM.ReadValue()
			if err != nil {
				return // Master killed
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// 3. CHAOS INJECTION: Terminate Master process abruptly mid-load-test
	t.Log("=== CHAOS INJECTION: TERMINATING MASTER SERVER ===")
	killTime := time.Now()
	_ = masterSrv.Close()
	_ = masterCache.Close()

	// 4. Poll Replica until it detects Master kill, executes self-promotion, and accepts new writes
	var recoveryTime time.Duration
	var promotedSuccess bool

	for {
		_ = wRepl.WriteArray([]resp.Value{
			resp.NewBulkStringFromString("SET"),
			resp.NewBulkStringFromString("post:failover:key"),
			resp.NewBulkStringFromString("promoted_value"),
		})
		_ = wRepl.Flush()

		res, err := rRepl.ReadValue()
		if err == nil && res.Str == "OK" {
			recoveryTime = time.Since(killTime)
			promotedSuccess = true
			break
		}

		if time.Since(killTime) > 5*time.Second {
			t.Fatalf("failover timeout: replica failed to self-promote after master termination")
		}
		time.Sleep(10 * time.Millisecond)
	}

	stopWorkload.Wait()

	if !promotedSuccess {
		t.Fatalf("replica failed to accept writes after master kill")
	}

	recoveryMs := float64(recoveryTime.Microseconds()) / 1000.0
	t.Logf("=== CHAOS FAILOVER RECOVERY METRICS ===")
	t.Logf("Master Kill Timestamp:           %s", killTime.Format("15:04:05.000"))
	t.Logf("Promoted Master Read/Write Ready: YES")
	t.Logf("Measured Recovery Time:          %v (%.2f ms)", recoveryTime, recoveryMs)

	// Verify post-failover write on newly promoted master
	_ = wRepl.WriteArray([]resp.Value{
		resp.NewBulkStringFromString("GET"),
		resp.NewBulkStringFromString("post:failover:key"),
	})
	_ = wRepl.Flush()
	valGet, err := rRepl.ReadValue()
	if err != nil || string(valGet.Bulk) != "promoted_value" {
		t.Fatalf("failed to retrieve write on promoted master: %+v", valGet)
	}
}
