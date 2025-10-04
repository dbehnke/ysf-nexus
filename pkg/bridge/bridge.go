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
	config config.BridgeConfig
	logger *logger.Logger
	server NetworkServer

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
	lastPingTime   time.Time
	awaitingPong   bool
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

				// If we've exceeded max retries (and maxRetries > 0), stop retrying and return.
				if b.maxRetries > 0 && b.retryCount >= b.maxRetries {
					b.logger.Error("Max retries exceeded - giving up on permanent bridge", logger.Int("retries", b.retryCount))
					return
				}

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

	b.logger.Info("Scheduled bridge timeout set",
		logger.String("bridge", b.config.Name),
		logger.Duration("duration", duration))

	// Try to connect with retries during the scheduled window
	for {
		select {
		case <-scheduleCtx.Done():
			b.logger.Info("Scheduled bridge window ended - disconnecting",
				logger.String("bridge", b.config.Name),
				logger.Duration("duration", duration))
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
				// Connection ended (dropped or stopped) — continue attempting to reconnect
				// until the scheduled window (scheduleCtx) expires.
				continue
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
			b.logger.Info("Bridge connection context cancelled - disconnecting",
				logger.String("bridge", b.config.Name),
				logger.String("reason", ctx.Err().Error()))
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

		// Send disconnect packet using sendDisconnectLocked to avoid double-lock
		if b.remoteAddr != nil {
			if err := b.sendDisconnectLocked(); err != nil {
				b.logger.Warn("Failed to send disconnect packet", logger.Error(err))
			}
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

// startHealthCheck starts periodic health checking like a proper YSF repeater
func (b *Bridge) startHealthCheck(ctx context.Context) {
	if b.config.HealthCheck <= 0 {
		return
	}

	// Use a ticker with a shorter interval to check if we need to send pings
	b.healthTicker = time.NewTicker(5 * time.Second)

	go func() {
		defer b.healthTicker.Stop()

		// Send initial ping
		if err := b.sendPing(); err != nil {
			b.logger.Warn("Failed to send initial ping", logger.Error(err))
		} else {
			b.lastPingTime = time.Now()
			b.awaitingPong = true
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-b.healthTicker.C:
				b.checkPingResponse(ctx)
			}
		}
	}()
}

// checkPingResponse checks if we need to send a ping or handle timeout
func (b *Bridge) checkPingResponse(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()

	// If we're awaiting a pong and it's been too long, consider connection lost
	if b.awaitingPong && now.Sub(b.lastPingTime) > b.config.HealthCheck {
		b.logger.Warn("Bridge ping timeout - no response received",
			logger.Duration("elapsed", now.Sub(b.lastPingTime)))
		b.awaitingPong = false
		// Connection might be lost, will retry on next health check cycle
	}

	// If we're not awaiting a pong and enough time has passed, send a new ping
	if !b.awaitingPong && now.Sub(b.lastPingTime) >= b.config.HealthCheck {
		if err := b.sendPingLocked(); err != nil {
			b.logger.Warn("Failed to send ping", logger.Error(err))
		} else {
			b.lastPingTime = now
			b.awaitingPong = true
		}
	}
}

// sendPingLocked sends a ping packet (assumes mutex is already locked)
func (b *Bridge) sendPingLocked() error {
	ping := b.createPingPacket()
	return b.sendPacketLocked(ping)
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

func (b *Bridge) sendDisconnectLocked() error {
	// Create YSF disconnect packet
	disconnect := b.createDisconnectPacket()
	return b.sendPacketLocked(disconnect)
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

func (b *Bridge) sendPacketLocked(data []byte) error {
	if b.remoteAddr == nil {
		return fmt.Errorf("bridge not connected")
	}

	err := b.server.SendPacket(data, b.remoteAddr)
	if err == nil {
		b.packetsTx++
		b.bytesTx += uint64(len(data))
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
	// Create proper YSF poll packet (YSFP) - 14 bytes total
	packet := make([]byte, 14)

	// Type: YSFP
	copy(packet[0:4], "YSFP")

	// Callsign (padded to 10 bytes with spaces)
	callsign := b.config.Name
	if len(callsign) > 10 {
		callsign = callsign[:10]
	}
	copy(packet[4:14], fmt.Sprintf("%-10s", callsign))

	return packet
}

func (b *Bridge) createDisconnectPacket() []byte {
	// Create YSFU (YSF Unlink) packet - 14 bytes total
	packet := make([]byte, 14)

	// Packet type: YSFU
	copy(packet[0:4], "YSFU")

	// Callsign (padded to 10 bytes with spaces)
	callsign := b.config.Name
	if len(callsign) > 10 {
		callsign = callsign[:10]
	}
	copy(packet[4:14], fmt.Sprintf("%-10s", callsign))

	return packet
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
		Duration:       b.config.Duration,
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

func (b *Bridge) SetNextSchedule(next *time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextSchedule = next
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

// OnPacketReceived handles incoming packets for ping response detection
func (b *Bridge) OnPacketReceived(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if this is a response to our ping (any packet indicates the bridge is alive)
	if b.awaitingPong {
		b.awaitingPong = false
		b.logger.Debug("Received response from bridge",
			logger.String("bridge", b.config.Name),
			logger.Int("size", len(data)))
	}
}

// ForwardPacket forwards a packet through this bridge connection
func (b *Bridge) ForwardPacket(data []byte) error {
	if b.state != StateConnected {
		return fmt.Errorf("bridge not connected")
	}

	return b.sendPacket(data)
}

// IsConnected returns true if the bridge is currently connected
func (b *Bridge) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state == StateConnected
}

// GetRemoteAddr returns the remote address of the bridge connection
func (b *Bridge) GetRemoteAddr() *net.UDPAddr {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.remoteAddr
}
