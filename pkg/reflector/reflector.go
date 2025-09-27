package reflector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
	"github.com/dbehnke/ysf-nexus/pkg/network"
	"github.com/dbehnke/ysf-nexus/pkg/repeater"
)

// Reflector represents the main YSF reflector application
type Reflector struct {
	config          *config.Config
	logger          *logger.Logger
	server          *network.Server
	repeaterManager *repeater.Manager
	eventChan       chan repeater.Event
	running         bool
	mu              sync.RWMutex
}

// New creates a new YSF reflector
func New(cfg *config.Config, log *logger.Logger) *Reflector {
	eventChan := make(chan repeater.Event, 1000)

	r := &Reflector{
		config:    cfg,
		logger:    log.WithComponent("reflector"),
		eventChan: eventChan,
	}

	// Initialize network server
	r.server = network.NewServer(cfg.Server.Host, cfg.Server.Port)
	r.server.SetDebug(cfg.Logging.Level == "debug")

	// Initialize repeater manager
	r.repeaterManager = repeater.NewManager(
		cfg.Server.Timeout,
		cfg.Server.MaxConnections,
		eventChan,
	)

	// Set up blocklist if configured
	if cfg.Blocklist.Enabled && len(cfg.Blocklist.Callsigns) > 0 {
		r.repeaterManager.GetBlocklist().SetBlocked(cfg.Blocklist.Callsigns)
		r.logger.Info("Blocklist configured",
			logger.Int("blocked_callsigns", len(cfg.Blocklist.Callsigns)))
	}

	// Register packet handlers
	r.registerHandlers()

	return r
}

// Start starts the reflector
func (r *Reflector) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("reflector already running")
	}
	r.running = true
	r.mu.Unlock()

	r.logger.Info("Starting YSF Nexus reflector",
		logger.String("host", r.config.Server.Host),
		logger.Int("port", r.config.Server.Port),
		logger.String("name", r.config.Server.Name),
		logger.Int("max_connections", r.config.Server.MaxConnections))

	var wg sync.WaitGroup

	// Start event processor
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.processEvents(ctx)
	}()

	// Start repeater cleanup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.repeaterManager.StartCleanup(ctx)
	}()

	// Start periodic stats logging
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.logStats(ctx)
	}()

	// Start network server (blocking)
	serverErr := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := r.server.Start(ctx); err != nil {
			serverErr <- err
		}
	}()

	// Wait for either context cancellation or server error
	select {
	case err := <-serverErr:
		r.logger.Error("Server error", logger.Error(err))
		return err
	case <-ctx.Done():
		r.logger.Info("Shutdown signal received")
	}

	// Wait for all goroutines to finish
	wg.Wait()

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	r.logger.Info("YSF Nexus reflector stopped")
	return nil
}

// IsRunning returns whether the reflector is running
func (r *Reflector) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// GetStats returns current reflector statistics
func (r *Reflector) GetStats() *Stats {
	managerStats := r.repeaterManager.GetStats()
	networkMetrics := r.server.GetMetrics()

	return &Stats{
		Uptime:           time.Since(networkMetrics.Uptime),
		ActiveRepeaters:  managerStats.ActiveRepeaters,
		TotalConnections: managerStats.TotalConnections,
		TotalPackets:     managerStats.TotalPackets,
		PacketsReceived:  networkMetrics.PacketsReceived,
		PacketsSent:      networkMetrics.PacketsSent,
		BytesReceived:    networkMetrics.BytesReceived,
		BytesSent:        networkMetrics.BytesSent,
		RepeaterStats:    managerStats,
	}
}

// Stats represents reflector statistics
type Stats struct {
	Uptime           time.Duration                `json:"uptime"`
	ActiveRepeaters  int                          `json:"active_repeaters"`
	TotalConnections uint64                       `json:"total_connections"`
	TotalPackets     uint64                       `json:"total_packets"`
	PacketsReceived  map[string]int64             `json:"packets_received"`
	PacketsSent      map[string]int64             `json:"packets_sent"`
	BytesReceived    int64                        `json:"bytes_received"`
	BytesSent        int64                        `json:"bytes_sent"`
	RepeaterStats    repeater.ManagerStats        `json:"repeater_stats"`
}

// registerHandlers registers packet handlers with the network server
func (r *Reflector) registerHandlers() {
	r.server.RegisterHandler(network.PacketTypePoll, r.handlePollPacket)
	r.server.RegisterHandler(network.PacketTypeData, r.handleDataPacket)
	r.server.RegisterHandler(network.PacketTypeUnlink, r.handleUnlinkPacket)
	r.server.RegisterHandler(network.PacketTypeStatus, r.handleStatusPacket)
}

