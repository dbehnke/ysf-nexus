package ysf2dmr

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/codec"
	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/dmr"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
	"github.com/dbehnke/ysf-nexus/pkg/network"
)

// CallDirection indicates the direction of a cross-mode call
type CallDirection int

const (
	DirectionNone CallDirection = iota
	DirectionYSFToDMR
	DirectionDMRToYSF
)

func (d CallDirection) String() string {
	switch d {
	case DirectionYSFToDMR:
		return "YSF→DMR"
	case DirectionDMRToYSF:
		return "DMR→YSF"
	default:
		return "None"
	}
}

// CallState tracks an active cross-mode call
type CallState struct {
	Direction    CallDirection
	YSFCallsign  string
	DMRID        uint32
	DMRCallsign  string
	TalkGroup    uint32
	Slot         uint8
	StartTime    time.Time
	LastActivity time.Time
	FrameCount   uint32
	StreamID     uint32
}

// Bridge coordinates YSF ↔ DMR cross-mode communication
type Bridge struct {
	config config.YSF2DMRConfig
	logger *logger.Logger

	// Network components
	ysfServer     *network.Server
	ownsYSFServer bool
	// function to obtain current repeater UDP addresses (provided by reflector)
	repeaterAddrProvider func() []*net.UDPAddr
	dmrNetwork           *dmr.Network
	lookup               *dmr.Lookup
	converter            *codec.Converter

	// State management
	mu         sync.RWMutex
	activeCall *CallState
	running    bool

	// Statistics
	stats Statistics
}

// Statistics tracks bridge performance
type Statistics struct {
	mu            sync.RWMutex
	TotalCalls    uint64
	YSFToDMRCalls uint64
	DMRToYSFCalls uint64
	YSFPackets    uint64
	DMRPackets    uint64
	FramesDropped uint64
	LastCallTime  time.Time
}

// NewBridge creates a new YSF2DMR bridge
// NewBridge creates a new YSF2DMR bridge. If srv is non-nil the bridge will use
// the provided YSF server (shared with the reflector) instead of creating its
// own local server. Pass nil to let the bridge create and manage its own server.
// NewBridge creates a new YSF2DMR bridge. If srv is non-nil the bridge will use
// the provided YSF server (shared with the reflector) instead of creating its
// own local server. Pass nil to let the bridge create and manage its own server.
// The repeaterAddrProvider is an optional function that returns the current
// list of repeater UDP addresses; when provided the bridge will use it to
// broadcast converted YSF packets to local repeaters.
func NewBridgeWithServer(cfg config.YSF2DMRConfig, log *logger.Logger, srv *network.Server, repeaterAddrProvider func() []*net.UDPAddr) (*Bridge, error) {
	b := &Bridge{
		config:               cfg,
		logger:               log.WithComponent("ysf2dmr"),
		converter:            codec.NewConverter(),
		repeaterAddrProvider: repeaterAddrProvider,
	}

	if srv != nil {
		b.ysfServer = srv
		b.ownsYSFServer = false
	} else {
		b.ysfServer = nil
		b.ownsYSFServer = true
	}

	return b, nil
}

// Backwards-compatible constructor used by tests and adapters that don't provide
// a shared YSF server. It simply calls NewBridgeWithServer with nil server and
// provider.
func NewBridge(cfg config.YSF2DMRConfig, log *logger.Logger) (*Bridge, error) {
	return NewBridgeWithServer(cfg, log, nil, nil)
}

