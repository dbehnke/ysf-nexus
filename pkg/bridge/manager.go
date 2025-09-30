package bridge

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// Manager manages multiple bridge connections with scheduling
type Manager struct {
	config      []config.BridgeConfig
	logger      *logger.Logger
	server      NetworkServer
	cron        *cron.Cron
	
	// Bridge tracking
	mu      sync.RWMutex
	bridges map[string]*Bridge
	
	// Schedule tracking for missed recovery
	schedules map[string]*ScheduleInfo
	
	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
	
	// Statistics
	stats BridgeStats
}

// ScheduleInfo tracks schedule information for missed recovery
type ScheduleInfo struct {
	Name           string
	Schedule       string
	Duration       time.Duration
	LastExecution  *time.Time
	NextExecution  *time.Time
	MissedWindows  int
}

// BridgeStats tracks overall bridge statistics
type BridgeStats struct {
	ActiveBridges    int
	ScheduledBridges int
	FailedBridges    int
	TotalConnections uint64
	MissedSchedules  int
}

// BridgeState represents the current state of a bridge
type BridgeState string

const (
	StateDisconnected BridgeState = "disconnected"
	StateConnecting   BridgeState = "connecting"
	StateConnected    BridgeState = "connected"
	StateFailed       BridgeState = "failed"
	StateScheduled    BridgeState = "scheduled"
)

// BridgeStatus holds runtime status information for a bridge
type BridgeStatus struct {
	Name           string        `json:"name"`
	State          BridgeState   `json:"state"`
	ConnectedAt    *time.Time    `json:"connected_at,omitempty"`
	DisconnectedAt *time.Time    `json:"disconnected_at,omitempty"`
	NextSchedule   *time.Time    `json:"next_schedule,omitempty"`
	Duration       time.Duration `json:"duration,omitempty"`
	RetryCount     int           `json:"retry_count"`
	LastError      string        `json:"last_error,omitempty"`
	PacketsRx      uint64        `json:"packets_rx"`
	PacketsTx      uint64        `json:"packets_tx"`
	BytesRx        uint64        `json:"bytes_rx"`
	BytesTx        uint64        `json:"bytes_tx"`
}

// NewManager creates a new bridge manager
func NewManager(config []config.BridgeConfig, server NetworkServer, logger *logger.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Manager{
		config:    config,
		logger:    logger,
		server:    server,
		cron:      cron.New(cron.WithSeconds()),
		bridges:   make(map[string]*Bridge),
		schedules: make(map[string]*ScheduleInfo),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start initializes and starts all configured bridges
func (m *Manager) Start() error {
	m.logger.Info("Starting bridge manager")
	
	for _, bridgeConfig := range m.config {
		if err := m.setupBridge(bridgeConfig); err != nil {
			m.logger.Error("Failed to setup bridge", 
				logger.String("name", bridgeConfig.Name), 
				logger.Error(err))
			continue
		}
	}
	
	// Start the cron scheduler
	m.cron.Start()
	
	// Start the missed schedule recovery checker
	go m.runMissedScheduleRecovery()
	
	m.logger.Info("Bridge manager started", logger.Int("bridges", len(m.bridges)))
	return nil
}

// runMissedScheduleRecovery periodically checks for missed schedules and recovers them
func (m *Manager) runMissedScheduleRecovery() {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkMissedSchedules()
		}
	}
}

// setupBridge configures a bridge based on its type (permanent or scheduled)
func (m *Manager) setupBridge(config config.BridgeConfig) error {
	bridge := NewBridge(config, m.server, m.logger)

	m.mu.Lock()
	m.bridges[config.Name] = bridge
	m.mu.Unlock()

	if config.Permanent {
		// Start permanent bridge immediately
		go bridge.RunPermanent(m.ctx)
		m.logger.Info("Started permanent bridge", logger.String("name", config.Name))
	} else if config.Schedule != "" {
		// Set up schedule tracking for missed recovery
		m.setupScheduleTracking(config)
		
		// Schedule the bridge using cron
		_, err := m.cron.AddFunc(config.Schedule, func() {
			m.startScheduledBridge(config.Name, config.Duration)
		})
		if err != nil {
			return fmt.Errorf("failed to schedule bridge %s: %w", config.Name, err)
		}
		
		m.logger.Info("Scheduled bridge", 
			logger.String("name", config.Name), 
			logger.String("schedule", config.Schedule))
		
		// Check if we should start this bridge now (missed schedule recovery)
		if m.shouldStartNow(config) {
			m.logger.Info("Recovering missed schedule", logger.String("name", config.Name))
			go m.startScheduledBridge(config.Name, config.Duration)
		}
	}
	
	return nil
}