// handlePollPacket handles YSFP (poll) packets
func (r *Reflector) handlePollPacket(packet *network.Packet) error {
	r.logger.Debug("Received poll packet",
		logger.String("callsign", packet.Callsign),
		logger.String("source", packet.Source.String()))

	// Add or update repeater
	rep, isNew := r.repeaterManager.AddRepeater(packet.Callsign, packet.Source)
	if rep == nil {
		r.logger.Warn("Repeater blocked or rejected",
			logger.String("callsign", packet.Callsign),
			logger.String("source", packet.Source.String()))
		return nil
	}

	if isNew {
		r.logger.Info("New repeater registered",
			logger.String("callsign", packet.Callsign),
			logger.String("source", packet.Source.String()))
	}

	// Process packet for statistics
	r.repeaterManager.ProcessPacket(packet.Callsign, packet.Source, packet.Type, len(packet.Data))

	// Send poll response
	response := network.CreatePollResponse()
	if err := r.server.SendPacket(response, packet.Source); err != nil {
		r.logger.Error("Failed to send poll response",
			logger.String("callsign", packet.Callsign),
			logger.Error(err))
		return err
	}

	r.repeaterManager.ProcessTransmit(packet.Source, len(response))
	return nil
}

// handleDataPacket handles YSFD (data) packets
func (r *Reflector) handleDataPacket(packet *network.Packet) error {
	r.logger.Debug("Received data packet",
		logger.String("callsign", packet.Callsign),
		logger.String("source", packet.Source.String()),
		logger.Uint32("sequence", packet.GetSequence()))

	// Ensure repeater is registered
	rep := r.repeaterManager.GetRepeater(packet.Source)
	if rep == nil {
		r.logger.Warn("Data packet from unregistered repeater",
			logger.String("callsign", packet.Callsign),
			logger.String("source", packet.Source.String()))
		return nil
	}

	// Process packet for statistics and state tracking
	r.repeaterManager.ProcessPacket(packet.Callsign, packet.Source, packet.Type, len(packet.Data))

	// Broadcast to all other repeaters
	addresses := r.repeaterManager.GetAllAddresses()
	if err := r.server.BroadcastData(packet.Data, addresses, packet.Source); err != nil {
		r.logger.Error("Failed to broadcast data packet",
			logger.String("callsign", packet.Callsign),
			logger.Error(err))
		return err
	}

	// Update transmit statistics for all recipients
	for _, addr := range addresses {
		if addr.String() != packet.Source.String() {
			r.repeaterManager.ProcessTransmit(addr, len(packet.Data))
		}
	}

	return nil
}

// handleUnlinkPacket handles YSFU (unlink) packets
func (r *Reflector) handleUnlinkPacket(packet *network.Packet) error {
	r.logger.Info("Received unlink packet",
		logger.String("callsign", packet.Callsign),
		logger.String("source", packet.Source.String()))

	// Remove repeater
	if r.repeaterManager.RemoveRepeater(packet.Source) {
		r.logger.Info("Repeater unlinked",
			logger.String("callsign", packet.Callsign),
			logger.String("source", packet.Source.String()))
	}

	return nil
}

// handleStatusPacket handles YSFS (status request) packets
func (r *Reflector) handleStatusPacket(packet *network.Packet) error {
	if !packet.IsStatusRequest() {
		return nil
	}

	r.logger.Debug("Received status request",
		logger.String("source", packet.Source.String()))

	// Create status response
	count := r.repeaterManager.Count()
	response := network.CreateStatusResponse(
		r.config.Server.Name,
		r.config.Server.Description,
		count,
	)

	// Send response
	if err := r.server.SendPacket(response, packet.Source); err != nil {
		r.logger.Error("Failed to send status response",
			logger.String("source", packet.Source.String()),
			logger.Error(err))
		return err
	}

	return nil
}

// processEvents processes repeater events
func (r *Reflector) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-r.eventChan:
			r.logger.Info("Repeater event",
				logger.String("type", event.Type),
				logger.String("callsign", event.Callsign),
				logger.String("address", event.Address),
				logger.Duration("duration", event.Duration))

			// TODO: Forward to MQTT if configured
			// TODO: Store in database if configured
		}
	}
}

// logStats periodically logs statistics
func (r *Reflector) logStats(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := r.GetStats()
			r.logger.Info("Reflector statistics",
				logger.Duration("uptime", stats.Uptime),
				logger.Int("active_repeaters", stats.ActiveRepeaters),
				logger.Uint64("total_connections", stats.TotalConnections),
				logger.Uint64("total_packets", stats.TotalPackets),
				logger.Int64("bytes_received", stats.BytesReceived),
				logger.Int64("bytes_sent", stats.BytesSent))

			// Also dump repeater details in debug mode
			if r.config.Logging.Level == "debug" {
				r.repeaterManager.DumpRepeaters()
			}
		}
	}
}