// Start starts the YSF2DMR bridge
func (b *Bridge) Start(ctx context.Context) error {
	b.logger.Info("Starting YSF2DMR bridge")

	// Initialize DMR ID lookup
	if b.config.Lookup.Enabled {
		lookupConfig := dmr.LookupConfig{
			FilePath:        b.config.Lookup.DMRIDFile,
			DownloadURL:     b.config.Lookup.DownloadURL,
			AutoDownload:    b.config.Lookup.AutoDownload,
			RefreshInterval: b.config.Lookup.RefreshInterval,
		}

		lookup, err := dmr.NewLookup(lookupConfig, b.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize DMR lookup: %w", err)
		}
		b.lookup = lookup

		// Start auto-refresh if configured
		if b.config.Lookup.RefreshInterval > 0 {
			stopChan := make(chan struct{})
			go b.lookup.StartAutoRefresh(b.config.Lookup.RefreshInterval, stopChan)
		}
	}

	// Initialize YSF server (for receiving from YSF side) only if we don't have one
	b.logger.Info("YSF server initialization check",
		logger.Any("server_nil", b.ysfServer == nil),
		logger.Any("owns_server", b.ownsYSFServer))

	if b.ysfServer == nil {
		b.logger.Warn("Creating NEW YSF server for bridge - this may not receive reflector traffic!")
		b.ysfServer = network.NewServer(
			b.config.YSF.LocalAddress,
			b.config.YSF.LocalPort,
		)

		// Start YSF server in background so we can initialize DMR network in the same Start call.
		errCh := make(chan error, 1)
		go func() {
			errCh <- b.ysfServer.Start(ctx)
		}()

		// Wait briefly for an immediate startup error (e.g. bind failure). If none, assume server started.
		select {
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("failed to start YSF server: %w", err)
			}
		case <-time.After(250 * time.Millisecond):
			// likely started successfully; server will log its listening address
		}

		b.logger.Info("YSF server started",
			logger.String("address", b.config.YSF.LocalAddress),
			logger.Int("port", b.config.YSF.LocalPort))
	} else {
		b.logger.Info("Using shared YSF server - can broadcast to reflector repeaters",
			logger.String("address", b.config.YSF.LocalAddress),
			logger.Int("port", b.config.YSF.LocalPort))
	}

	// Initialize DMR network connection
	if b.config.DMR.Enabled {
		// Debug: log the effective DMR config we will use (mask password)
		maskedPwd := ""
		if b.config.DMR.Password != "" {
			maskedPwd = "***"
		}
		b.logger.Debug("Preparing DMR network with config",
			logger.String("address", b.config.DMR.Address),
			logger.Int("port", b.config.DMR.Port),
			logger.Uint32("id", b.config.DMR.ID),
			logger.String("password", maskedPwd),
			logger.Uint32("startup_tg", b.config.DMR.StartupTG))

		// Also log at info level so we always see the DMR startup attempt in logs
		b.logger.Info("YSF2DMR: DMR network enabled",
			logger.String("address", b.config.DMR.Address),
			logger.Int("port", b.config.DMR.Port),
			logger.Uint32("id", b.config.DMR.ID),
			logger.Uint32("startup_tg", b.config.DMR.StartupTG))

		dmrConfig := dmr.Config{
			Address:    b.config.DMR.Address,
			Port:       b.config.DMR.Port,
			RepeaterID: b.config.DMR.ID,
			Password:   b.config.DMR.Password,
			// Use explicit DMR.callsign if configured, otherwise use YSF callsign
			Callsign: func() string {
				if b.config.DMR.Callsign != "" {
					return b.config.DMR.Callsign
				}
				return b.config.YSF.Callsign
			}(),
			RXFreq:       b.config.DMR.RXFreq,
			TXFreq:       b.config.DMR.TXFreq,
			TXPower:      b.config.DMR.TXPower,
			ColorCode:    b.config.DMR.ColorCode,
			Latitude:     b.config.DMR.Latitude,
			Longitude:    b.config.DMR.Longitude,
			Height:       b.config.DMR.Height,
			Location:     b.config.DMR.Location,
			Description:  b.config.DMR.Description,
			URL:          b.config.DMR.URL,
			Slot:         b.config.DMR.Slot,
			TalkGroup:    b.config.DMR.StartupTG,
			PingInterval: b.config.DMR.PingInterval,
			AuthTimeout:  b.config.DMR.AuthTimeout,
			// Pass through safe RPTC flag
			// Note: All RPTC fields are sent from config values
			// ...
		}

		network := dmr.NewNetwork(dmrConfig, b.logger)
		if err := network.Start(ctx); err != nil {
			return fmt.Errorf("failed to start DMR network: %w", err)
		}
		b.dmrNetwork = network

		b.logger.Info("DMR network connected",
			logger.String("network", b.config.DMR.Network),
			logger.Uint32("id", b.config.DMR.ID))
	}

	b.mu.Lock()
	b.running = true
	b.mu.Unlock()

	// Register YSF packet handler
	b.ysfServer.RegisterHandler("YSFD", func(packet *network.Packet) error {
		return b.HandleYSFPacket(packet)
	})

	// Start DMR packet handler if network is enabled
	if b.dmrNetwork != nil {
		go b.dmrPacketHandler(ctx)
	}

	b.logger.Info("YSF2DMR bridge started successfully")
	return nil
}

