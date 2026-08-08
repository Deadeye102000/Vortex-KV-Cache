package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"vortex-cache/internal/replication"
	"vortex-cache/internal/resp"
	"vortex-cache/internal/store"
)

// Server represents the Vortex TCP server handling Redis protocol connections.
type Server struct {
	addr        string
	cache       *store.Store
	masterMgr   *replication.MasterManager
	replicaMgr  *replication.ReplicaManager
	listener    net.Listener
	mu          sync.RWMutex
	startTime   time.Time
	activeConns atomic.Int64
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewServer initializes a new Server for the target network address.
func NewServer(addr string, cache *store.Store) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	masterMgr := replication.NewMasterManager(cache)
	replicaMgr := replication.NewReplicaManager(cache)

	return &Server{
		addr:       addr,
		cache:      cache,
		masterMgr:  masterMgr,
		replicaMgr: replicaMgr,
		startTime:  time.Now(),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// MasterManager returns the server's master replication manager.
func (s *Server) MasterManager() *replication.MasterManager {
	return s.masterMgr
}

// ReplicaManager returns the server's replica replication manager.
func (s *Server) ReplicaManager() *replication.ReplicaManager {
	return s.replicaMgr
}

// Listen binds the TCP socket synchronously.
func (s *Server) Listen() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return nil
	}

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to bind TCP socket on %s: %w", s.addr, err)
	}
	s.listener = listener
	return nil
}

// Serve enters the connection accept loop using an existing listener.
func (s *Server) Serve() error {
	s.mu.RLock()
	listener := s.listener
	s.mu.RUnlock()

	if listener == nil {
		if err := s.Listen(); err != nil {
			return err
		}
		s.mu.RLock()
		listener = s.listener
		s.mu.RUnlock()
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				return err
			}
		}

		s.wg.Add(1)
		s.activeConns.Add(1)
		go s.handleConn(conn)
	}
}

// ListenAndServe binds the socket and starts serving connections.
func (s *Server) ListenAndServe() error {
	if err := s.Listen(); err != nil {
		return err
	}
	return s.Serve()
}

// handleConn manages individual client TCP connections.
func (s *Server) handleConn(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		s.activeConns.Add(-1)
		s.wg.Done()
	}()

	reader := resp.NewReader(conn)
	writer := resp.NewWriter(conn)
	handler := NewHandler(s.cache, s.replicaMgr, s.masterMgr, s.startTime, conn)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		cmdVal, err := reader.ReadValue()
		if err != nil {
			if err == io.EOF {
				return
			}
			_ = writer.WriteError(fmt.Sprintf("ERR %v", err))
			_ = writer.Flush()
			return
		}

		respVal := handler.Dispatch(cmdVal)
		if err := writer.WriteValue(respVal); err != nil {
			return
		}

		if err := writer.Flush(); err != nil {
			return
		}
	}
}

// Close gracefully stops the TCP server and replication managers.
func (s *Server) Close() error {
	s.cancel()

	if s.replicaMgr != nil {
		s.replicaMgr.Close()
	}

	s.mu.Lock()
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}

// ActiveConns returns count of active client connections.
func (s *Server) ActiveConns() int64 {
	return s.activeConns.Load()
}

// Addr returns bound network address.
func (s *Server) Addr() net.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}
