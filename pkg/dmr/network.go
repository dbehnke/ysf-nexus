package dmr

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// NetworkState represents the DMR network connection state
type NetworkState int

const (
	StateDisconnected NetworkState = iota
	StateConnecting
	StateAuthenticating
	StateAuthenticated
	StateRunning
	StateFailed
)

func (s NetworkState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateAuthenticating:
		return "authenticating"
	case StateAuthenticated:
		return "authenticated"
	case StateRunning:
		return "running"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Config holds DMR network configuration
type Config struct {
	Address     string  // DMR network server address
	Port        int     // DMR network server port
	RepeaterID  uint32  // Our DMR ID/Repeater ID
	Password    string  // Network password
	Callsign    string  // Station callsign
	RXFreq      uint32  // RX frequency in Hz
	TXFreq      uint32  // TX frequency in Hz
	TXPower     uint8   // TX power in dBm (00-99)
	ColorCode   uint8   // DMR color code (01-15)
	Latitude    float32 // Station latitude
	Longitude   float32 // Station longitude
	Height      int32   // Antenna height in meters
	Location    string  // Station location text
	Description string  // Station description
	URL         string  // Station URL
	SoftwareID  string  // Software identification
	PackageID   string  // Package identification
	// If true, send a minimal/safe RPTC to reduce fields that may be rejected
	SafeRPTC  bool
	Slot      uint8  // Default slot (1 or 2)
	TalkGroup uint32 // Default talkgroup

	// Network options
	PingInterval time.Duration // How often to respond to pings
	AuthTimeout  time.Duration // Authentication timeout
}

// Network represents a DMR network client
type Network struct {
	config Config
	logger *logger.Logger

	// Connection
	conn       *net.UDPConn
	remoteAddr *net.UDPAddr
	localAddr  *net.UDPAddr

	// State
	mu            sync.RWMutex
	state         NetworkState
	authenticated bool
	salt          []byte
	lastError     string

	// Channels
	rxChan   chan *Packet
	txQueue  chan []byte
	stopChan chan struct{}

	// Stream tracking
	streamID     uint32
	streamIDLock sync.Mutex

	// Statistics
	packetsRx uint64
	packetsTx uint64
	bytesRx   uint64
	bytesTx   uint64

	// Timing
	lastPacketRx time.Time
	lastPacketTx time.Time
	lastPing     time.Time
}

// Temporary testing flag: when true, do not send RPTC after authentication.
// This is intended for short-lived debugging only and should be removed
// or made configurable once the investigation is complete.
var skipRPTC bool = false

// NewNetwork creates a new DMR network client
func NewNetwork(config Config, log *logger.Logger) *Network {
	// Set default values
	if config.PingInterval == 0 {
		config.PingInterval = 10 * time.Second
	}
	if config.AuthTimeout == 0 {
		config.AuthTimeout = 30 * time.Second
	}
	if config.SoftwareID == "" {
		config.SoftwareID = "MMDVM" // Default to MMDVM for BrandMeister compatibility
	}
	if config.PackageID == "" {
		config.PackageID = "MMDVM" // Default to MMDVM for BrandMeister compatibility
	}

	return &Network{
		config:   config,
		logger:   log.WithComponent("dmr-network"),
		state:    StateDisconnected,
		rxChan:   make(chan *Packet, 100),
		txQueue:  make(chan []byte, 100),
		stopChan: make(chan struct{}),
	}
}

// Start starts the DMR network client
func (n *Network) Start(ctx context.Context) error {
	n.logger.Info("Starting DMR network client",
		logger.String("address", n.config.Address),
		logger.Int("port", n.config.Port),
		logger.Uint32("repeater_id", n.config.RepeaterID))

	// Info: resolving remote address (always log attempt)
	n.logger.Info("Resolving DMR server address", logger.String("address", n.config.Address), logger.Int("port", n.config.Port))
	// Debug: resolving remote address
	n.logger.Debug("Resolving DMR server address (debug)", logger.String("address", n.config.Address), logger.Int("port", n.config.Port))
	// Resolve remote address
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", n.config.Address, n.config.Port))
	if err != nil {
		return fmt.Errorf("failed to resolve DMR server address: %w", err)
	}
	n.remoteAddr = addr

	// Create UDP connection
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection: %w", err)
	}
	n.conn = conn
	n.localAddr = conn.LocalAddr().(*net.UDPAddr)

	n.logger.Info("DMR network UDP socket created",
		logger.String("local", n.localAddr.String()),
		logger.String("remote", n.remoteAddr.String()))
	n.logger.Debug("DMR network socket ready to send/receive", logger.String("local", n.localAddr.String()))

	// Start receiver goroutine
	go n.receiveLoop(ctx)

	// Start transmitter goroutine
	go n.transmitLoop(ctx)

	// Authenticate
	if err := n.authenticate(ctx); err != nil {
		n.setState(StateFailed)
		n.setError(err.Error())
		return fmt.Errorf("authentication failed: %w", err)
	}

	n.setState(StateRunning)
	n.logger.Info("DMR network client running")

	// Start ping responder
	go n.pingLoop(ctx)

	return nil
}

