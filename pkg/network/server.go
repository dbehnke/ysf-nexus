package network

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
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
func NewServer(host string, port int) *Server {
	return &Server{
		host:     host,
		port:     port,
		handlers: make(map[string]PacketHandler),
		metrics: &Metrics{
			PacketsReceived: make(map[string]int64),
			PacketsSent:     make(map[string]int64),
			Uptime:          time.Now(),
		},
	}
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

	log.Printf("YSF server listening on %s:%d", s.host, s.port)

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
			s.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			n, addr, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Timeout is expected, continue
				}
				if s.isRunning() {
					log.Printf("Error reading UDP packet: %v", err)
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

	if s.debug {
		log.Printf("Received %d bytes from %s: %x", len(data), addr, data[:min(len(data), 16)])
	}

	// Parse packet
	packet, err := ParsePacket(data, addr)
	if err != nil {
		if s.debug {
			log.Printf("Failed to parse packet from %s: %v", addr, err)
		}
		return
	}

	if s.debug {
		log.Printf("Parsed packet: %s", packet.String())
	}

	// Find handler for packet type
	s.mu.RLock()
	handler, exists := s.handlers[packet.Type]
	s.mu.RUnlock()

	if !exists {
		if s.debug {
			log.Printf("No handler for packet type: %s", packet.Type)
		}
		return
	}

	// Call handler
	if err := handler(packet); err != nil {
		log.Printf("Handler error for packet type %s: %v", packet.Type, err)
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

	if s.debug {
		log.Printf("Sent %d bytes to %s: %x", n, addr, data[:min(len(data), 16)])
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
			log.Printf("Failed to send packet to %s: %v", addr, err)
			continue
		}
		sent++
	}

	if s.debug {
		log.Printf("Broadcasted to %d addresses (excluded %v)", sent, exclude)
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

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}