// Stop stops the YSF2DMR bridge
func (b *Bridge) Stop() error {
	b.logger.Info("Stopping YSF2DMR bridge")

	b.mu.Lock()
	b.running = false
	b.mu.Unlock()

	// End any active call
	if b.activeCall != nil {
		b.endCall()
	}

	// Stop DMR network
	if b.dmrNetwork != nil {
		if err := b.dmrNetwork.Stop(); err != nil {
			b.logger.Warn("Error stopping DMR network", logger.Error(err))
		}
	}

	// Stop YSF server
	if b.ysfServer != nil {
		if b.ownsYSFServer {
			if err := b.ysfServer.Stop(); err != nil {
				b.logger.Warn("Error stopping YSF server", logger.Error(err))
			}
		} else {
			b.logger.Debug("Shared YSF server not stopped by bridge")
		}
	}

	b.logger.Info("YSF2DMR bridge stopped")
	return nil
}

// HandleYSFPacket processes a YSF packet and forwards to DMR
// This is now exported so the DMRBridgeAdapter can forward packets to it
func (b *Bridge) HandleYSFPacket(packet *network.Packet) error {
	// Only process YSFD (voice data) packets
	if packet.Type != "YSFD" {
		return nil
	}

	b.stats.mu.Lock()
	b.stats.YSFPackets++
	b.stats.mu.Unlock()

	// Extract callsign from YSF packet
	ysfCallsign := extractYSFCallsign(packet.Data)
	if ysfCallsign == "" {
		return fmt.Errorf("no callsign in YSF packet")
	}

	// Lookup DMR ID
	var dmrID uint32
	if b.lookup != nil {
		var ok bool
		dmrID, ok = b.lookup.GetDMRID(ysfCallsign)
		if !ok {
			b.logger.Debug("No DMR ID found for callsign",
				logger.String("callsign", ysfCallsign))
			// Use bridge's DMR ID as fallback
			dmrID = b.config.DMR.ID
		}
	} else {
		dmrID = b.config.DMR.ID
	}

	// Check if this is a new call or continuation
	b.mu.Lock()
	if b.activeCall == nil {
		// Start new YSF→DMR call
		b.activeCall = &CallState{
			Direction:    DirectionYSFToDMR,
			YSFCallsign:  ysfCallsign,
			DMRID:        dmrID,
			TalkGroup:    b.config.DMR.StartupTG,
			Slot:         b.config.DMR.Slot,
			StartTime:    time.Now(),
			LastActivity: time.Now(),
			StreamID:     b.dmrNetwork.GetStreamID(),
		}

		b.logger.Info("Starting YSF→DMR call",
			logger.String("callsign", ysfCallsign),
			logger.Uint32("dmr_id", dmrID),
			logger.Uint32("talkgroup", b.config.DMR.StartupTG))

		// Send DMR voice header
		if err := b.dmrNetwork.SendVoiceHeader(
			dmrID,
			b.config.DMR.StartupTG,
			b.config.DMR.Slot,
			dmr.CallTypeGroup,
			b.activeCall.StreamID,
		); err != nil {
			b.mu.Unlock()
			return fmt.Errorf("failed to send DMR voice header: %w", err)
		}

		b.stats.mu.Lock()
		b.stats.TotalCalls++
		b.stats.YSFToDMRCalls++
		b.stats.mu.Unlock()
	} else if b.activeCall.Direction == DirectionYSFToDMR {
		// Continue existing call
		b.activeCall.LastActivity = time.Now()
		b.activeCall.FrameCount++
	} else {
		// Different direction call in progress, drop packet
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()

	// Convert YSF to DMR
	dmrVoice, err := b.converter.YSFToDMR(packet.Data)
	if err != nil {
		return fmt.Errorf("YSF→DMR conversion failed: %w", err)
	}

	// Send DMR voice data
	if err := b.dmrNetwork.SendVoiceData(
		dmrID,
		b.config.DMR.StartupTG,
		b.config.DMR.Slot,
		dmr.CallTypeGroup,
		b.activeCall.StreamID,
		uint8(b.activeCall.FrameCount%256),
		dmrVoice,
	); err != nil {
		return fmt.Errorf("failed to send DMR voice: %w", err)
	}

	b.stats.mu.Lock()
	b.stats.DMRPackets++
	b.stats.mu.Unlock()

	// Check for call timeout
	b.checkCallTimeout()

	return nil
}

// dmrPacketHandler handles incoming DMR packets (DMR → YSF direction)
func (b *Bridge) dmrPacketHandler(ctx context.Context) {
	b.logger.Info("DMR packet handler started")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Receive DMR packet
			packet, err := b.dmrNetwork.ReceivePacket(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				b.logger.Debug("DMR receive error", logger.Error(err))
				continue
			}

			// Process packet
			if err := b.handleDMRPacket(packet); err != nil {
				b.logger.Debug("DMR packet handling error", logger.Error(err))
			}
		}
	}
}