// Stop stops the DMR network client
func (n *Network) Stop() error {
	n.logger.Info("Stopping DMR network client")

	close(n.stopChan)

	if n.conn != nil {
		if err := n.conn.Close(); err != nil {
			n.logger.Warn("Error closing UDP connection", logger.Error(err))
		}
	}

	n.setState(StateDisconnected)
	return nil
}

// authenticate performs the DMR network authentication sequence
func (n *Network) authenticate(ctx context.Context) error {
	n.setState(StateConnecting)

	// Step 1: Send RPTL (login)
	n.logger.Info("Sending RPTL login packet",
		logger.Uint32("repeater_id", n.config.RepeaterID),
		logger.String("repeater_id_hex", fmt.Sprintf("%08X", n.config.RepeaterID)))
	loginPacket := NewRPTLPacket(n.config.RepeaterID)
	rpltBytes := loginPacket.Serialize()
	n.logger.Debug("RPTL packet bytes", logger.String("hex", fmt.Sprintf("%x", rpltBytes)))
	if err := n.sendPacket(rpltBytes); err != nil {
		return fmt.Errorf("failed to send RPTL: %w", err)
	}

	// Step 2: Wait for MSTACK or RPTA with salt
	n.setState(StateAuthenticating)
	n.logger.Info("Waiting for MSTACK/RPTA from server")

	authTimeout := time.NewTimer(n.config.AuthTimeout)
	defer authTimeout.Stop()

	var salt []byte
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-authTimeout.C:
			return fmt.Errorf("authentication timeout waiting for MSTACK/RPTA")
		case packet := <-n.rxChan:
			n.logger.Debug("Auth packet received",
				logger.String("type", packet.Type),
				logger.Int("len", len(packet.Data)),
				logger.String("hex", fmt.Sprintf("%x", packet.Data)))

			switch packet.Type {
			case PacketTypeMSTAK, "MSTACK": // Handle both forms
				n.logger.Info("Received MSTACK - login accepted, waiting for RPTA challenge")
				// Continue waiting for RPTA with salt
				continue
			case PacketTypeRPTA, PacketTypeRPTACK: // Handle both RPTA and RPTACK
				_, receivedSalt, err := ParseRPTAPacket(packet.Data)
				if err != nil {
					return fmt.Errorf("failed to parse RPTA: %w", err)
				}
				salt = receivedSalt
				n.salt = salt
				n.logger.Info("Received RPTA with salt",
					logger.Int("salt_len", len(salt)),
					logger.String("salt_hex", fmt.Sprintf("%x", salt)))

				// Compute RPTK hash: SHA256(salt + password) - send the literal ASCII bytes
				// For passwords that look like hex (e.g. "FFEE1122") we want to send
				// the ASCII hex characters, not the binary decode.
				passwordBytes := []byte(n.config.Password)
				method := "ASCII"

				// CORRECT ORDER: salt + password (not password + salt!)
				h := sha256.New()
				h.Write(salt)
				h.Write(passwordBytes)
				hash := h.Sum(nil)

				n.logger.Info("Computed RPTK hash",
					logger.String("method", method),
					logger.Int("password_bytes", len(passwordBytes)),
					logger.String("salt_hex", fmt.Sprintf("%x", salt)),
					logger.String("hash_hex", fmt.Sprintf("%x", hash)))

				goto saltReceived
			case PacketTypeMSTNAK, PacketTypeMSTN:
				n.logger.Error("Authentication rejected by server (MSTNAK/MSTN)")
				return fmt.Errorf("authentication rejected by server")
			default:
				n.logger.Debug("Ignoring packet during auth", logger.String("type", packet.Type))
			}
		}
	}

