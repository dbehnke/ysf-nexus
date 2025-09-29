package network

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// PacketHandler handles incoming packets
type PacketHandler func(*Packet) error

// Server represents the UDP server for YSF communication
type Server struct {
	host     string
	port     int
	conn     *net.UDPConn
	handlers map[string]PacketHandler
	metrics  *Metrics
	debug    bool
	mu       sync.RWMutex
	running  bool
	logger   *logger.Logger
}

// Metrics holds server metrics
type Metrics struct {
	PacketsReceived map[string]int64
	PacketsSent     map[string]int64
	BytesReceived   int64
	BytesSent       int64
	Connections     int64
	Uptime          time.Time
	mu              sync.RWMutex
}

// NewServer creates a new UDP server
// NewServer creates a new UDP server with a default logger.
func NewServer(host string, port int) *Server {
	return NewServerWithLogger(host, port, logger.Default())
}

// NewServerWithLogger creates a new UDP server and attaches the provided logger.
func NewServerWithLogger(host string, port int, log *logger.Logger) *Server {
	s := &Server{
		host:     host,
		port:     port,
		handlers: make(map[string]PacketHandler),
		metrics: &Metrics{
			PacketsReceived: make(map[string]int64),
			PacketsSent:     make(map[string]int64),
			Uptime:          time.Now(),
		},
		logger: log.WithComponent("network"),
	}
	return s
}

// RegisterHandler registers a packet handler for a specific packet type
func (s *Server) RegisterHandler(packetType string, handler PacketHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[packetType] = handler
}

// SetDebug enables or disables debug logging
func (s *Server) SetDebug(debug bool) {
	s.debug = debug
}

// Start starts the UDP server
func (s *Server) Start(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", s.host, s.port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	s.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to start UDP server: %w", err)
	}

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Info("YSF server listening", logger.String("host", s.host), logger.Int("port", s.port))
	}

	// Start packet processing goroutine
	go s.processPackets(ctx)

	// Wait for context cancellation
	<-ctx.Done()

	return s.Stop()
}

// Stop stops the UDP server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	if s.conn != nil {
		return s.conn.Close()
	}

	return nil
}

// processPackets processes incoming UDP packets
func (s *Server) processPackets(ctx context.Context) {
	buffer := make([]byte, 1024)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Set read timeout to allow periodic context checking
			if err := s.conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				if s.isRunning() && s.logger != nil {
					s.logger.Warn("SetReadDeadline failed", logger.Error(err))
				}
			}

			n, addr, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Timeout is expected, continue
				}
				if s.isRunning() && s.logger != nil {
					s.logger.Error("Error reading UDP packet", logger.Error(err))
				}
				continue
			}

			// Create packet copy
			data := make([]byte, n)
			copy(data, buffer[:n])

			// Process packet in goroutine to avoid blocking
			go s.handlePacket(data, addr)
		}
	}
}

// handlePacket handles a single incoming packet
func (s *Server) handlePacket(data []byte, addr *net.UDPAddr) {
	// Update metrics
	s.updateMetrics(data, true)

	// Determine packet type for logging (use first 4 bytes when available)
	pktType := ""
	if len(data) >= 4 {
		pktType = string(data[:4])
	} else if len(data) > 0 {
		pktType = string(data)
	}

	isYSF := len(data) >= 3 && string(data[:3]) == "YSF"
	if isYSF {
		if s.debug {
			if s.logger != nil {
				s.logger.Debug("YSF RX hexdump",
					logger.String("from", addr.String()),
					logger.Int("size", len(data)),
					logger.String("hexdump", hexdumpSideBySide(data)))
			}
		}
		// When debug is off we will emit a single INFO log after parsing succeeds.
		// If parsing fails, the parse error branch will emit a fallback INFO log.
	}

	// Parse packet
	packet, err := ParsePacket(data, addr)
	if err != nil {
		if s.debug {
			if s.logger != nil {
				s.logger.Debug("Failed to parse packet", logger.String("from", addr.String()), logger.Error(err))
			}
		} else if isYSF {
			// Parsing failed and debug is off — emit fallback concise INFO once
			if pktType == "" {
				pktType = "YSF"
			}
			s.infoRxLog(pktType, nil, addr, len(data))
		}
		return
	}

	if s.debug {
		if s.logger != nil {
			s.logger.Debug("Parsed packet", logger.String("packet", packet.String()))
		}
	}

	// When debug is off, emit concise INFO-level logging including packet type, size, and callsign (if present)
	if !s.debug && isYSF {
		// Use consolidated helper with parsed packet available
		s.infoRxLog("", packet, nil, 0)
	}

	// Find handler for packet type
	s.mu.RLock()
	handler, exists := s.handlers[packet.Type]
	s.mu.RUnlock()

	if !exists {
		if s.debug && s.logger != nil {
			s.logger.Debug("No handler for packet type", logger.String("type", packet.Type))
		}
		return
	}

	// Call handler
	if err := handler(packet); err != nil {
		if s.logger != nil {
			s.logger.Error("Handler error for packet type", logger.String("type", packet.Type), logger.Error(err))
		}
	}
}

