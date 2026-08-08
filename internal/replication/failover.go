package replication

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"vortex-cache/internal/resp"
)

// FailoverConfig configures heartbeat monitoring parameters.
type FailoverConfig struct {
	HeartbeatInterval time.Duration // Interval between PING health checks. Default: 100ms.
	MaxMissedBeats    int           // Number of missed heartbeats before triggering failover. Default: 3.
	MasterAddr        string        // Target Master network address (host:port).
}

// DefaultFailoverConfig returns baseline sentinel heartbeat settings.
func DefaultFailoverConfig(masterAddr string) FailoverConfig {
	return FailoverConfig{
		HeartbeatInterval: 100 * time.Millisecond,
		MaxMissedBeats:    3,
		MasterAddr:        masterAddr,
	}
}

// FailoverMonitor handles periodic heartbeat health checking and automatic self-promotion.
type FailoverMonitor struct {
	cfg            FailoverConfig
	replica        *ReplicaManager
	mu             sync.Mutex
	missedBeats    int
	isMonitoring   bool
	ctx            context.Context
	cancel         context.CancelFunc
	failoverStart  time.Time
	failoverEnd    time.Time
	lastFailoverMs float64
}

// NewFailoverMonitor creates a new failover monitor instance.
func NewFailoverMonitor(cfg FailoverConfig, replica *ReplicaManager) *FailoverMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &FailoverMonitor{
		cfg:     cfg,
		replica: replica,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start begins periodic heartbeat monitoring of the target Master node.
func (fm *FailoverMonitor) Start() {
	fm.mu.Lock()
	if fm.isMonitoring {
		fm.mu.Unlock()
		return
	}
	fm.isMonitoring = true
	fm.missedBeats = 0
	fm.mu.Unlock()

	go fm.monitorLoop()
}

// monitorLoop periodically pings Master and triggers failover upon N consecutive failures.
func (fm *FailoverMonitor) monitorLoop() {
	ticker := time.NewTicker(fm.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fm.ctx.Done():
			return
		case <-ticker.C:
			if err := fm.checkHeartbeat(); err != nil {
				fm.mu.Lock()
				if fm.missedBeats == 0 {
					fm.failoverStart = time.Now()
				}
				fm.missedBeats++
				missed := fm.missedBeats
				fm.mu.Unlock()

				if missed >= fm.cfg.MaxMissedBeats {
					fm.triggerFailover()
					return
				}
			} else {
				fm.mu.Lock()
				fm.missedBeats = 0
				fm.mu.Unlock()
			}
		}
	}
}

// checkHeartbeat dials Master and issues a PING command.
func (fm *FailoverMonitor) checkHeartbeat() error {
	conn, err := net.DialTimeout("tcp", fm.cfg.MasterAddr, 200*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
	writer := resp.NewWriter(conn)
	reader := resp.NewReader(conn)

	if err := writer.WriteArray([]resp.Value{resp.NewBulkStringFromString("PING")}); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	val, err := reader.ReadValue()
	if err != nil || (val.Str != "PONG" && string(val.Bulk) != "PONG") {
		return fmt.Errorf("invalid heartbeat response: %v", err)
	}
	return nil
}

// triggerFailover executes automatic self-promotion of replica to master.
func (fm *FailoverMonitor) triggerFailover() {
	fm.mu.Lock()
	fm.isMonitoring = false
	fm.failoverEnd = time.Now()
	if !fm.failoverStart.IsZero() {
		fm.lastFailoverMs = float64(fm.failoverEnd.Sub(fm.failoverStart).Microseconds()) / 1000.0
	}
	fm.mu.Unlock()

	// Execute self-promotion: transition from Replica to Master (removes READONLY)
	_ = fm.replica.SlaveOf("NO", "ONE")
}

// LastFailoverDurationMs returns the measured duration in milliseconds of the last failover event.
func (fm *FailoverMonitor) LastFailoverDurationMs() float64 {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	return fm.lastFailoverMs
}

// Stop terminates the failover monitor loop.
func (fm *FailoverMonitor) Stop() {
	fm.cancel()
	fm.mu.Lock()
	fm.isMonitoring = false
	fm.mu.Unlock()
}
