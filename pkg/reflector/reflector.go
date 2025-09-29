package reflector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/bridge"
	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
	"github.com/dbehnke/ysf-nexus/pkg/network"
	"github.com/dbehnke/ysf-nexus/pkg/repeater"
	"github.com/dbehnke/ysf-nexus/pkg/web"
)

// bridgeTalker tracks bridge talker state
type bridgeTalker struct {
	callsign     string
	bridgeName   string
	bridgeAddr   string
	startTime    time.Time
	lastSeen     time.Time
	isTalking    bool
	lastSequence uint32
}

// GetCallsign returns the callsign of the bridge talker
func (bt *bridgeTalker) GetCallsign() string {
	return bt.callsign
}

// GetBridgeName returns the name of the bridge
func (bt *bridgeTalker) GetBridgeName() string {
	return bt.bridgeName
}

// GetTalkDuration returns how long the bridge talker has been talking
func (bt *bridgeTalker) GetTalkDuration() time.Duration {
	return time.Since(bt.startTime)
}

// Reflector represents the main YSF reflector application
type Reflector struct {
	config          *config.Config
	logger          *logger.Logger
	server          *network.Server
	repeaterManager *repeater.Manager
	bridgeManager   *bridge.Manager
	webServer       *web.Server
	eventChan       chan repeater.Event
	running         bool
	mu              sync.RWMutex
	
	// Bridge talker tracking
	bridgeTalkers   map[string]*bridgeTalker // key: callsign+bridge_name
	talkersMu       sync.RWMutex
}

// New creates a new YSF reflector
func New(cfg *config.Config, log *logger.Logger) *Reflector {
	eventChan := make(chan repeater.Event, 1000)

	r := &Reflector{
		config:        cfg,
		logger:        log.WithComponent("reflector"),
		eventChan:     eventChan,
		bridgeTalkers: make(map[string]*bridgeTalker),
	}

	// Initialize network server
	r.server = network.NewServer(cfg.Server.Host, cfg.Server.Port)
	r.server.SetDebug(cfg.Logging.Level == "debug")

	// Initialize repeater manager
	r.repeaterManager = repeater.NewManagerWithLogger(
		cfg.Server.Timeout,
		cfg.Server.MaxConnections,
		eventChan,
		cfg.Server.TalkMaxDuration,
		cfg.Server.UnmuteAfter,
		r.logger,
	)

	// Initialize bridge manager
	r.bridgeManager = bridge.NewManager(cfg.Bridges, r.server, r.logger)

	// Initialize web server
	r.webServer = web.NewServer(cfg, log, r.repeaterManager, eventChan, r.bridgeManager, r)

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

	// NOTE: Event processor removed - web server handles events directly from the repeater manager
	// The reflector was consuming events from the channel, preventing the web server from receiving them

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

	// Start web server
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := r.webServer.Start(ctx); err != nil {
			r.logger.Error("Web server error", logger.Error(err))
		}
	}()

	// Start bridge manager
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := r.bridgeManager.Start(); err != nil {
			r.logger.Error("Bridge manager error", logger.Error(err))
		}
	}()

	// Start bridge talker cleanup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.cleanupBridgeTalkers(ctx)
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
	Uptime           time.Duration         `json:"uptime"`
	ActiveRepeaters  int                   `json:"active_repeaters"`
	TotalConnections uint64                `json:"total_connections"`
	TotalPackets     uint64                `json:"total_packets"`
	PacketsReceived  map[string]int64      `json:"packets_received"`
	PacketsSent      map[string]int64      `json:"packets_sent"`
	BytesReceived    int64                 `json:"bytes_received"`
	BytesSent        int64                 `json:"bytes_sent"`
	RepeaterStats    repeater.ManagerStats `json:"repeater_stats"`
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

	// Check if this packet is from a bridge connection
	r.bridgeManager.HandleIncomingPacket(packet.Data, packet.Source)

	// Don't treat bridge connections as repeaters - bridges handle their own responses
	if r.bridgeManager.IsBridgeAddress(packet.Source) {
		r.logger.Debug("Ignoring poll from bridge connection",
			logger.String("callsign", packet.Callsign),
			logger.String("source", packet.Source.String()))
		return nil
	}

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
	} else {
		// Log repeated connections for debugging OpenSpot issue
		r.logger.Debug("Existing repeater poll",
			logger.String("callsign", packet.Callsign),
			logger.String("source", packet.Source.String()),
			logger.Duration("uptime", rep.Uptime()))
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

	// Check if this packet is from a bridge connection
	r.bridgeManager.HandleIncomingPacket(packet.Data, packet.Source)

	// Handle bridge data packets differently
	if r.bridgeManager.IsBridgeAddress(packet.Source) {
		r.logger.Debug("Received data from bridge connection",
			logger.String("callsign", packet.Callsign),
			logger.String("source", packet.Source.String()))
		
		// Track bridge talker activity
		r.processBridgeTalker(packet)
		
		// Forward bridge data to all local repeaters (bridge acts as special repeater)
		addresses := r.repeaterManager.GetAllAddresses()
		if len(addresses) > 0 {
			if err := r.server.BroadcastData(packet.Data, addresses, packet.Source); err != nil {
				r.logger.Error("Failed to forward bridge data to repeaters",
					logger.String("callsign", packet.Callsign),
					logger.Error(err))
				return err
			}
			
			r.logger.Debug("Forwarded bridge data to local repeaters",
				logger.String("callsign", packet.Callsign),
				logger.Int("repeaters", len(addresses)))
			
			// Update transmit statistics for all recipients
			for _, addr := range addresses {
				r.repeaterManager.ProcessTransmit(addr, len(packet.Data))
			}
		}
		return nil
	}

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

	// Forward local repeater traffic to all bridges (bidirectional bridge forwarding)
	r.forwardToBridges(packet.Data, packet.Callsign)

	return nil
}