// handleDMRPacket processes a DMR packet and forwards to YSF
func (b *Bridge) handleDMRPacket(packet *dmr.Packet) error {
	// Only process DMRD (voice data) packets
	if packet.Type != dmr.PacketTypeDMRD {
		return nil
	}

	b.stats.mu.Lock()
	b.stats.DMRPackets++
	b.stats.mu.Unlock()

	// Parse DMRD packet
	dmrdPacket, err := dmr.ParseDMRDPacket(packet.Data)
	if err != nil {
		return fmt.Errorf("failed to parse DMRD: %w", err)
	}

	// Filter for our talkgroup and slot
	if dmrdPacket.DstID != b.config.DMR.StartupTG || dmrdPacket.Slot != b.config.DMR.Slot {
		return nil
	}

	// Lookup callsign for DMR ID
	var dmrCallsign string
	if b.lookup != nil {
		dmrCallsign = b.lookup.GetCallsign(dmrdPacket.SrcID)
	}
	if dmrCallsign == "" {
		dmrCallsign = fmt.Sprintf("DMR-%d", dmrdPacket.SrcID)
	}

	// Check if this is a new call or continuation
	b.mu.Lock()
	if b.activeCall == nil {
		// Start new DMR→YSF call
		b.activeCall = &CallState{
			Direction:    DirectionDMRToYSF,
			DMRID:        dmrdPacket.SrcID,
			DMRCallsign:  dmrCallsign,
			TalkGroup:    dmrdPacket.DstID,
			Slot:         dmrdPacket.Slot,
			StartTime:    time.Now(),
			LastActivity: time.Now(),
			StreamID:     dmrdPacket.StreamID,
		}

		b.logger.Info("Starting DMR→YSF call",
			logger.String("callsign", dmrCallsign),
			logger.Uint32("dmr_id", dmrdPacket.SrcID),
			logger.Uint32("talkgroup", dmrdPacket.DstID))

		// Set converter metadata for packet generation
		b.converter.SetMetadata(dmrCallsign, dmrdPacket.SrcID)

		b.stats.mu.Lock()
		b.stats.TotalCalls++
		b.stats.DMRToYSFCalls++
		b.stats.mu.Unlock()
	} else if b.activeCall.Direction == DirectionDMRToYSF {
		// Continue existing call
		b.activeCall.LastActivity = time.Now()
		b.activeCall.FrameCount++
	} else {
		// Different direction call in progress, drop packet
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()

	// Convert DMR to YSF
	ysfPacket, err := b.converter.DMRToYSF(dmrdPacket.Data)
	if err != nil {
		// Might be buffering, not an error
		return nil
	}

	if ysfPacket != nil {
		// Send to YSF reflector via shared server and repeater addresses
		b.logger.Info("Generated YSF packet from DMR",
			logger.Int("size", len(ysfPacket)),
			logger.String("header", fmt.Sprintf("%X", ysfPacket[0:35])),
			logger.String("callsign", string(ysfPacket[4:14])))

		// Broadcast to repeaters if we have a provider and server
		if b.repeaterAddrProvider != nil && b.ysfServer != nil {
			addrs := b.repeaterAddrProvider()
			if len(addrs) > 0 {
				// Log detailed information about what we're sending
				addrStrings := make([]string, len(addrs))
				for i, addr := range addrs {
					addrStrings[i] = addr.String()
				}

				b.logger.Info("Broadcasting DMR→YSF converted packet",
					logger.Int("repeaters", len(addrs)),
					logger.Int("size", len(ysfPacket)),
					logger.Any("repeater_addresses", addrStrings),
					logger.Any("server_instance", fmt.Sprintf("%p", b.ysfServer)))

				if err := b.ysfServer.BroadcastData(ysfPacket, addrs, nil); err != nil {
					b.logger.Error("Failed to broadcast DMR→YSF packet", logger.Error(err))
				} else {
					b.logger.Info("DMR→YSF broadcast completed successfully")
				}
			} else {
				b.logger.Warn("No YSF repeaters connected - DMR→YSF packet not sent")
			}
		} else {
			// More explicit debug info for missing components
			hasProvider := b.repeaterAddrProvider != nil
			hasServer := b.ysfServer != nil
			b.logger.Warn("Cannot broadcast DMR→YSF packet - missing components",
				logger.Any("has_provider", hasProvider),
				logger.Any("has_server", hasServer))
		}

		b.stats.mu.Lock()
		b.stats.YSFPackets++
		b.stats.mu.Unlock()
	}

	// Check for call timeout
	b.checkCallTimeout()

	return nil
}

// checkCallTimeout checks if the active call has timed out
func (b *Bridge) checkCallTimeout() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.activeCall == nil {
		return
	}

	// End call if no activity for hang time
	if time.Since(b.activeCall.LastActivity) > b.config.YSF.HangTime {
		b.logger.Info("Call ended (timeout)",
			logger.String("direction", b.activeCall.Direction.String()),
			logger.Duration("duration", time.Since(b.activeCall.StartTime)))

		b.endCallLocked()
	}
}

