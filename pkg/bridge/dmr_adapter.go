package bridge

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
	"github.com/dbehnke/ysf-nexus/pkg/network"
	"github.com/dbehnke/ysf-nexus/pkg/ysf2dmr"
)

// DMRBridgeAdapter adapts ysf2dmr.Bridge to work with the bridge manager
type DMRBridgeAdapter struct {
	config               config.BridgeConfig
	logger               *logger.Logger
	bridge               *ysf2dmr.Bridge
	server               *network.Server
	repeaterAddrProvider func() []*net.UDPAddr

	// Connection state
	mu             sync.RWMutex
	state          BridgeState
	connectedAt    *time.Time
	disconnectedAt *time.Time
	nextSchedule   *time.Time
	lastError      string
	retryCount     int
	maxRetries     int
	baseRetryDelay time.Duration
}

// NewDMRBridgeAdapter creates a new DMR bridge adapter
// NewDMRBridgeAdapter creates a new DMR bridge adapter. server and repeaterAddrProvider
// are optional; if provided the underlying ysf2dmr bridge will be constructed with
// the shared server so converted packets can be broadcast.
func NewDMRBridgeAdapter(cfg config.BridgeConfig, logger *logger.Logger, server *network.Server, repeaterAddrProvider func() []*net.UDPAddr) (*DMRBridgeAdapter, error) {
	if cfg.DMR == nil {
		return nil, fmt.Errorf("DMR configuration is required for DMR bridge")
	}

	// Convert BridgeConfig to YSF2DMRConfig
	ysf2dmrConfig := convertToYSF2DMRConfig(cfg)

	// Create the underlying ysf2dmr bridge; prefer constructor with server/provider when available
	var ysf2dmrBridge *ysf2dmr.Bridge
	var err error
	if server != nil && repeaterAddrProvider != nil {
		ysf2dmrBridge, err = ysf2dmr.NewBridgeWithServer(ysf2dmrConfig, logger, server, repeaterAddrProvider)
	} else {
		ysf2dmrBridge, err = ysf2dmr.NewBridge(ysf2dmrConfig, logger)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create ysf2dmr bridge: %w", err)
	}

	maxRetries := cfg.MaxRetries
	retryDelay := cfg.RetryDelay
	if retryDelay == 0 {
		retryDelay = 30 * time.Second
	}

	return &DMRBridgeAdapter{
		config:               cfg,
		logger:               logger,
		bridge:               ysf2dmrBridge,
		server:               server,
		repeaterAddrProvider: repeaterAddrProvider,
		state:                StateDisconnected,
		maxRetries:           maxRetries,
		baseRetryDelay:       retryDelay,
	}, nil
}

// convertToYSF2DMRConfig converts BridgeConfig with DMR settings to YSF2DMRConfig
func convertToYSF2DMRConfig(cfg config.BridgeConfig) config.YSF2DMRConfig {
	dmr := cfg.DMR

	// Determine which callsign to use for DMR: prefer explicit DMR.callsign, fall back to bridge name
	callsign := cfg.Name
	if dmr.Callsign != "" {
		callsign = dmr.Callsign
	}

	return config.YSF2DMRConfig{
		Enabled: true,
		YSF: config.YSF2DMRYSFConfig{
			Callsign:     cfg.Name, // Use bridge name as callsign
			LocalAddress: "0.0.0.0",
			LocalPort:    0, // Will be dynamically assigned
			HangTime:     5 * time.Second,
		},
		DMR: config.YSF2DMRDMRConfig{
			Enabled:           true,
			Callsign:          callsign,
			ID:                dmr.ID,
			Network:           dmr.Network,
			Address:           dmr.Address,
			Port:              dmr.Port,
			Password:          dmr.Password,
			StartupTG:         dmr.TalkGroup,
			Slot:              dmr.Slot,
			ColorCode:         dmr.ColorCode,
			EnablePrivateCall: dmr.EnablePrivateCall,
			RXFreq:            dmr.RXFreq,
			TXFreq:            dmr.TXFreq,
			TXPower:           dmr.TXPower,
			Latitude:          dmr.Latitude,
			Longitude:         dmr.Longitude,
			Height:            dmr.Height,
			Location:          dmr.Location,
			Description:       dmr.Description,
			URL:               dmr.URL,
			PingInterval:      dmr.PingInterval,
			AuthTimeout:       dmr.AuthTimeout,
		},
		Lookup: config.DMRLookupConfig{
			Enabled: false, // Disabled for individual bridges
		},
		Audio: config.AudioConfig{
			Gain: 1.0,
		},
	}
}