// processBridgeTalker tracks bridge talker state and sends events
func (r *Reflector) processBridgeTalker(packet *network.Packet) {
	if packet.Callsign == "" {
		return
	}
	
	// Get bridge name for this address
	bridgeName := r.getBridgeNameByAddress(packet.Source.String())
	if bridgeName == "" {
		bridgeName = "bridge:" + packet.Source.String()
	}
	
	talkerKey := packet.Callsign + ":" + bridgeName
	sequence := packet.GetSequence()
	now := time.Now()
	
	r.talkersMu.Lock()
	defer r.talkersMu.Unlock()
	
	talker, exists := r.bridgeTalkers[talkerKey]
	
	if !exists {
		// New talker
		talker = &bridgeTalker{
			callsign:     packet.Callsign,
			bridgeName:   bridgeName,
			bridgeAddr:   packet.Source.String(),
			startTime:    now,
			lastSeen:     now,
			isTalking:    true,
			lastSequence: sequence,
		}
		r.bridgeTalkers[talkerKey] = talker
		
		// Send talk start event
		r.sendBridgeEvent(repeater.EventTalkStart, packet.Callsign, bridgeName, 0)
		
		r.logger.Info("Bridge talker started",
			logger.String("callsign", packet.Callsign),
			logger.String("bridge", bridgeName))
	} else {
		// Existing talker - update last seen
		talker.lastSeen = now
		talker.lastSequence = sequence
	}
}

// getBridgeNameByAddress returns the bridge name for the given address
func (r *Reflector) getBridgeNameByAddress(addr string) string {
	// Get all bridge statuses and find matching address
	bridgeStatuses := r.bridgeManager.GetStatus()
	for bridgeName := range bridgeStatuses {
		bridge := r.bridgeManager.GetBridge(bridgeName)
		if bridge != nil {
			remoteAddr := bridge.GetRemoteAddr()
			if remoteAddr != nil && remoteAddr.String() == addr {
				return bridgeName
			}
		}
	}
	return ""
}

// sendBridgeEvent sends an event to the event channel for bridge activities
func (r *Reflector) sendBridgeEvent(eventType, callsign, bridgeIdentifier string, duration time.Duration) {
	if r.eventChan == nil {
		return
	}

	event := repeater.Event{
		Type:      eventType,
		Callsign:  callsign,
		Address:   bridgeIdentifier, // Use bridge name/identifier as address
		Timestamp: time.Now(),
		Duration:  duration,
	}

	select {
	case r.eventChan <- event:
	default:
		// Don't block if event channel is full
		r.logger.Warn("Event channel full, dropping bridge event", 
			logger.String("event_type", eventType),
			logger.String("callsign", callsign))
	}
}

