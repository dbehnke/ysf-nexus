package dmr

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// MockDMRServer simulates a DMR network server for testing
type MockDMRServer struct {
	conn          *net.UDPConn
	addr          *net.UDPAddr
	clients       map[string]*net.UDPAddr
	authenticated map[string]bool
	salt          []byte
	stopChan      chan struct{}
	t             *testing.T
}

// NewMockDMRServer creates a new mock DMR server
func NewMockDMRServer(t *testing.T) *MockDMRServer {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}

	return &MockDMRServer{
		conn:          conn,
		addr:          conn.LocalAddr().(*net.UDPAddr),
		clients:       make(map[string]*net.UDPAddr),
		authenticated: make(map[string]bool),
		salt:          []byte("testsalt12345678"),
		stopChan:      make(chan struct{}),
		t:             t,
	}
}

// Start starts the mock server
func (s *MockDMRServer) Start(ctx context.Context) {
	go s.handlePackets(ctx)
}

// Stop stops the mock server
func (s *MockDMRServer) Stop() {
	close(s.stopChan)
	if err := s.conn.Close(); err != nil {
		s.t.Logf("Error closing mock server connection: %v", err)
	}
}

// GetAddress returns the server address
func (s *MockDMRServer) GetAddress() string {
	return s.addr.IP.String()
}

// GetPort returns the server port
func (s *MockDMRServer) GetPort() int {
	return s.addr.Port
}

// handlePackets handles incoming packets
func (s *MockDMRServer) handlePackets(ctx context.Context) {
	buffer := make([]byte, 1500)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		default:
			if err := s.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
				s.t.Logf("Failed to set read deadline: %v", err)
				continue
			}

			length, addr, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				s.t.Logf("Mock server read error: %v", err)
				continue
			}

			packet, err := ParsePacket(buffer[:length])
			if err != nil {
				s.t.Logf("Mock server parse error: %v", err)
				continue
			}

			s.handlePacket(packet, addr)
		}
	}
}

// handlePacket handles a specific packet type
func (s *MockDMRServer) handlePacket(packet *Packet, addr *net.UDPAddr) {
	clientKey := addr.String()

	switch packet.Type {
	case PacketTypeRPTL:
		// Login request - send RPTA with salt
		s.t.Log("Mock server: Received RPTL, sending RPTA with salt")
		s.clients[clientKey] = addr

		// Send RPTA with salt
		response := make([]byte, 8+len(s.salt))
		copy(response[0:4], PacketTypeRPTA)
		// Copy repeater ID from request
		copy(response[4:8], packet.Data[4:8])
		copy(response[8:], s.salt)

		if _, err := s.conn.WriteToUDP(response, addr); err != nil {
			s.t.Logf("Failed to write RPTA to UDP: %v", err)
		}

	case PacketTypeRPTK:
		// Password hash - send RPTA confirmation
		s.t.Log("Mock server: Received RPTK, sending RPTA confirmation")

		// Mark as authenticated
		s.authenticated[clientKey] = true

		// Send RPTA
		response := make([]byte, 8)
		copy(response[0:4], PacketTypeRPTA)
		copy(response[4:8], packet.Data[4:8])

		if _, err := s.conn.WriteToUDP(response, addr); err != nil {
			s.t.Logf("Failed to write RPTA confirmation to UDP: %v", err)
		}

	case PacketTypeRPTC:
		// Configuration - send final RPTA
		s.t.Log("Mock server: Received RPTC, sending final RPTA")

		// Send RPTA
		response := make([]byte, 8)
		copy(response[0:4], PacketTypeRPTA)
		copy(response[4:8], packet.Data[4:8])

		if _, err := s.conn.WriteToUDP(response, addr); err != nil {
			s.t.Logf("Failed to write final RPTA to UDP: %v", err)
		}

	case PacketTypeMSTP:
		// Ping response - acknowledged
		s.t.Log("Mock server: Received MSTP pong")

	case PacketTypeDMRD:
		// DMR data packet - echo back for testing
		s.t.Log("Mock server: Received DMRD, echoing back")
		if _, err := s.conn.WriteToUDP(packet.Data, addr); err != nil {
			s.t.Logf("Failed to echo DMRD to UDP: %v", err)
		}

	default:
		s.t.Logf("Mock server: Unknown packet type: %s", packet.Type)
	}
}

// SendPing sends a ping to a client
func (s *MockDMRServer) SendPing(clientAddr *net.UDPAddr, repeaterID uint32) error {
	packet := make([]byte, 11)
	copy(packet[0:4], PacketTypeMSTP)
	// RepeaterID at bytes 7-11
	packet[7] = byte(repeaterID >> 24)
	packet[8] = byte(repeaterID >> 16)
	packet[9] = byte(repeaterID >> 8)
	packet[10] = byte(repeaterID)

	_, err := s.conn.WriteToUDP(packet, clientAddr)
	return err
}

