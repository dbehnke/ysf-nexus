package bridge

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
	"github.com/robfig/cron/v3"
)

// BridgeRunner is the common interface for all bridge types
type BridgeRunner interface {
	RunPermanent(ctx context.Context)
	RunScheduled(ctx context.Context, duration time.Duration)
	GetStatus() BridgeStatus
	GetName() string
	GetType() string
	SetNextSchedule(t *time.Time)
	IsConnected() bool
	Disconnect() error
}

// Manager manages multiple bridge connections with scheduling
type Manager struct {
	config []config.BridgeConfig
	logger *logger.Logger
	server NetworkServer
	cron   *cron.Cron

	// Bridge tracking (now supports both YSF and DMR bridges)
	mu      sync.RWMutex
	bridges map[string]BridgeRunner

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
	Name          string
	Schedule      string
	Duration      time.Duration
	LastExecution *time.Time
	NextExecution *time.Time
	MissedWindows int
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
	Name           string                 `json:"name"`
	Type           string                 `json:"type"` // Bridge type: "ysf" or "dmr"
	State          BridgeState            `json:"state"`
	ConnectedAt    *time.Time             `json:"connected_at,omitempty"`
	DisconnectedAt *time.Time             `json:"disconnected_at,omitempty"`
	NextSchedule   *time.Time             `json:"next_schedule,omitempty"`
	Duration       time.Duration          `json:"duration,omitempty"`
	RetryCount     int                    `json:"retry_count"`
	LastError      string                 `json:"last_error,omitempty"`
	PacketsRx      uint64                 `json:"packets_rx"`
	PacketsTx      uint64                 `json:"packets_tx"`
	BytesRx        uint64                 `json:"bytes_rx"`
	BytesTx        uint64                 `json:"bytes_tx"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"` // Type-specific metadata
}

// NewManager creates a new bridge manager
func NewManager(config []config.BridgeConfig, server NetworkServer, logger *logger.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:    config,
		logger:    logger,
		server:    server,
		cron:      cron.New(cron.WithSeconds()),
		bridges:   make(map[string]BridgeRunner),
		schedules: make(map[string]*ScheduleInfo),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start initializes and starts all configured bridges
func (m *Manager) Start() error {
	m.logger.Info("Starting bridge manager")

	for _, bridgeConfig := range m.config {
		// Skip disabled bridges
		if !bridgeConfig.Enabled {
			m.logger.Info("Skipping disabled bridge", logger.String("name", bridgeConfig.Name))
			continue
		}

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
	// Default type to "ysf" if not specified
	bridgeType := config.Type
	if bridgeType == "" {
		bridgeType = "ysf"
	}

	var bridge BridgeRunner
	var err error

	// Create appropriate bridge type
	switch bridgeType {
	case "ysf":
		ysfBridge := NewBridge(config, m.server, m.logger)
		bridge = ysfBridge
		m.logger.Info("Created YSF bridge", logger.String("name", config.Name))

	case "dmr":
		dmrBridge, err := NewDMRBridgeAdapter(config, m.logger)
		if err != nil {
			return fmt.Errorf("failed to create DMR bridge %s: %w", config.Name, err)
		}
		bridge = dmrBridge
		m.logger.Info("Created DMR bridge",
			logger.String("name", config.Name),
			logger.String("network", config.DMR.Network),
			logger.Uint32("talk_group", config.DMR.TalkGroup))

	default:
		return fmt.Errorf("unsupported bridge type: %s", bridgeType)
	}

	m.mu.Lock()
	m.bridges[config.Name] = bridge
	m.mu.Unlock()

	if config.Permanent {
		// Start permanent bridge immediately
		go bridge.RunPermanent(m.ctx)
		m.logger.Info("Started permanent bridge",
			logger.String("name", config.Name),
			logger.String("type", bridgeType))
	} else if config.Schedule != "" {
		// Set up schedule tracking for missed recovery
		m.setupScheduleTracking(config)

		// Schedule the bridge using cron
		_, err = m.cron.AddFunc(config.Schedule, func() {
			m.startScheduledBridge(config.Name, config.Duration)
		})
		if err != nil {
			return fmt.Errorf("failed to schedule bridge %s: %w", config.Name, err)
		}

		m.logger.Info("Scheduled bridge",
			logger.String("name", config.Name),
			logger.String("type", bridgeType),
			logger.String("schedule", config.Schedule))

		// Check if we should start this bridge now (missed schedule recovery)
		if shouldStart, remainingDuration := m.shouldStartNowWithDuration(config); shouldStart {
			m.logger.Info("Recovering missed schedule",
				logger.String("name", config.Name),
				logger.Duration("remaining_duration", remainingDuration))
			go m.startScheduledBridge(config.Name, remainingDuration)
		}
	}

	return nil
}

// setupScheduleTracking initializes schedule tracking for missed recovery
func (m *Manager) setupScheduleTracking(config config.BridgeConfig) {
	schedule, err := parseSchedule(config.Schedule)
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

// (deprecated wrapper removed)

// shouldStartNowWithDuration determines if a scheduled bridge should start now and returns remaining duration
func (m *Manager) shouldStartNowWithDuration(config config.BridgeConfig) (bool, time.Duration) {
	schedule, err := parseSchedule(config.Schedule)
	if err != nil {
		return false, 0
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
			// Calculate remaining duration
			remainingDuration := windowEnd.Sub(now)

			m.logger.Info("Detected missed schedule within window",
				logger.String("name", config.Name),
				logger.Any("scheduled_at", lastScheduled),
				logger.Any("window_ends", windowEnd),
				logger.Any("now", now),
				logger.Duration("remaining_duration", remainingDuration))
			return true, remainingDuration
		}
	}

	return false, 0
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

// parseSchedule tries parsing a cron expression that may include seconds (6 fields)
// or just standard minute-based expressions (5 fields). It tries the parser with
// seconds first, and falls back to the 5-field parser.
func parseSchedule(expr string) (cron.Schedule, error) {
	// Try with seconds
	secParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if sched, err := secParser.Parse(expr); err == nil {
		return sched, nil
	}

	// Fall back to minute-based (no seconds)
	minParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	return minParser.Parse(expr)
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
		if shouldRecover, remainingDuration := m.shouldRecoverScheduleWithDuration(schedInfo); shouldRecover {
			m.logger.Info("Recovering missed schedule",
				logger.String("name", schedInfo.Name),
				logger.Any("last_execution", schedInfo.LastExecution),
				logger.Duration("remaining_duration", remainingDuration))

			m.mu.Lock()
			schedInfo.MissedWindows++
			m.stats.MissedSchedules++
			m.mu.Unlock()

			go m.startScheduledBridge(schedInfo.Name, remainingDuration)
		}
	}
}

// (deprecated wrapper removed)

// shouldRecoverScheduleWithDuration determines if a schedule should be recovered and returns remaining duration
func (m *Manager) shouldRecoverScheduleWithDuration(schedInfo *ScheduleInfo) (bool, time.Duration) {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(schedInfo.Schedule)
	if err != nil {
		return false, 0
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
					// Calculate remaining duration
					remainingDuration := windowEnd.Sub(now)
					return true, remainingDuration
				}
			}
		}
	}

	return false, 0
}