saltReceived:
	// Step 3: Send RPTK (password hash)
	n.logger.Info("Sending RPTK password hash")

	// Use the literal ASCII password bytes for RPTK (per requested change)
	passwordBytes := []byte(n.config.Password)
	n.logger.Info("Using ASCII password bytes for RPTK")

	keyPacket := NewRPTKPacketBytes(n.config.RepeaterID, passwordBytes, salt)
	rptkBytes := keyPacket.Serialize()
	// Log the exact RPTK packet bytes (hex) for debugging
	n.logger.Info("RPTK packet being sent",
		logger.String("repeater_id_hex", fmt.Sprintf("%08X", n.config.RepeaterID)),
		logger.String("packet_hex", fmt.Sprintf("%x", rptkBytes)))
	if err := n.sendPacket(rptkBytes); err != nil {
		return fmt.Errorf("failed to send RPTK: %w", err)
	}

	// Step 4: Wait for RPTA confirmation
	n.logger.Info("Waiting for RPTK acknowledgment")
	authTimeout.Reset(n.config.AuthTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-authTimeout.C:
			return fmt.Errorf("authentication timeout waiting for RPTK ACK")
		case packet := <-n.rxChan:
			n.logger.Info("Received packet after RPTK",
				logger.String("type", packet.Type),
				logger.Int("len", len(packet.Data)),
				logger.String("hex", fmt.Sprintf("%x", packet.Data)))

			switch packet.Type {
			case PacketTypeRPTA, PacketTypeRPTACK:
				n.logger.Info("✓ RPTK authentication accepted by server")
				goto authenticated
			case PacketTypeMSTN, PacketTypeMSTNAK:
				n.logger.Error("✗ Authentication rejected by server",
					logger.String("packet_type", packet.Type),
					logger.String("payload_hex", fmt.Sprintf("%x", packet.Data)))
				return fmt.Errorf("authentication rejected by server (%s)", packet.Type)
			default:
				n.logger.Debug("Ignoring non-auth packet after RPTK", logger.String("type", packet.Type))
			}
		}
	}

authenticated:
	// Step 5: Send RPTC (configuration)
	if skipRPTC {
		n.logger.Info("Skipping RPTC send due to temporary debug flag")
		// If we're skipping RPTC, report success immediately (no final RPTA wait)
		n.authenticated = true
		n.setState(StateAuthenticated)
		return nil
	}

	n.logger.Info("Sending RPTC configuration packet")
	configPacket := NewRPTCPacket(n.config.RepeaterID)
	// Populate RPTC fields from config
	configPacket.Callsign = n.config.Callsign
	configPacket.RXFreq = n.config.RXFreq
	configPacket.TXFreq = n.config.TXFreq
	configPacket.TXPower = n.config.TXPower
	configPacket.ColorCode = n.config.ColorCode
	configPacket.Latitude = n.config.Latitude
	configPacket.Longitude = n.config.Longitude
	configPacket.Height = n.config.Height
	configPacket.Location = n.config.Location
	configPacket.Description = n.config.Description
	configPacket.URL = n.config.URL
	configPacket.SoftwareID = n.config.SoftwareID
	configPacket.PackageID = n.config.PackageID

	rptcBytes := configPacket.Serialize()
	n.logger.Debug("RPTC packet details",
		logger.String("callsign", n.config.Callsign),
		logger.Uint32("rx_freq", n.config.RXFreq),
		logger.Uint32("tx_freq", n.config.TXFreq),
		logger.Int("packet_len", len(rptcBytes)))
	n.logger.Info("RPTC packet hex", logger.String("hex", fmt.Sprintf("%x", rptcBytes)))

	if err := n.sendPacket(rptcBytes); err != nil {
		return fmt.Errorf("failed to send RPTC: %w", err)
	}

	// Step 6: Wait for final RPTA
	n.logger.Info("Waiting for RPTC acknowledgment")
	authTimeout.Reset(n.config.AuthTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-authTimeout.C:
			return fmt.Errorf("authentication timeout waiting for RPTC ACK")
		case packet := <-n.rxChan:
			n.logger.Info("Received packet after RPTC",
				logger.String("type", packet.Type),
				logger.Int("len", len(packet.Data)),
				logger.String("hex", fmt.Sprintf("%x", packet.Data)))

			switch packet.Type {
			case PacketTypeRPTA, PacketTypeRPTACK:
				n.logger.Info("✓ DMR network authentication SUCCESSFUL",
					logger.String("network", n.config.Address),
					logger.Uint32("repeater_id", n.config.RepeaterID))
				n.authenticated = true
				n.setState(StateAuthenticated)
				return nil
			case PacketTypeMSTN, PacketTypeMSTNAK:
				n.logger.Error("✗ Configuration rejected by server",
					logger.String("packet_type", packet.Type),
					logger.String("payload_hex", fmt.Sprintf("%x", packet.Data)))
				return fmt.Errorf("configuration rejected by server (%s)", packet.Type)
			default:
				n.logger.Debug("Ignoring non-config packet after RPTC", logger.String("type", packet.Type))
			}
		}
	}
}