// setupScheduleTracking initializes schedule tracking for missed recovery
func (m *Manager) setupScheduleTracking(config config.BridgeConfig) {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(config.Schedule)
	if err != nil {
		m.logger.Error("Failed to parse schedule for tracking",
			logger.String("name", config.Name),
			logger.Error(err))
		return
	}

	now := time.Now()
	nextRun := schedule.Next(now)

	m.mu.Lock()
	m.schedules[config.Name] = &ScheduleInfo{
		Name:          config.Name,
		Schedule:      config.Schedule,
		Duration:      config.Duration,
		NextExecution: &nextRun,
	}

	// Update the bridge's nextSchedule field
	if bridge, ok := m.bridges[config.Name]; ok {
		bridge.SetNextSchedule(&nextRun)
	}
	m.mu.Unlock()
}

// shouldStartNow determines if a scheduled bridge should start now due to missed schedule
func (m *Manager) shouldStartNow(config config.BridgeConfig) bool {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(config.Schedule)
	if err != nil {
		return false
	}
	
	now := time.Now()
	
	// Look back to see if we missed a recent schedule
	// Check the last 2 hours for potential missed schedules
	checkFrom := now.Add(-2 * time.Hour)
	
	// Find the most recent scheduled time before now
	lastScheduled := schedule.Next(checkFrom)
	for {
		nextScheduled := schedule.Next(lastScheduled)
		if nextScheduled.After(now) {
			break
		}
		lastScheduled = nextScheduled
	}
	
	// Check if we're within the duration window of the last scheduled time
	if lastScheduled.Before(now) {
		windowEnd := lastScheduled.Add(config.Duration)
		if now.Before(windowEnd) {
			// We're within the scheduled window, should be running
			m.logger.Info("Detected missed schedule within window", 
				logger.String("name", config.Name),
				logger.Any("scheduled_at", lastScheduled),
				logger.Any("window_ends", windowEnd),
				logger.Any("now", now))
			return true
		}
	}
	
	return false
}

// startScheduledBridge starts a bridge for its scheduled duration
func (m *Manager) startScheduledBridge(name string, duration time.Duration) {
	m.mu.RLock()
	bridge, exists := m.bridges[name]
	m.mu.RUnlock()

	if !exists {
		m.logger.Error("Attempted to start unknown bridge", logger.String("name", name))
		return
	}

	m.logger.Info("Starting scheduled bridge",
		logger.String("name", name),
		logger.Duration("duration", duration))

	// Create an independent context for this bridge that won't affect the manager
	// The bridge will manage its own timeout via RunScheduled's WithTimeout
	bridgeCtx, cancel := context.WithCancel(context.Background())

	// Run the bridge for the scheduled duration in a goroutine
	go func() {
		defer cancel() // Clean up context when bridge completes

		bridge.RunScheduled(bridgeCtx, duration)

		// After the bridge completes, update the next schedule time
		m.updateScheduleExecution(name)

		m.logger.Info("Scheduled bridge completed, next run scheduled",
			logger.String("name", name))
	}()
}

// updateScheduleExecution updates the schedule tracking information
func (m *Manager) updateScheduleExecution(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if schedInfo, exists := m.schedules[name]; exists {
		now := time.Now()
		schedInfo.LastExecution = &now

		// Calculate next execution time
		parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		if schedule, err := parser.Parse(schedInfo.Schedule); err == nil {
			next := schedule.Next(now)
			schedInfo.NextExecution = &next

			// Update the bridge's nextSchedule field so it shows in status
			if bridge, ok := m.bridges[name]; ok {
				bridge.SetNextSchedule(&next)
			}
		}
	}
}

// checkMissedSchedules checks for schedules that should be running but aren't
func (m *Manager) checkMissedSchedules() {
	m.mu.RLock()
	schedules := make([]*ScheduleInfo, 0, len(m.schedules))
	for _, sched := range m.schedules {
		schedules = append(schedules, sched)
	}
	m.mu.RUnlock()
	
	for _, schedInfo := range schedules {
		if m.shouldRecoverSchedule(schedInfo) {
			m.logger.Info("Recovering missed schedule", 
				logger.String("name", schedInfo.Name),
				logger.Any("last_execution", schedInfo.LastExecution))
			
			m.mu.Lock()
			schedInfo.MissedWindows++
			m.stats.MissedSchedules++
			m.mu.Unlock()
			
			// Find the bridge config to get duration
			var bridgeConfig *config.BridgeConfig
			for _, cfg := range m.config {
				if cfg.Name == schedInfo.Name {
					bridgeConfig = &cfg
					break
				}
			}
			
			if bridgeConfig != nil {
				go m.startScheduledBridge(schedInfo.Name, bridgeConfig.Duration)
			}
		}
	}
}