// endCall ends the active call (externally callable)
func (b *Bridge) endCall() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.endCallLocked()
}

// endCallLocked ends the active call (must hold lock)
func (b *Bridge) endCallLocked() {
	if b.activeCall == nil {
		return
	}

	// Send terminator based on direction
	if b.activeCall.Direction == DirectionYSFToDMR && b.dmrNetwork != nil {
		if err := b.dmrNetwork.SendVoiceTerminator(
			b.activeCall.DMRID,
			b.activeCall.TalkGroup,
			b.activeCall.Slot,
			dmr.CallTypeGroup,
			b.activeCall.StreamID,
			uint8(b.activeCall.FrameCount%256),
		); err != nil {
			b.logger.Warn("Failed to send voice terminator", logger.Error(err))
		}
	}

	b.stats.mu.Lock()
	b.stats.LastCallTime = time.Now()
	b.stats.mu.Unlock()

	b.activeCall = nil
	b.converter.Reset()
}

// snapshotStats returns a copy of statistics without copying the internal mutex
func (b *Bridge) snapshotStats() Statistics {
	b.stats.mu.RLock()
	defer b.stats.mu.RUnlock()

	return Statistics{
		TotalCalls:    b.stats.TotalCalls,
		YSFToDMRCalls: b.stats.YSFToDMRCalls,
		DMRToYSFCalls: b.stats.DMRToYSFCalls,
		YSFPackets:    b.stats.YSFPackets,
		DMRPackets:    b.stats.DMRPackets,
		FramesDropped: b.stats.FramesDropped,
		LastCallTime:  b.stats.LastCallTime,
	}
}

// GetStatistics returns bridge statistics
func (b *Bridge) GetStatistics() Statistics {
	return b.snapshotStats()
}

// GetActiveCall returns the current active call state
func (b *Bridge) GetActiveCall() *CallState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.activeCall == nil {
		return nil
	}

	// Return a copy
	call := *b.activeCall
	return &call
}

// IsRunning returns whether the bridge is running
func (b *Bridge) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

// BridgeStatus represents the current status of the YSF2DMR bridge
type BridgeStatus struct {
	Enabled      bool        `json:"enabled"`
	DMRConnected bool        `json:"dmr_connected"`
	YSFListening bool        `json:"ysf_listening"`
	ActiveCall   *CallState  `json:"active_call,omitempty"`
	Stats        *Statistics `json:"stats"`
	DMRNetwork   string      `json:"dmr_network,omitempty"`
	DMRID        uint32      `json:"dmr_id,omitempty"`
	TalkGroup    uint32      `json:"talk_group,omitempty"`
	YSFCallsign  string      `json:"ysf_callsign,omitempty"`
}

// GetStatus returns the current bridge status for the web dashboard
func (b *Bridge) GetStatus() BridgeStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stats := b.snapshotStats()
	status := BridgeStatus{
		Enabled:      b.running,
		DMRConnected: b.dmrNetwork != nil,
		YSFListening: b.ysfServer != nil,
		Stats:        &stats,
	}

	// Add DMR network info if connected
	if b.dmrNetwork != nil && b.config.DMR.Enabled {
		status.DMRNetwork = b.config.DMR.Network
		status.DMRID = b.config.DMR.ID
		status.TalkGroup = b.config.DMR.StartupTG
	}

	// Add YSF info
	if b.config.YSF.Callsign != "" {
		status.YSFCallsign = b.config.YSF.Callsign
	}

	// Add active call if present
	if b.activeCall != nil {
		callCopy := *b.activeCall
		status.ActiveCall = &callCopy
	}

	return status
}

// Helper functions

// extractYSFCallsign extracts the callsign from a YSFD packet
func extractYSFCallsign(data []byte) string {
	if len(data) < 14 {
		return ""
	}

	// Callsign is at bytes 4-13 (10 bytes)
	callsign := string(data[4:14])
	// Trim spaces and null bytes
	for i, c := range callsign {
		if c == 0 || c == ' ' {
			callsign = callsign[:i]
			break
		}
	}

	return callsign
}

// packetSource represents where a packet came from
// packetSource type was removed as it was not used.
