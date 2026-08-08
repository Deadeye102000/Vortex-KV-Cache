package replication

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"vortex-cache/internal/resp"
	"vortex-cache/internal/store"
)

// MasterManager manages replica TCP streams and broadcasts live store mutations.
type MasterManager struct {
	cache       *store.Store
	replicasMu  sync.RWMutex
	replicas    map[net.Conn]*resp.Writer
	totalOffset int64
}

// NewMasterManager creates a new master replication manager.
func NewMasterManager(cache *store.Store) *MasterManager {
	m := &MasterManager{
		cache:    cache,
		replicas: make(map[net.Conn]*resp.Writer),
	}

	// Register store write hook to broadcast mutations to all active replicas
	cache.OnWrite(m.handleStoreWrite)
	return m
}

// RegisterReplica performs the full-resync snapshot transmission and registers replica for live updates.
func (m *MasterManager) RegisterReplica(conn net.Conn) error {
	writer := resp.NewWriter(conn)

	// 1. Take point-in-time snapshot of current non-expired store entries
	snapshot := m.cache.Snapshot()

	// Send confirmation header
	if err := writer.WriteSimpleString("FULLRESYNC 0000000000000000000000000000000000000000 0"); err != nil {
		return err
	}
	_ = writer.Flush()

	// 2. Stream snapshot as initial SET commands over TCP
	for _, entry := range snapshot {
		args := []resp.Value{
			resp.NewBulkStringFromString("SET"),
			resp.NewBulkStringFromString(entry.Key),
			resp.NewBulkString(entry.Value),
		}
		if entry.TTL > 0 {
			ms := entry.TTL.Milliseconds()
			if ms > 0 {
				args = append(args, resp.NewBulkStringFromString("PX"), resp.NewBulkStringFromString(strconv.FormatInt(ms, 10)))
			}
		}
		if err := writer.WriteArray(args); err != nil {
			return fmt.Errorf("failed to stream snapshot entry: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush snapshot: %w", err)
	}

	// 3. Register persistent replica writer for live updates
	m.replicasMu.Lock()
	m.replicas[conn] = writer
	m.replicasMu.Unlock()

	return nil
}

// UnregisterReplica removes a disconnected replica connection.
func (m *MasterManager) UnregisterReplica(conn net.Conn) {
	m.replicasMu.Lock()
	delete(m.replicas, conn)
	m.replicasMu.Unlock()
}

// ConnectedReplicas returns the count of active streaming replicas.
func (m *MasterManager) ConnectedReplicas() int {
	m.replicasMu.RLock()
	defer m.replicasMu.RUnlock()
	return len(m.replicas)
}

// handleStoreWrite receives store mutation events and broadcasts them to active replicas.
func (m *MasterManager) handleStoreWrite(cmdName string, key string, val []byte, ttl time.Duration) {
	m.replicasMu.RLock()
	if len(m.replicas) == 0 {
		m.replicasMu.RUnlock()
		return
	}

	// Build RESP array for mutation command
	var args []resp.Value
	switch cmdName {
	case "SET":
		args = []resp.Value{
			resp.NewBulkStringFromString("SET"),
			resp.NewBulkStringFromString(key),
			resp.NewBulkString(val),
		}
		if ttl > 0 {
			ms := ttl.Milliseconds()
			if ms > 0 {
				args = append(args, resp.NewBulkStringFromString("PX"), resp.NewBulkStringFromString(strconv.FormatInt(ms, 10)))
			}
		}
	case "DEL":
		args = []resp.Value{
			resp.NewBulkStringFromString("DEL"),
			resp.NewBulkStringFromString(key),
		}
	case "EXPIRE":
		args = []resp.Value{
			resp.NewBulkStringFromString("EXPIRE"),
			resp.NewBulkStringFromString(key),
			resp.NewBulkStringFromString(strconv.FormatInt(int64(ttl.Seconds()), 10)),
		}
	default:
		m.replicasMu.RUnlock()
		return
	}

	cmdVal := resp.NewArray(args)

	// Broadcast to all active replica stream connections
	for conn, writer := range m.replicas {
		if err := writer.WriteValue(cmdVal); err != nil || writer.Flush() != nil {
			// Asynchronously drop failed connection
			go m.UnregisterReplica(conn)
		}
	}
	m.replicasMu.RUnlock()
}
