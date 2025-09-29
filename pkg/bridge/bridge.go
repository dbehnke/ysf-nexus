package bridge

import (
	"context"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// Bridge represents a connection to another YSF reflector
type Bridge struct {
	config   config.BridgeConfig
	logger   *logger.Logger
	server   NetworkServer
	
	// Connection state
	mu             sync.RWMutex
	state          BridgeState
	remoteAddr     *net.UDPAddr
	connectedAt    *time.Time
	disconnectedAt *time.Time
	nextSchedule   *time.Time
	lastError      string
	
	// Retry handling
	retryCount     int
	maxRetries     int
	baseRetryDelay time.Duration
	
	// Statistics
	packetsRx uint64
	packetsTx uint64
	bytesRx   uint64
	bytesTx   uint64
	
	// Health checking
	lastPacketTime time.Time
	healthTicker   *time.Ticker
}

// NewBridge creates a new bridge instance
func NewBridge(cfg config.BridgeConfig, server NetworkServer, logger *logger.Logger) *Bridge {
	// Set default values if not configured
	maxRetries := cfg.MaxRetries
	retryDelay := cfg.RetryDelay
	if retryDelay == 0 {
		retryDelay = 30 * time.Second
	}
	
	return &Bridge{
		config:         cfg,
		logger:         logger,
		server:         server,
		state:          StateDisconnected,
		maxRetries:     maxRetries,
		baseRetryDelay: retryDelay,
		lastPacketTime: time.Now(),
	}
}

// RunPermanent runs a permanent bridge connection with auto-reconnection
func (b *Bridge) RunPermanent(ctx context.Context) {
	b.logger.Info("Starting permanent bridge")
	b.setState(StateScheduled)
	
	for {
		select {
		case <-ctx.Done():
			b.logger.Info("Permanent bridge context cancelled")
			b.disconnect()
			return
		default:
			if err := b.connect(ctx); err != nil {
				b.logger.Error("Failed to connect permanent bridge", logger.Error(err))
				b.handleConnectionFailure()
				
				// Wait before retry with exponential backoff
				delay := b.calculateRetryDelay()
				b.logger.Info("Retrying permanent bridge connection", 
					logger.Duration("delay", delay), 
					logger.Int("attempt", b.retryCount+1))
				
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
					continue
				}
			} else {
				// Connected successfully, reset retry count
				b.retryCount = 0
				b.maintainConnection(ctx)
			}
		}
	}
}

// RunScheduled runs a bridge connection for a scheduled duration
func (b *Bridge) RunScheduled(ctx context.Context, duration time.Duration) {
	b.logger.Info("Starting scheduled bridge", logger.Duration("duration", duration))
	b.setState(StateScheduled)
	
	// Create a timeout context for the scheduled duration
	scheduleCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	
	// Try to connect with retries during the scheduled window
	for {
		select {
		case <-scheduleCtx.Done():
			b.logger.Info("Scheduled bridge window ended")
			b.disconnect()
			return
		default:
			if err := b.connect(scheduleCtx); err != nil {
				b.logger.Error("Failed to connect scheduled bridge", logger.Error(err))
				b.handleConnectionFailure()
				
				// For scheduled bridges, be more aggressive with retries
				delay := b.calculateRetryDelay()
				if delay > 5*time.Minute {
					delay = 5 * time.Minute // Cap delay for scheduled bridges
				}
				
				b.logger.Info("Retrying scheduled bridge connection", 
					logger.Duration("delay", delay), 
					logger.Int("attempt", b.retryCount+1))
				
				select {
				case <-scheduleCtx.Done():
					return
				case <-time.After(delay):
					continue
				}
			} else {
				// Connected successfully, reset retry count and maintain connection
				b.retryCount = 0
				b.maintainConnection(scheduleCtx)
				return // Connection ended naturally
			}
		}
	}
}

// connect establishes a connection to the remote reflector
func (b *Bridge) connect(ctx context.Context) error {
	b.setState(StateConnecting)
	
	// Resolve the remote address
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", b.config.Host, b.config.Port))
	if err != nil {
		return fmt.Errorf("failed to resolve bridge address: %w", err)
	}
	
	b.mu.Lock()
	b.remoteAddr = addr
	b.mu.Unlock()
	
	// Send initial connection packet (YSF handshake)
	if err := b.sendHandshake(); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}
	
	// Wait for connection acknowledgment or timeout
	// For now, we'll consider the connection established after sending handshake
	// In a full implementation, you'd wait for a response packet
	
	now := time.Now()
	b.mu.Lock()
	b.state = StateConnected
	b.connectedAt = &now
	b.disconnectedAt = nil
	b.lastError = ""
	b.lastPacketTime = now
	b.mu.Unlock()
	
	b.logger.Info("Bridge connected", logger.String("remote", addr.String()))
	
	// Start health checking if configured
	if b.config.HealthCheck > 0 {
		b.startHealthCheck(ctx)
	}
	
	return nil
}

// maintainConnection maintains the bridge connection and handles packets
func (b *Bridge) maintainConnection(ctx context.Context) {
	// Send periodic keep-alive packets
	keepAliveTicker := time.NewTicker(30 * time.Second)
	defer keepAliveTicker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			b.logger.Info("Bridge connection context cancelled")
			b.disconnect()
			return
		case <-keepAliveTicker.C:
			if err := b.sendKeepAlive(); err != nil {
				b.logger.Error("Failed to send keep-alive", logger.Error(err))
				b.setConnectionError("keep-alive failed: " + err.Error())
				b.disconnect()
				return
			}
		}
		
		// Check if connection is healthy
		if b.config.HealthCheck > 0 && time.Since(b.lastPacketTime) > b.config.HealthCheck*2 {
			b.logger.Warn("Bridge connection unhealthy - no packets received", 
				logger.Any("last_packet", b.lastPacketTime))
			b.setConnectionError("connection timeout - no packets received")
			b.disconnect()
			return
		}
	}
}