func TestNetworkAuthentication(t *testing.T) {
	// Create mock server
	server := NewMockDMRServer(t)
	defer func() {
		server.Stop()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server.Start(ctx)

	// Create network client
	log, _ := logger.New(logger.Config{Level: "debug", Format: "text"})
	config := Config{
		Address:      server.GetAddress(),
		Port:         server.GetPort(),
		RepeaterID:   1234567,
		Password:     "testpassword",
		Callsign:     "W1ABC",
		RXFreq:       446000000,
		TXFreq:       446000000,
		TXPower:      10,
		ColorCode:    1,
		Latitude:     40.7128,
		Longitude:    -74.0060,
		Height:       100,
		Location:     "Test Location",
		Description:  "Test Station",
		URL:          "https://example.com",
		Slot:         2,
		TalkGroup:    91,
		PingInterval: 1 * time.Second,
		AuthTimeout:  5 * time.Second,
	}

	network := NewNetwork(config, log)

	// Start network client (which triggers authentication)
	if err := network.Start(ctx); err != nil {
		t.Fatalf("Failed to start network: %v", err)
	}
	defer func() {
		if err := network.Stop(); err != nil {
			t.Logf("Error stopping network: %v", err)
		}
	}()

	// Verify authenticated
	if !network.IsAuthenticated() {
		t.Error("Network should be authenticated")
	}

	// Verify state
	if network.GetState() != StateRunning {
		t.Errorf("Expected state %s, got %s", StateRunning, network.GetState())
	}

	t.Log("Authentication successful!")
}

func TestNetworkSendVoicePackets(t *testing.T) {
	// Create mock server
	server := NewMockDMRServer(t)
	defer func() {
		server.Stop()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server.Start(ctx)

	// Create network client
	log, _ := logger.New(logger.Config{Level: "debug", Format: "text"})
	config := Config{
		Address:      server.GetAddress(),
		Port:         server.GetPort(),
		RepeaterID:   1234567,
		Password:     "testpassword",
		Callsign:     "W1ABC",
		ColorCode:    1,
		Slot:         2,
		TalkGroup:    91,
		PingInterval: 1 * time.Second,
		AuthTimeout:  5 * time.Second,
	}

	network := NewNetwork(config, log)

	if err := network.Start(ctx); err != nil {
		t.Fatalf("Failed to start network: %v", err)
	}
	defer func() {
		if err := network.Stop(); err != nil {
			t.Logf("Error stopping network: %v", err)
		}
	}()

	// Get stream ID
	streamID := network.GetStreamID()

	// Send voice header
	if err := network.SendVoiceHeader(1234567, 91, 2, CallTypeGroup, streamID); err != nil {
		t.Errorf("Failed to send voice header: %v", err)
	}

	// Send voice data
	voiceData := make([]byte, 33)
	for i := range voiceData {
		voiceData[i] = byte(i)
	}

	if err := network.SendVoiceData(1234567, 91, 2, CallTypeGroup, streamID, 1, voiceData); err != nil {
		t.Errorf("Failed to send voice data: %v", err)
	}

	// Send voice terminator
	if err := network.SendVoiceTerminator(1234567, 91, 2, CallTypeGroup, streamID, 2); err != nil {
		t.Errorf("Failed to send voice terminator: %v", err)
	}

	// Give packets time to be transmitted
	time.Sleep(100 * time.Millisecond)

	// Check statistics
	packetsRx, packetsTx, _, _ := network.GetStatistics()
	t.Logf("Packets RX: %d, TX: %d", packetsRx, packetsTx)

	// We expect at least authentication packets (RPTL, RPTK, RPTC) + voice packets (Header, Data, Terminator)
	// Note: The exact count may vary due to timing, so we just verify packets were sent
	if packetsTx < 3 { // At minimum the auth packets
		t.Errorf("Expected at least 3 packets transmitted, got %d", packetsTx)
	}

	t.Logf("Successfully sent voice packets")
}

func TestNetworkStreamIDGeneration(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "debug", Format: "text"})
	config := Config{
		Address:    "127.0.0.1",
		Port:       62030,
		RepeaterID: 1234567,
	}

	network := NewNetwork(config, log)

	// Generate multiple stream IDs
	ids := make(map[uint32]bool)
	for i := 0; i < 100; i++ {
		id := network.GetStreamID()
		if ids[id] {
			t.Errorf("Duplicate stream ID generated: %d", id)
		}
		ids[id] = true
	}

	if len(ids) != 100 {
		t.Errorf("Expected 100 unique stream IDs, got %d", len(ids))
	}
}

func TestNetworkStateTransitions(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "debug", Format: "text"})
	config := Config{
		Address:    "127.0.0.1",
		Port:       62030,
		RepeaterID: 1234567,
	}

	network := NewNetwork(config, log)

	// Initial state should be disconnected
	if network.GetState() != StateDisconnected {
		t.Errorf("Expected initial state %s, got %s", StateDisconnected, network.GetState())
	}
}

func BenchmarkSendVoiceData(b *testing.B) {
	// Create mock server
	server := NewMockDMRServer(&testing.T{})
	defer func() {
		server.Stop()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server.Start(ctx)

	// Create network client
	log, _ := logger.New(logger.Config{Level: "error", Format: "text"}) // Reduce logging for benchmark
	config := Config{
		Address:      server.GetAddress(),
		Port:         server.GetPort(),
		RepeaterID:   1234567,
		Password:     "testpassword",
		Callsign:     "W1ABC",
		ColorCode:    1,
		Slot:         2,
		TalkGroup:    91,
		PingInterval: 10 * time.Second,
		AuthTimeout:  5 * time.Second,
	}

	network := NewNetwork(config, log)
	if err := network.Start(ctx); err != nil {
		b.Fatalf("Failed to start network: %v", err)
	}
	defer func() {
		if err := network.Stop(); err != nil {
			b.Logf("Error stopping network: %v", err)
		}
	}()

	streamID := network.GetStreamID()
	voiceData := make([]byte, 33)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := network.SendVoiceData(1234567, 91, 2, CallTypeGroup, streamID, uint8(i%256), voiceData); err != nil {
			b.Fatalf("SendVoiceData failed in benchmark: %v", err)
		}
	}
}