// receiveLoop receives packets from the network
func (n *Network) receiveLoop(ctx context.Context) {
	buffer := make([]byte, 1500)

	for {
		select {
		case <-ctx.Done():
			return
		case <-n.stopChan:
			return
		default:
			// Set read deadline to allow checking context
			if err := n.conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				// If we can't set a deadline, log and continue - receiver may still work
				n.logger.Debug("Failed to set read deadline", logger.Error(err))
			}

			length, addr, err := n.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				n.logger.Error("Failed to read from UDP", logger.Error(err))
				continue
			}

			// Verify source address
			if addr.String() != n.remoteAddr.String() {
				n.logger.Warn("Received packet from unexpected address",
					logger.String("expected", n.remoteAddr.String()),
					logger.String("actual", addr.String()))
				continue
			}

			// Update statistics
			n.mu.Lock()
			n.packetsRx++
			n.bytesRx += uint64(length)
			n.lastPacketRx = time.Now()
			n.mu.Unlock()

			// Parse packet
			packet, err := ParsePacket(buffer[:length])
			if err != nil {
				n.logger.Error("Failed to parse packet", logger.Error(err))
				continue
			}

			// Log at DEBUG level for all packets
			n.logger.Debug("Received DMR packet",
				logger.String("type", packet.Type),
				logger.Int("size", length))

			// Log at INFO level for voice data packets (DMRD)
			if packet.Type == PacketTypeDMRD {
				// Parse DMRD packet to get source/dest IDs
				if dmrdPacket, err := ParseDMRDPacket(packet.Data); err == nil {
					n.logger.Info("RX",
						logger.String("type", "DMRD"),
						logger.String("source", addr.String()),
						logger.Int("size", length),
						logger.Uint32("src_id", dmrdPacket.SrcID),
						logger.Uint32("dst_id", dmrdPacket.DstID),
						logger.Int("slot", int(dmrdPacket.Slot)))
				}
			}

			// Send to rx channel (non-blocking)
			select {
			case n.rxChan <- packet:
			default:
				n.logger.Warn("RX channel full, dropping packet")
			}
		}
	}
}

// transmitLoop sends packets from the tx queue
func (n *Network) transmitLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-n.stopChan:
			return
		case data := <-n.txQueue:
			if _, err := n.conn.WriteToUDP(data, n.remoteAddr); err != nil {
				n.logger.Error("Failed to send packet", logger.Error(err))
				continue
			}

			// Update statistics
			n.mu.Lock()
			n.packetsTx++
			n.bytesTx += uint64(len(data))
			n.lastPacketTx = time.Now()
			n.mu.Unlock()

			// Log TX at INFO level for DMRD packets
			if len(data) >= 4 {
				packetType := string(data[:4])
				if packetType == PacketTypeDMRD {
					// Parse to get details
					if packet, err := ParsePacket(data); err == nil {
						if dmrdPacket, err := ParseDMRDPacket(packet.Data); err == nil {
							n.logger.Info("TX",
								logger.String("type", "DMRD"),
								logger.String("to", n.remoteAddr.String()),
								logger.Int("size", len(data)),
								logger.Uint32("src_id", dmrdPacket.SrcID),
								logger.Uint32("dst_id", dmrdPacket.DstID),
								logger.Int("slot", int(dmrdPacket.Slot)))
						}
					}
				}
			}
		}
	}
}