// shouldBeActive was removed because it was unused; schedule checking is handled
// by shouldStartNow and related helpers in this manager.

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

// GetBridge returns a bridge by name (returns YSF bridge or nil if DMR)
func (m *Manager) GetBridge(name string) *Bridge {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Type assert to *Bridge - will be nil if it's a DMR bridge
	if bridge, ok := m.bridges[name].(*Bridge); ok {
		return bridge
	}
	return nil
}

// IsBridgeAddress checks if an address belongs to a YSF bridge connection
// (DMR bridges don't use UDP addresses in the same way)
func (m *Manager) IsBridgeAddress(addr *net.UDPAddr) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, bridgeRunner := range m.bridges {
		// Only YSF bridges have IsConnectedTo method
		if ysfBridge, ok := bridgeRunner.(*Bridge); ok {
			if ysfBridge.IsConnectedTo(addr) {
				return true
			}
		}
	}
	return false
}

// HandleIncomingPacket processes packets received from YSF bridge connections
// (Only applies to YSF bridges, DMR bridges handle packets internally)
func (m *Manager) HandleIncomingPacket(data []byte, fromAddr *net.UDPAddr) {
	// Forward packets from bridge connections to all local repeaters
	// This will be called by the network server when it receives packets
	// from addresses that are known bridge connections

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find which YSF bridge this packet came from
	for _, bridgeRunner := range m.bridges {
		if ysfBridge, ok := bridgeRunner.(*Bridge); ok {
			if ysfBridge.IsConnectedTo(fromAddr) {
				m.logger.Debug("Received packet from YSF bridge",
					logger.String("bridge", ysfBridge.GetName()),
					logger.String("addr", fromAddr.String()),
					logger.Int("size", len(data)))

				ysfBridge.IncrementRxStats(uint64(len(data)))

				// Notify bridge of received packet (for ping response handling)
				ysfBridge.OnPacketReceived(data)

				// TODO: Forward to local repeaters (implement packet routing)
				// This would integrate with the main reflector's packet routing system
				break
			}
		}
	}
}

// GetConnectedAddresses returns the addresses of all currently connected YSF bridges
// (DMR bridges don't have UDP addresses)
func (m *Manager) GetConnectedAddresses() []*net.UDPAddr {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var addresses []*net.UDPAddr
	for _, bridgeRunner := range m.bridges {
		// Only YSF bridges have GetRemoteAddr
		if ysfBridge, ok := bridgeRunner.(*Bridge); ok {
			if ysfBridge.IsConnected() {
				if addr := ysfBridge.GetRemoteAddr(); addr != nil {
					addresses = append(addresses, addr)
				}
			}
		}
	}
	return addresses
}