// disconnect closes the bridge connection
func (b *Bridge) disconnect() {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if b.state == StateConnected {
		b.logger.Info("Disconnecting bridge")
		
		// Send disconnect packet
		if b.remoteAddr != nil {
			b.sendDisconnect()
		}
	}
	
	now := time.Now()
	b.state = StateDisconnected
	b.disconnectedAt = &now
	b.connectedAt = nil
	
	if b.healthTicker != nil {
		b.healthTicker.Stop()
		b.healthTicker = nil
	}
}

// handleConnectionFailure handles connection failures with retry logic
func (b *Bridge) handleConnectionFailure() {
	b.mu.Lock()
	b.retryCount++
	b.state = StateFailed
	now := time.Now()
	b.disconnectedAt = &now
	b.mu.Unlock()
	
	// Check if we've exceeded max retries (0 means infinite)
	if b.maxRetries > 0 && b.retryCount >= b.maxRetries {
		b.logger.Error("Bridge max retries exceeded", logger.Int("retries", b.retryCount))
		// State remains StateFailed - don't change it
	}
}

// calculateRetryDelay calculates the delay before next retry using exponential backoff
func (b *Bridge) calculateRetryDelay() time.Duration {
	// Exponential backoff: baseDelay * 2^retryCount with jitter
	delay := b.baseRetryDelay * time.Duration(math.Pow(2, float64(b.retryCount)))
	
	// Cap the delay at 10 minutes
	maxDelay := 10 * time.Minute
	if delay > maxDelay {
		delay = maxDelay
	}
	
	// Add jitter (±25%) to avoid thundering herd
	jitter := float64(delay) * 0.25
	jitterDelay := time.Duration(float64(delay) + (jitter * (2*float64(b.retryCount%2) - 1)))
	
	return jitterDelay
}

// startHealthCheck starts periodic health checking
func (b *Bridge) startHealthCheck(ctx context.Context) {
	if b.config.HealthCheck <= 0 {
		return
	}
	
	b.healthTicker = time.NewTicker(b.config.HealthCheck)
	
	go func() {
		defer b.healthTicker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-b.healthTicker.C:
				// Send ping packet to check connection health
				if err := b.sendPing(); err != nil {
					b.logger.Warn("Failed to send health check ping", logger.Error(err))
				}
			}
		}
	}()
}

// Packet sending methods (YSF protocol specific)

func (b *Bridge) sendHandshake() error {
	// Create YSF handshake packet
	// This would be protocol-specific implementation
	handshake := b.createHandshakePacket()
	return b.sendPacket(handshake)
}

func (b *Bridge) sendKeepAlive() error {
	// Create YSF keep-alive packet
	keepAlive := b.createKeepAlivePacket()
	return b.sendPacket(keepAlive)
}

func (b *Bridge) sendPing() error {
	// Create YSF ping packet for health checking
	ping := b.createPingPacket()
	return b.sendPacket(ping)
}

func (b *Bridge) sendDisconnect() error {
	// Create YSF disconnect packet
	disconnect := b.createDisconnectPacket()
	return b.sendPacket(disconnect)
}

func (b *Bridge) sendPacket(data []byte) error {
	if b.remoteAddr == nil {
		return fmt.Errorf("bridge not connected")
	}
	
	err := b.server.SendPacket(data, b.remoteAddr)
	if err == nil {
		b.mu.Lock()
		b.packetsTx++
		b.bytesTx += uint64(len(data))
		b.mu.Unlock()
	}
	
	return err
}

// Packet creation methods (placeholder implementations)

func (b *Bridge) createHandshakePacket() []byte {
	// TODO: Implement YSF-specific handshake packet
	// For now, return a basic placeholder
	return []byte("YSFP" + b.config.Name + "\000")
}

func (b *Bridge) createKeepAlivePacket() []byte {
	// TODO: Implement YSF-specific keep-alive packet
	return []byte("YSFS\000\000\000\000")
}

func (b *Bridge) createPingPacket() []byte {
	// TODO: Implement YSF-specific ping packet
	return []byte("YSFP")
}

func (b *Bridge) createDisconnectPacket() []byte {
	// TODO: Implement YSF-specific disconnect packet
	return []byte("YSFD")
}

// Status and utility methods

func (b *Bridge) GetStatus() BridgeStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	return BridgeStatus{
		Name:           b.config.Name,
		State:          b.state,
		ConnectedAt:    b.connectedAt,
		DisconnectedAt: b.disconnectedAt,
		NextSchedule:   b.nextSchedule,
		RetryCount:     b.retryCount,
		LastError:      b.lastError,
		PacketsRx:      b.packetsRx,
		PacketsTx:      b.packetsTx,
		BytesRx:        b.bytesRx,
		BytesTx:        b.bytesTx,
	}
}

func (b *Bridge) GetName() string {
	return b.config.Name
}

func (b *Bridge) IsConnectedTo(addr *net.UDPAddr) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	return b.remoteAddr != nil && b.remoteAddr.String() == addr.String()
}

func (b *Bridge) setState(state BridgeState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = state
}

func (b *Bridge) setConnectionError(err string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastError = err
}

func (b *Bridge) IncrementRxStats(bytes uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.packetsRx++
	b.bytesRx += bytes
	b.lastPacketTime = time.Now()
}

// ForwardPacket forwards a packet through this bridge connection
func (b *Bridge) ForwardPacket(data []byte) error {
	if b.state != StateConnected {
		return fmt.Errorf("bridge not connected")
	}
	
	return b.sendPacket(data)
}