// pingLoop responds to server pings
func (n *Network) pingLoop(ctx context.Context) {
	n.logger.Info("Starting ping responder loop")

	for {
		select {
		case <-ctx.Done():
			return
		case <-n.stopChan:
			return
		case packet := <-n.rxChan:
			// Check if this is a ping packet
			isPing := packet.Type == PacketTypeMSTP || packet.Type == "MSTP"
			if !isPing && len(packet.Data) >= 7 {
				isPing = string(packet.Data[:7]) == "MSTPING"
			}

			if isPing {
				n.logger.Debug("Received MSTPING from server",
					logger.String("packet_hex", fmt.Sprintf("%x", packet.Data)))
				n.lastPing = time.Now()

				// Send RPTPONG response
				response := NewMSTPPacket(n.config.RepeaterID)
				pongBytes := response.Serialize()
				n.logger.Debug("Sending RPTPONG",
					logger.String("packet_hex", fmt.Sprintf("%x", pongBytes)))
				if err := n.sendPacket(pongBytes); err != nil {
					n.logger.Error("Failed to send RPTPONG response", logger.Error(err))
				} else {
					n.logger.Debug("Sent RPTPONG to server")
				}
			} else {
				// Put non-ping packet back for other handlers
				select {
				case n.rxChan <- packet:
				default:
					n.logger.Warn("Failed to return non-ping packet to channel",
						logger.String("type", packet.Type))
				}
			}
		}
	}
}

// sendPacket sends a packet to the network
func (n *Network) sendPacket(data []byte) error {
	select {
	case n.txQueue <- data:
		n.logger.Debug("Enqueued packet for transmit", logger.Int("len", len(data)))
		return nil
	case <-time.After(1 * time.Second):
		n.logger.Warn("Timed out enqueuing packet to tx queue", logger.Int("len", len(data)))
		return fmt.Errorf("tx queue full")
	}
}

// SendVoiceHeader sends a DMR voice header packet
func (n *Network) SendVoiceHeader(srcID, dstID uint32, slot uint8, callType uint8, streamID uint32) error {
	packet := NewDMRDPacket()
	packet.Sequence = 0
	packet.SrcID = srcID
	packet.DstID = dstID
	packet.RepeaterID = n.config.RepeaterID
	packet.Slot = slot
	packet.CallType = callType
	packet.FrameType = FrameTypeVoiceHeader
	packet.StreamID = streamID

	return n.sendPacket(packet.Serialize())
}

// SendVoiceData sends a DMR voice data packet
func (n *Network) SendVoiceData(srcID, dstID uint32, slot uint8, callType uint8, streamID uint32, sequence uint8, data []byte) error {
	packet := NewDMRDPacket()
	packet.Sequence = sequence
	packet.SrcID = srcID
	packet.DstID = dstID
	packet.RepeaterID = n.config.RepeaterID
	packet.Slot = slot
	packet.CallType = callType
	packet.FrameType = FrameTypeVoiceData
	packet.StreamID = streamID
	copy(packet.Data, data)

	return n.sendPacket(packet.Serialize())
}

// SendVoiceTerminator sends a DMR voice terminator packet
func (n *Network) SendVoiceTerminator(srcID, dstID uint32, slot uint8, callType uint8, streamID uint32, sequence uint8) error {
	packet := NewDMRDPacket()
	packet.Sequence = sequence
	packet.SrcID = srcID
	packet.DstID = dstID
	packet.RepeaterID = n.config.RepeaterID
	packet.Slot = slot
	packet.CallType = callType
	packet.FrameType = FrameTypeVoiceTerminator
	packet.StreamID = streamID

	return n.sendPacket(packet.Serialize())
}

// ReceivePacket receives a packet from the network (blocking)
func (n *Network) ReceivePacket(ctx context.Context) (*Packet, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case packet := <-n.rxChan:
		return packet, nil
	}
}

// GetStreamID generates a new unique stream ID
func (n *Network) GetStreamID() uint32 {
	n.streamIDLock.Lock()
	defer n.streamIDLock.Unlock()

	n.streamID++
	if n.streamID == 0 {
		n.streamID = 1
	}

	return n.streamID
}

// GetState returns the current network state
func (n *Network) GetState() NetworkState {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.state
}

// IsAuthenticated returns true if authenticated
func (n *Network) IsAuthenticated() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.authenticated
}

// GetStatistics returns network statistics
func (n *Network) GetStatistics() (packetsRx, packetsTx, bytesRx, bytesTx uint64) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.packetsRx, n.packetsTx, n.bytesRx, n.bytesTx
}

// setState sets the current state
func (n *Network) setState(state NetworkState) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.state = state
	n.logger.Info("DMR network state changed", logger.String("state", state.String()))
}

// setError sets the last error
func (n *Network) setError(err string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.lastError = err
}

// decodeHexPassword removed: we now always use ASCII password bytes for RPTK
