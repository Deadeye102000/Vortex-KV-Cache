package replication

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"vortex-cache/internal/resp"
	"vortex-cache/internal/store"
)

type Role string

const (
	RoleMaster  Role = "master"
	RoleReplica Role = "slave"
)

// ReplicaManager handles replica synchronization, master stream consumption, and failover monitoring.
type ReplicaManager struct {
	cache           *store.Store
	mu              sync.RWMutex
	role            Role
	masterAddr      string
	masterLinkUp    bool
	masterOffset    atomic.Int64
	lastMasterAck   atomic.Int64
	ctx             context.Context
	cancel          context.CancelFunc
	syncConn        net.Conn
	monitor         *FailoverMonitor
	lastLagDuration time.Duration
}

// NewReplicaManager initializes a replication manager for a node.
func NewReplicaManager(cache *store.Store) *ReplicaManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReplicaManager{
		cache:  cache,
		role:   RoleMaster,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Role returns current replication role (master or slave).
func (rm *ReplicaManager) Role() Role {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.role
}

// IsReadOnly returns true if node is acting as a replica.
func (rm *ReplicaManager) IsReadOnly() bool {
	return rm.Role() == RoleReplica
}

// SlaveOf transitions server to Replica role pointing to target master, or Master if NO ONE.
func (rm *ReplicaManager) SlaveOf(host string, port string) error {
	rm.mu.Lock()

	if strings.ToUpper(host) == "NO" && strings.ToUpper(port) == "ONE" {
		rm.role = RoleMaster
		rm.masterAddr = ""
		rm.masterLinkUp = false
		if rm.syncConn != nil {
			_ = rm.syncConn.Close()
			rm.syncConn = nil
		}
		if rm.monitor != nil {
			rm.monitor.Stop()
			rm.monitor = nil
		}
		rm.mu.Unlock()
		return nil
	}

	masterAddr := fmt.Sprintf("%s:%s", host, port)
	rm.role = RoleReplica
	rm.masterAddr = masterAddr
	rm.masterLinkUp = false

	if rm.syncConn != nil {
		_ = rm.syncConn.Close()
	}

	if rm.monitor != nil {
		rm.monitor.Stop()
	}

	// Create and start heartbeat failover monitor
	cfg := DefaultFailoverConfig(masterAddr)
	rm.monitor = NewFailoverMonitor(cfg, rm)
	rm.monitor.Start()

	rm.mu.Unlock()

	go rm.connectAndSync(masterAddr)
	return nil
}

// FailoverMonitor returns the current failover monitor if active.
func (rm *ReplicaManager) FailoverMonitor() *FailoverMonitor {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.monitor
}

func (rm *ReplicaManager) connectAndSync(masterAddr string) {
	conn, err := net.DialTimeout("tcp", masterAddr, 5*time.Second)
	if err != nil {
		return
	}

	rm.mu.Lock()
	rm.syncConn = conn
	rm.masterLinkUp = true
	rm.mu.Unlock()

	defer func() {
		_ = conn.Close()
		rm.mu.Lock()
		rm.masterLinkUp = false
		rm.mu.Unlock()
	}()

	reader := resp.NewReader(conn)
	writer := resp.NewWriter(conn)

	_ = writer.WriteArray([]resp.Value{
		resp.NewBulkStringFromString("PSYNC"),
		resp.NewBulkStringFromString("?"),
		resp.NewBulkStringFromString("-1"),
	})
	if err := writer.Flush(); err != nil {
		return
	}

	header, err := reader.ReadValue()
	if err != nil {
		return
	}
	_ = header

	rm.lastMasterAck.Store(time.Now().UnixNano())

	for {
		select {
		case <-rm.ctx.Done():
			return
		default:
		}

		cmdVal, err := reader.ReadValue()
		if err != nil {
			if err == io.EOF {
				return
			}
			return
		}

		rm.lastMasterAck.Store(time.Now().UnixNano())
		rm.masterOffset.Add(1)

		rm.applyMasterCommand(cmdVal)
	}
}

func (rm *ReplicaManager) applyMasterCommand(cmdVal resp.Value) {
	if cmdVal.Type != resp.TypeArray || len(cmdVal.Array) == 0 {
		return
	}

	cmdName := strings.ToUpper(extractString(cmdVal.Array[0]))
	args := cmdVal.Array[1:]

	switch cmdName {
	case "SET":
		if len(args) >= 2 {
			key := extractString(args[0])
			val := args[1].Bulk
			if val == nil {
				val = []byte(args[1].Str)
			}

			var ttl time.Duration
			for i := 2; i < len(args); i++ {
				flag := strings.ToUpper(extractString(args[i]))
				if (flag == "EX" || flag == "PX") && i+1 < len(args) {
					num, err := strconv.ParseInt(extractString(args[i+1]), 10, 64)
					if err == nil && num > 0 {
						if flag == "EX" {
							ttl = time.Duration(num) * time.Second
						} else {
							ttl = time.Duration(num) * time.Millisecond
						}
					}
				}
			}
			_ = rm.cache.Set(key, val, ttl)
		}
	case "DEL":
		for _, arg := range args {
			_ = rm.cache.Delete(extractString(arg))
		}
	case "EXPIRE":
		if len(args) == 2 {
			key := extractString(args[0])
			sec, err := strconv.ParseInt(extractString(args[1]), 10, 64)
			if err == nil {
				_ = rm.cache.Expire(key, time.Duration(sec)*time.Second)
			}
		}
	}
}

func extractString(v resp.Value) string {
	if v.Type == resp.TypeBulkString {
		return string(v.Bulk)
	}
	return v.Str
}

func (rm *ReplicaManager) Info() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	roleStr := string(rm.role)
	if rm.role == RoleMaster {
		return fmt.Sprintf("role:%s\r\nconnected_slaves:0\r\nmaster_repl_offset:%d", roleStr, rm.masterOffset.Load())
	}

	linkStatus := "down"
	if rm.masterLinkUp {
		linkStatus = "up"
	}

	var lagSec int64
	lastAck := rm.lastMasterAck.Load()
	if lastAck > 0 {
		lagSec = int64(time.Since(time.Unix(0, lastAck)).Seconds())
	}

	return fmt.Sprintf("role:%s\r\nmaster_host:%s\r\nmaster_link_status:%s\r\nmaster_last_io_seconds_ago:%d\r\nslave_repl_offset:%d",
		roleStr,
		rm.masterAddr,
		linkStatus,
		lagSec,
		rm.masterOffset.Load(),
	)
}

func (rm *ReplicaManager) Close() {
	rm.cancel()
	rm.mu.Lock()
	if rm.syncConn != nil {
		_ = rm.syncConn.Close()
	}
	if rm.monitor != nil {
		rm.monitor.Stop()
	}
	rm.mu.Unlock()
}
