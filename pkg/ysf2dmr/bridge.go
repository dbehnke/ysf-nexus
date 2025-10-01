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
	Direction     CallDirection
	YSFCallsign   string
	DMRID         uint32
	DMRCallsign   string
	TalkGroup     uint32
	Slot          uint8
	StartTime     time.Time
	LastActivity  time.Time
	FrameCount    uint32
	StreamID      uint32
}

// Bridge coordinates YSF ↔ DMR cross-mode communication
type Bridge struct {
	config config.YSF2DMRConfig
	logger *logger.Logger

	// Network components
	ysfServer  *network.Server
	dmrNetwork *dmr.Network
	lookup     *dmr.Lookup
	converter  *codec.Converter

	// State management
	mu          sync.RWMutex
	activeCall  *CallState
	running     bool

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
func NewBridge(cfg config.YSF2DMRConfig, log *logger.Logger) (*Bridge, error) {
	b := &Bridge{
		config:    cfg,
		logger:    log.WithComponent("ysf2dmr"),
		converter: codec.NewConverter(),
	}

	return b, nil
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

	// Initialize YSF server (for receiving from YSF side)
	b.ysfServer = network.NewServer(
		b.config.YSF.LocalAddress,
		b.config.YSF.LocalPort,
	)

	if err := b.ysfServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start YSF server: %w", err)
	}

	b.logger.Info("YSF server started",
		logger.String("address", b.config.YSF.LocalAddress),
		logger.Int("port", b.config.YSF.LocalPort))

	// Initialize DMR network connection
	if b.config.DMR.Enabled {
		dmrConfig := dmr.Config{
			Address:    b.config.DMR.Address,
			Port:       b.config.DMR.Port,
			RepeaterID: b.config.DMR.ID,
			Password:   b.config.DMR.Password,
			Callsign:   b.config.YSF.Callsign,
			RXFreq:     b.config.DMR.RXFreq,
			TXFreq:     b.config.DMR.TXFreq,
			TXPower:    b.config.DMR.TXPower,
			ColorCode:  b.config.DMR.ColorCode,
			Latitude:   b.config.DMR.Latitude,
			Longitude:  b.config.DMR.Longitude,
			Height:     b.config.DMR.Height,
			Location:   b.config.DMR.Location,
			Description: b.config.DMR.Description,
			URL:        b.config.DMR.URL,
			Slot:       b.config.DMR.Slot,
			TalkGroup:  b.config.DMR.StartupTG,
			PingInterval: b.config.DMR.PingInterval,
			AuthTimeout:  b.config.DMR.AuthTimeout,
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
		return b.handleYSFPacket(packet)
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
		if err := b.ysfServer.Stop(); err != nil {
			b.logger.Warn("Error stopping YSF server", logger.Error(err))
		}
	}

	b.logger.Info("YSF2DMR bridge stopped")
	return nil
}

// handleYSFPacket processes a YSF packet and forwards to DMR
func (b *Bridge) handleYSFPacket(packet *network.Packet) error {
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
		// Send to YSF reflector
		// TODO: Integrate with main reflector to broadcast
		b.logger.Debug("Generated YSF packet",
			logger.Int("size", len(ysfPacket)))

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
		b.dmrNetwork.SendVoiceTerminator(
			b.activeCall.DMRID,
			b.activeCall.TalkGroup,
			b.activeCall.Slot,
			dmr.CallTypeGroup,
			b.activeCall.StreamID,
			uint8(b.activeCall.FrameCount%256),
		)
	}

	b.stats.mu.Lock()
	b.stats.LastCallTime = time.Now()
	b.stats.mu.Unlock()

	b.activeCall = nil
	b.converter.Reset()
}

// GetStatistics returns bridge statistics
func (b *Bridge) GetStatistics() Statistics {
	b.stats.mu.RLock()
	defer b.stats.mu.RUnlock()

	return b.stats
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
type packetSource struct {
	addr *net.UDPAddr
}
