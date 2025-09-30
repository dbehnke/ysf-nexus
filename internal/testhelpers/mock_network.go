package testhelpers

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// MockUDPConn simulates a UDP connection for testing
type MockUDPConn struct {
	localAddr  *net.UDPAddr
	remoteAddr *net.UDPAddr
	packets    [][]byte
	responses  [][]byte
	closed     bool
	mu         sync.RWMutex
	
	// Channels for packet flow
	incomingPackets chan PacketData
	outgoingPackets chan PacketData
}

type PacketData struct {
	Data []byte
	Addr *net.UDPAddr
}

// NewMockUDPConn creates a new mock UDP connection
func NewMockUDPConn(local, remote string) (*MockUDPConn, error) {
	localAddr, err := net.ResolveUDPAddr("udp", local)
	if err != nil {
		return nil, err
	}
	
	remoteAddr, err := net.ResolveUDPAddr("udp", remote)
	if err != nil {
		return nil, err
	}
	
	return &MockUDPConn{
		localAddr:       localAddr,
		remoteAddr:      remoteAddr,
		packets:         make([][]byte, 0),
		responses:       make([][]byte, 0),
		incomingPackets: make(chan PacketData, 100),
		outgoingPackets: make(chan PacketData, 100),
	}, nil
}

// Read simulates reading from UDP connection
func (c *MockUDPConn) Read(b []byte) (int, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return 0, fmt.Errorf("connection closed")
	}
	c.mu.RUnlock()
	
	// Wait for incoming packet
	select {
	case packet := <-c.incomingPackets:
		n := copy(b, packet.Data)
		return n, nil
	case <-time.After(100 * time.Millisecond):
		return 0, fmt.Errorf("read timeout")
	}
}

// Write simulates writing to UDP connection
func (c *MockUDPConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.closed {
		return 0, fmt.Errorf("connection closed")
	}
	
	// Store the packet
	packet := make([]byte, len(b))
	copy(packet, b)
	c.packets = append(c.packets, packet)
	
	// Send to outgoing channel
	select {
	case c.outgoingPackets <- PacketData{Data: packet, Addr: c.remoteAddr}:
	default:
		// Channel full, drop packet (simulate network drop)
	}
	
	return len(b), nil
}

// ReadFromUDP simulates reading with address information
func (c *MockUDPConn) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return 0, nil, fmt.Errorf("connection closed")
	}
	c.mu.RUnlock()
	
	select {
	case packet := <-c.incomingPackets:
		n := copy(b, packet.Data)
		return n, packet.Addr, nil
	case <-time.After(100 * time.Millisecond):
		return 0, nil, fmt.Errorf("read timeout")
	}
}

// WriteToUDP simulates writing to a specific address
func (c *MockUDPConn) WriteToUDP(b []byte, addr *net.UDPAddr) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.closed {
		return 0, fmt.Errorf("connection closed")
	}
	
	packet := make([]byte, len(b))
	copy(packet, b)
	c.packets = append(c.packets, packet)
	
	// Send to outgoing channel with specific address
	select {
	case c.outgoingPackets <- PacketData{Data: packet, Addr: addr}:
	default:
	}
	
	return len(b), nil
}

// Close simulates closing the connection
func (c *MockUDPConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if !c.closed {
		c.closed = true
		close(c.incomingPackets)
		close(c.outgoingPackets)
	}
	return nil
}

// LocalAddr returns the local address
func (c *MockUDPConn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr returns the remote address
func (c *MockUDPConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// SetDeadline simulates setting deadlines
func (c *MockUDPConn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline simulates setting read deadline
func (c *MockUDPConn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline simulates setting write deadline
func (c *MockUDPConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// InjectPacket injects a packet as if it was received from the network
func (c *MockUDPConn) InjectPacket(data []byte, fromAddr *net.UDPAddr) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.closed {
		select {
		case c.incomingPackets <- PacketData{Data: data, Addr: fromAddr}:
		default:
		}
	}
}

// GetSentPackets returns all packets sent through this connection
func (c *MockUDPConn) GetSentPackets() [][]byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make([][]byte, len(c.packets))
	for i, packet := range c.packets {
		result[i] = make([]byte, len(packet))
		copy(result[i], packet)
	}
	return result
}

// GetOutgoingPackets returns a channel to monitor outgoing packets
func (c *MockUDPConn) GetOutgoingPackets() <-chan PacketData {
	return c.outgoingPackets
}

// ClearPackets clears the stored packets
func (c *MockUDPConn) ClearPackets() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.packets = c.packets[:0]
}

// MockUDPServer simulates a UDP server that can handle multiple connections
type MockUDPServer struct {
	addr        *net.UDPAddr
	connections map[string]*MockUDPConn
	packets     []PacketData
	running     bool
	mu          sync.RWMutex
	
	// Packet routing
	packetHandler func(data []byte, from *net.UDPAddr)
}

// NewMockUDPServer creates a new mock UDP server
func NewMockUDPServer(address string) (*MockUDPServer, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	
	return &MockUDPServer{
		addr:        addr,
		connections: make(map[string]*MockUDPConn),
		packets:     make([]PacketData, 0),
	}, nil
}

// Start starts the mock server
func (s *MockUDPServer) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
}

// Stop stops the mock server
func (s *MockUDPServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.running = false
	for _, conn := range s.connections {
		if err := conn.Close(); err != nil {
			// log without importing log at top-level to avoid change in function signatures
			fmt.Printf("mock udp server: failed to close connection: %v\n", err)
		}
	}
}

// AddConnection adds a mock connection to the server
func (s *MockUDPServer) AddConnection(remoteAddr string) (*MockUDPConn, error) {
	conn, err := NewMockUDPConn(s.addr.String(), remoteAddr)
	if err != nil {
		return nil, err
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.connections[remoteAddr] = conn
	return conn, nil
}

// SendPacketToConnection sends a packet to a specific connection
func (s *MockUDPServer) SendPacketToConnection(remoteAddr string, data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	conn, exists := s.connections[remoteAddr]
	if !exists {
		return fmt.Errorf("connection not found: %s", remoteAddr)
	}
	
	addr, _ := net.ResolveUDPAddr("udp", remoteAddr)
	conn.InjectPacket(data, addr)
	return nil
}

// SetPacketHandler sets a handler for incoming packets
func (s *MockUDPServer) SetPacketHandler(handler func(data []byte, from *net.UDPAddr)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packetHandler = handler
}

// BroadcastPacket sends a packet to all connected clients
func (s *MockUDPServer) BroadcastPacket(data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, conn := range s.connections {
		conn.InjectPacket(data, s.addr)
	}
}

// GetConnection returns a connection by remote address
func (s *MockUDPServer) GetConnection(remoteAddr string) *MockUDPConn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.connections[remoteAddr]
}

// GetAllConnections returns all connections
func (s *MockUDPServer) GetAllConnections() map[string]*MockUDPConn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make(map[string]*MockUDPConn)
	for k, v := range s.connections {
		result[k] = v
	}
	return result
}