// RunPermanent runs a permanent DMR bridge connection with auto-reconnection
func (a *DMRBridgeAdapter) RunPermanent(ctx context.Context) {
	a.logger.Info("Starting permanent DMR bridge", logger.String("name", a.config.Name))
	a.setState(StateScheduled)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Permanent DMR bridge context cancelled", logger.String("name", a.config.Name))
			a.disconnect()
			return
		default:
			if err := a.connect(ctx); err != nil {
				a.logger.Error("Failed to connect permanent DMR bridge",
					logger.String("name", a.config.Name),
					logger.Error(err))
				a.handleConnectionFailure()

				// Wait before retry with exponential backoff
				delay := a.calculateRetryDelay()
				a.logger.Info("Retrying permanent DMR bridge connection",
					logger.String("name", a.config.Name),
					logger.Duration("delay", delay),
					logger.Int("attempt", a.retryCount+1))

				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
					continue
				}
			} else {
				// Connected successfully, reset retry count
				a.retryCount = 0
				a.maintainConnection(ctx)
			}
		}
	}
}

// RunScheduled runs a DMR bridge for a scheduled duration
func (a *DMRBridgeAdapter) RunScheduled(ctx context.Context, duration time.Duration) {
	a.logger.Info("Starting scheduled DMR bridge",
		logger.String("name", a.config.Name),
		logger.Duration("duration", duration))

	// Create a timeout context for the scheduled duration
	timeoutCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	a.setState(StateScheduled)

	if err := a.connect(timeoutCtx); err != nil {
		a.logger.Error("Failed to connect scheduled DMR bridge",
			logger.String("name", a.config.Name),
			logger.Error(err))
		a.setState(StateFailed)
		a.setLastError(err.Error())
		return
	}

	// Maintain connection until timeout or cancellation
	a.maintainConnection(timeoutCtx)

	a.logger.Info("Scheduled DMR bridge duration completed", logger.String("name", a.config.Name))
	a.disconnect()
}

// connect establishes connection to DMR network
func (a *DMRBridgeAdapter) connect(ctx context.Context) error {
	a.setState(StateConnecting)

	// Start the ysf2dmr bridge
	a.logger.Info("DMR adapter invoking ysf2dmr.Bridge.Start", logger.String("name", a.config.Name))
	if err := a.bridge.Start(ctx); err != nil {
		a.logger.Error("ysf2dmr.Bridge.Start returned error", logger.Error(err), logger.String("name", a.config.Name))
		a.setLastError(err.Error())
		a.setState(StateFailed)
		return err
	}
	a.logger.Info("ysf2dmr.Bridge.Start completed successfully", logger.String("name", a.config.Name))

	now := time.Now()
	a.mu.Lock()
	a.connectedAt = &now
	a.disconnectedAt = nil
	a.lastError = ""
	a.mu.Unlock()

	a.setState(StateConnected)
	a.logger.Info("DMR bridge connected",
		logger.String("name", a.config.Name),
		logger.String("network", a.config.DMR.Network),
		logger.Uint32("talk_group", a.config.DMR.TalkGroup))

	return nil
}

// maintainConnection keeps the bridge running until context is cancelled
func (a *DMRBridgeAdapter) maintainConnection(ctx context.Context) {
	<-ctx.Done()
	a.logger.Info("DMR bridge connection context done", logger.String("name", a.config.Name))
}

// disconnect closes the DMR bridge connection
func (a *DMRBridgeAdapter) disconnect() {
	a.logger.Info("Disconnecting DMR bridge", logger.String("name", a.config.Name))

	a.setState(StateDisconnected)

	// Stop the ysf2dmr bridge
	if a.bridge != nil {
		if err := a.bridge.Stop(); err != nil {
			// Log stop error but continue disconnect
			a.logger.Warn("Error stopping ysf2dmr bridge", logger.Error(err))
		}
	}

	now := time.Now()
	a.mu.Lock()
	a.disconnectedAt = &now
	a.mu.Unlock()
}

// Disconnect is the public interface for disconnecting
func (a *DMRBridgeAdapter) Disconnect() error {
	a.disconnect()
	return nil
}