// shouldRecoverSchedule determines if a schedule should be recovered
func (m *Manager) shouldRecoverSchedule(schedInfo *ScheduleInfo) bool {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(schedInfo.Schedule)
	if err != nil {
		return false
	}
	
	now := time.Now()
	
	// If we have a last execution time, use it as reference
	checkFrom := now.Add(-1 * time.Hour) // Default to 1 hour back
	if schedInfo.LastExecution != nil {
		checkFrom = *schedInfo.LastExecution
	}
	
	// Find the most recent scheduled time that we might have missed
	lastScheduled := schedule.Next(checkFrom)
	for {
		nextScheduled := schedule.Next(lastScheduled)
		if nextScheduled.After(now) {
			break
		}
		lastScheduled = nextScheduled
	}
	
	// Check if we're within the duration window of a missed schedule
	if lastScheduled.After(checkFrom) && lastScheduled.Before(now) {
		windowEnd := lastScheduled.Add(schedInfo.Duration)
		if now.Before(windowEnd) {
			// We should be running but aren't - check if bridge is actually active
			m.mu.RLock()
			bridge, exists := m.bridges[schedInfo.Name]
			m.mu.RUnlock()
			
			if exists {
				status := bridge.GetStatus()
				// Only recover if the bridge is not currently connected or scheduled
				if status.State == StateDisconnected || status.State == StateFailed {
					return true
				}
			}
		}
	}
	
	return false
}

// shouldBeActive checks if a scheduled bridge should currently be active
// This handles missed cron triggers by checking if we're within the schedule window
func (m *Manager) shouldBeActive(cfg config.BridgeConfig) bool {
	if cfg.Permanent {
		return true // Permanent bridges are always active
	}
	
	// Parse the cron schedule to determine the next run time
	schedule, err := cron.ParseStandard(cfg.Schedule)
	if err != nil {
		m.logger.Error("Failed to parse bridge schedule", 
			logger.String("bridge", cfg.Name), 
			logger.String("schedule", cfg.Schedule), 
			logger.Error(err))
		return false
	}
	
	now := time.Now()
	
	// Get the previous scheduled time (when this should have started)
	prevRun := schedule.Next(now.Add(-24 * time.Hour)) // Look back up to 24 hours
	for prevRun.Before(now) && now.Sub(prevRun) > 24*time.Hour {
		prevRun = schedule.Next(prevRun)
	}
	
	// Check if we're within the duration window of the last scheduled time
	if prevRun.Before(now) && now.Sub(prevRun) <= cfg.Duration {
		return true
	}
	
	return false
}

// Stop stops all bridges and the scheduler
func (m *Manager) Stop() {
	m.logger.Info("Stopping bridge manager")
	
	// Stop the scheduler
	m.cron.Stop()
	
	// Cancel all bridge contexts
	m.cancel()
	
	m.logger.Info("Bridge manager stopped")
}

// GetStatus returns the status of all bridges
func (m *Manager) GetStatus() map[string]BridgeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	status := make(map[string]BridgeStatus)
	for name, bridge := range m.bridges {
		status[name] = bridge.GetStatus()
	}
	
	return status
}

// GetBridge returns a bridge by name
func (m *Manager) GetBridge(name string) *Bridge {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.bridges[name]
}

// IsBridgeAddress checks if an address belongs to a bridge connection
func (m *Manager) IsBridgeAddress(addr *net.UDPAddr) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, bridge := range m.bridges {
		if bridge.IsConnectedTo(addr) {
			return true
		}
	}
	return false
}

// HandleIncomingPacket processes packets received from bridge connections
func (m *Manager) HandleIncomingPacket(data []byte, fromAddr *net.UDPAddr) {
	// Forward packets from bridge connections to all local repeaters
	// This will be called by the network server when it receives packets
	// from addresses that are known bridge connections
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Find which bridge this packet came from
	for _, bridge := range m.bridges {
		if bridge.IsConnectedTo(fromAddr) {
			m.logger.Debug("Received packet from bridge", 
				logger.String("bridge", bridge.GetName()), 
				logger.String("addr", fromAddr.String()), 
				logger.Int("size", len(data)))
			
			bridge.IncrementRxStats(uint64(len(data)))
			
			// Notify bridge of received packet (for ping response handling)
			bridge.OnPacketReceived(data)
			
			// TODO: Forward to local repeaters (implement packet routing)
			// This would integrate with the main reflector's packet routing system
			break
		}
	}
}

// GetConnectedAddresses returns the addresses of all currently connected bridges
func (m *Manager) GetConnectedAddresses() []*net.UDPAddr {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var addresses []*net.UDPAddr
	for _, bridge := range m.bridges {
		if bridge.IsConnected() {
			if addr := bridge.GetRemoteAddr(); addr != nil {
				addresses = append(addresses, addr)
			}
		}
	}
	return addresses
}