// SendPacket sends a packet to the specified address
func (s *Server) SendPacket(data []byte, addr *net.UDPAddr) error {
	if !s.isRunning() {
		return fmt.Errorf("server not running")
	}

	n, err := s.conn.WriteToUDP(data, addr)
	if err != nil {
		return fmt.Errorf("failed to send packet: %w", err)
	}

	// Update metrics
	s.updateMetrics(data, false)

	// YSF TX logging
	if len(data) >= 3 && string(data[:3]) == "YSF" {
		pktType := ""
		if len(data) >= 4 {
			pktType = string(data[:4])
		} else {
			pktType = "YSF"
		}

		if s.debug {
			if s.logger != nil {
				s.logger.Debug("YSF TX hexdump",
					logger.String("to", addr.String()),
					logger.Int("size", n),
					logger.String("hexdump", hexdumpSideBySide(data)))
			}
		} else if s.logger != nil {
			s.logger.Info("TX",
				logger.String("type", pktType),
				logger.String("to", addr.String()),
				logger.Int("size", n))
		}
	} else if s.debug && s.logger != nil {
		s.logger.Debug("Sent bytes",
			logger.String("to", addr.String()),
			logger.Int("size", n),
			logger.String("preview", fmt.Sprintf("%x", data[:min(len(data), 16)])))
	}

	return nil
}

// BroadcastData broadcasts data to multiple addresses
func (s *Server) BroadcastData(data []byte, addresses []*net.UDPAddr, exclude *net.UDPAddr) error {
	if !s.isRunning() {
		return fmt.Errorf("server not running")
	}

	sent := 0
	for _, addr := range addresses {
		if exclude != nil && addr.String() == exclude.String() {
			continue
		}

		if err := s.SendPacket(data, addr); err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to send packet", logger.String("to", addr.String()), logger.Error(err))
			}
			continue
		}
		sent++
	}

	if s.debug && s.logger != nil {
		s.logger.Debug("Broadcast completed", logger.Int("sent", sent))
	}

	return nil
}

// GetMetrics returns current server metrics
func (s *Server) GetMetrics() *Metrics {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	// Create a copy to avoid race conditions
	metrics := &Metrics{
		PacketsReceived: make(map[string]int64),
		PacketsSent:     make(map[string]int64),
		BytesReceived:   s.metrics.BytesReceived,
		BytesSent:       s.metrics.BytesSent,
		Connections:     s.metrics.Connections,
		Uptime:          s.metrics.Uptime,
	}

	for k, v := range s.metrics.PacketsReceived {
		metrics.PacketsReceived[k] = v
	}

	for k, v := range s.metrics.PacketsSent {
		metrics.PacketsSent[k] = v
	}

	return metrics
}

// isRunning checks if the server is running (thread-safe)
func (s *Server) isRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// updateMetrics updates server metrics
func (s *Server) updateMetrics(data []byte, received bool) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	if len(data) >= 4 {
		packetType := string(data[:4])
		if received {
			s.metrics.PacketsReceived[packetType]++
			s.metrics.BytesReceived += int64(len(data))
		} else {
			s.metrics.PacketsSent[packetType]++
			s.metrics.BytesSent += int64(len(data))
		}
	}
}

// infoRxLog emits a concise INFO-level RX log line.
// If packet is non-nil, it uses fields from the parsed packet (Type, Source, Callsign, Data).
// Otherwise it falls back to pktType, addr and dataLen provided by the caller.
func (s *Server) infoRxLog(pktType string, packet *Packet, addr *net.UDPAddr, dataLen int) {
	if packet != nil {
		// Use parsed packet information
		if s.logger != nil {
			fields := []logger.Field{
				logger.String("type", packet.Type),
				logger.String("source", packet.Source.String()),
				logger.Int("size", len(packet.Data)),
			}
			if packet.Callsign != "" {
				fields = append(fields, logger.String("callsign", packet.Callsign))
			}
			s.logger.Info("RX", fields...)
		}
		return
	}

	// Fallback when packet is not available
	if pktType == "" {
		pktType = "YSF"
	}
	if s.logger != nil {
		s.logger.Info("RX",
			logger.String("type", pktType),
			logger.String("from", addr.String()),
			logger.Int("size", dataLen))
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// hexdumpSideBySide returns a simple side-by-side hex + ASCII dump of data
func hexdumpSideBySide(b []byte) string {
	var sb strings.Builder
	const cols = 16
	for i := 0; i < len(b); i += cols {
		end := min(i+cols, len(b))
		chunk := b[i:end]

		// hex
		for j := 0; j < cols; j++ {
			if i+j < len(b) {
				sb.WriteString(fmt.Sprintf("%02x ", b[i+j]))
			} else {
				sb.WriteString("   ")
			}
		}

		sb.WriteString(" | ")

		// ascii
		for _, c := range chunk {
			if c >= 32 && c <= 126 {
				sb.WriteByte(c)
			} else {
				sb.WriteByte('.')
			}
		}

		sb.WriteString("\n")
	}
	return sb.String()
}

// GetListenAddress returns the UDP address the server is listening on
func (s *Server) GetListenAddress() *net.UDPAddr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.conn == nil {
		// If not started yet, construct address from host and port
		addr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", s.host, s.port))
		return addr
	}
	
	return s.conn.LocalAddr().(*net.UDPAddr)
}