// GetStatus returns the current status of the DMR bridge
func (a *DMRBridgeAdapter) GetStatus() BridgeStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	status := BridgeStatus{
		Name:           a.config.Name,
		Type:           "dmr",
		State:          a.state,
		ConnectedAt:    a.connectedAt,
		DisconnectedAt: a.disconnectedAt,
		NextSchedule:   a.nextSchedule,
		Duration:       a.config.Duration,
		RetryCount:     a.retryCount,
		LastError:      a.lastError,
	}

	// Get stats from ysf2dmr bridge if available
	if a.bridge != nil {
		stats := a.bridge.GetStatistics()
		status.PacketsRx = stats.YSFPackets + stats.DMRPackets
		status.PacketsTx = stats.YSFPackets + stats.DMRPackets // Combined count
		status.BytesRx = 0                                     // Not tracked separately
		status.BytesTx = 0                                     // Not tracked separately

		// Add DMR-specific metadata
		status.Metadata = map[string]interface{}{
			"dmr_network":      a.config.DMR.Network,
			"talk_group":       a.config.DMR.TalkGroup,
			"dmr_id":           a.config.DMR.ID,
			"slot":             a.config.DMR.Slot,
			"total_calls":      stats.TotalCalls,
			"ysf_to_dmr_calls": stats.YSFToDMRCalls,
			"dmr_to_ysf_calls": stats.DMRToYSFCalls,
			"frames_dropped":   stats.FramesDropped,
		}

		// Add active call if present
		if activeCall := a.bridge.GetActiveCall(); activeCall != nil {
			status.Metadata["active_call"] = map[string]interface{}{
				"direction":    activeCall.Direction,
				"ysf_callsign": activeCall.YSFCallsign,
				"dmr_id":       activeCall.DMRID,
				"talk_group":   activeCall.TalkGroup,
				"start_time":   activeCall.StartTime,
			}
		}
	}

	return status
}

// GetName returns the bridge name
func (a *DMRBridgeAdapter) GetName() string {
	return a.config.Name
}

// GetType returns the bridge type
func (a *DMRBridgeAdapter) GetType() string {
	return "dmr"
}

// GetState returns the current connection state
func (a *DMRBridgeAdapter) GetState() BridgeState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// SetNextSchedule sets the next scheduled run time
func (a *DMRBridgeAdapter) SetNextSchedule(t *time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nextSchedule = t
}

// IsConnected returns whether the bridge is currently connected
func (a *DMRBridgeAdapter) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state == StateConnected
}

// setState updates the connection state
func (a *DMRBridgeAdapter) setState(state BridgeState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = state
}

// setLastError updates the last error message
func (a *DMRBridgeAdapter) setLastError(err string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastError = err
}

// handleConnectionFailure handles connection failures with retry logic
func (a *DMRBridgeAdapter) handleConnectionFailure() {
	a.mu.Lock()
	a.retryCount++
	a.mu.Unlock()

	a.setState(StateFailed)
}

// calculateRetryDelay calculates exponential backoff delay
func (a *DMRBridgeAdapter) calculateRetryDelay() time.Duration {
	a.mu.RLock()
	retryCount := a.retryCount
	a.mu.RUnlock()

	// Exponential backoff: baseDelay * 2^retryCount, capped at 5 minutes
	delay := a.baseRetryDelay * time.Duration(1<<uint(retryCount))
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	return delay
}

// InjectYSFPacket forwards a YSF packet to the underlying YSF2DMR bridge for conversion
// This allows the reflector to forward local repeater traffic to the DMR network
func (a *DMRBridgeAdapter) InjectYSFPacket(packet *network.Packet) error {
	// Only process if connected
	if !a.IsConnected() {
		return nil // Silently ignore if not connected
	}

	// Forward packet to the underlying ysf2dmr bridge for processing
	if a.bridge != nil {
		a.logger.Debug("Forwarding YSF packet to DMR bridge",
			logger.String("bridge", a.config.Name),
			logger.String("gateway", packet.Callsign),
			logger.String("source_cs", packet.SourceCS),
			logger.Int("size", len(packet.Data)))

		// The ysf2dmr bridge has a handler that processes YSFD packets
		// We need to call it directly since we're bypassing the shared server's
		// normal packet routing (the reflector handles packets before bridges see them)
		// This is a package-internal call to avoid reflection - we'll need to expose
		// the handler as a public method or use an internal method
		// For now, we'll use the bridge's handleYSFPacket directly (need to export it)
		return a.bridge.HandleYSFPacket(packet)
	}

	return nil
}