// cleanupBridgeTalkers periodically checks for inactive bridge talkers
func (r *Reflector) cleanupBridgeTalkers(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.checkBridgeTalkerTimeouts()
		}
	}
}

// checkBridgeTalkerTimeouts checks for bridge talkers that have timed out
func (r *Reflector) checkBridgeTalkerTimeouts() {
	now := time.Now()
	talkTimeout := 10 * time.Second // Consider talkers inactive after 10 seconds of no packets
	
	r.talkersMu.Lock()
	defer r.talkersMu.Unlock()
	
	for key, talker := range r.bridgeTalkers {
		if talker.isTalking && now.Sub(talker.lastSeen) > talkTimeout {
			// Talker has timed out
			duration := now.Sub(talker.startTime)
			talker.isTalking = false
			
			// Send talk end event
			r.sendBridgeEvent(repeater.EventTalkEnd, talker.callsign, talker.bridgeName, duration)
			
			r.logger.Info("Bridge talker ended",
				logger.String("callsign", talker.callsign),
				logger.String("bridge", talker.bridgeName),
				logger.Duration("duration", duration))
			
			// Remove from active talkers
			delete(r.bridgeTalkers, key)
		}
	}
}

// GetCurrentBridgeTalker returns the currently active bridge talker, if any
func (r *Reflector) GetCurrentBridgeTalker() interface{} {
	r.talkersMu.RLock()
	defer r.talkersMu.RUnlock()
	
	// Find the first active bridge talker
	for _, talker := range r.bridgeTalkers {
		if talker.isTalking {
			return talker
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
		// Log non-status packets that come through this handler for debugging
		r.logger.Debug("Received non-status packet in status handler",
			logger.String("source", packet.Source.String()),
			logger.String("type", packet.Type),
			logger.Int("size", len(packet.Data)))
		return nil
	}

	r.logger.Info("Received status request",
		logger.String("source", packet.Source.String()),
		logger.String("callsign", packet.Callsign))

	// Create status response
	count := r.repeaterManager.Count()
	response := network.CreateStatusResponse(
		r.config.Server.Name,
		r.config.Server.Description,
		count,
	)

	r.logger.Debug("Sending status response",
		logger.String("source", packet.Source.String()),
		logger.String("name", r.config.Server.Name),
		logger.String("description", r.config.Server.Description),
		logger.Int("count", count),
		logger.Int("response_size", len(response)))

	// Send response
	if err := r.server.SendPacket(response, packet.Source); err != nil {
		r.logger.Error("Failed to send status response",
			logger.String("source", packet.Source.String()),
			logger.Error(err))
		return err
	}

	r.logger.Info("Status response sent successfully",
		logger.String("source", packet.Source.String()))

	return nil
}

// processEvents was removed because events are forwarded/consumed elsewhere. If needed,
// reintroduce with careful event channel ownership semantics.

// forwardToBridges forwards local repeater traffic to all connected bridges
func (r *Reflector) forwardToBridges(data []byte, callsign string) {
	// Get bridge addresses from bridge manager
	bridgeAddresses := r.bridgeManager.GetConnectedAddresses()
	
	if len(bridgeAddresses) > 0 {
		// Forward data to all connected bridges
		for _, addr := range bridgeAddresses {
			if err := r.server.SendPacket(data, addr); err != nil {
				r.logger.Error("Failed to forward data to bridge",
					logger.String("callsign", callsign),
					logger.String("bridge", addr.String()),
					logger.Error(err))
			} else {
				r.logger.Debug("Forwarded local data to bridge",
					logger.String("callsign", callsign),
					logger.String("bridge", addr.String()))
			}
		}
		
		r.logger.Debug("Forwarded local repeater traffic to bridges",
			logger.String("callsign", callsign),
			logger.Int("bridges", len(bridgeAddresses)))
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
