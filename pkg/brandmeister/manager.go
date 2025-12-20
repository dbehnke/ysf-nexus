package brandmeister

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// Config holds BrandMeister bridge configuration
type Config struct {
	MasterServer  string
	Callsign      string
	Password      string
	TargetTG      int
	DMRID         uint32
	HeartbeatFreq time.Duration
}

// PacketHandler is a callback for incoming data packets
type PacketHandler func(data []byte, source *net.UDPAddr)

// Status represents the current status of the BrandMeister connection
type Status struct {
	Connected bool
	State     State
}

// Manager manages the connection to BrandMeister
type Manager struct {
	config Config
	conn   *net.UDPConn
	fsm    *FSM

	handler PacketHandler

	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewManager creates a new BrandMeister manager
func NewManager(cfg Config) *Manager {
	if cfg.HeartbeatFreq == 0 {
		cfg.HeartbeatFreq = 10 * time.Second
	}

	m := &Manager{
		config:   cfg,
		stopChan: make(chan struct{}),
	}

	m.fsm = NewFSM(m, cfg.Callsign, cfg.Password, cfg.TargetTG)
	m.fsm.SetDMRID(cfg.DMRID)

	return m
}

// Start initiates the connection and handshake
func (m *Manager) Start(ctx context.Context) error {
	raddr, err := net.ResolveUDPAddr("udp", m.config.MasterServer)
	if err != nil {
		return fmt.Errorf("failed to resolve master server: %v", err)
	}

	// Use a dynamic local port to avoid conflicts with the reflector port
	laddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	if err != nil {
		return fmt.Errorf("failed to resolve local address: %v", err)
	}

	conn, err := net.DialUDP("udp", laddr, raddr)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	m.conn = conn

	m.wg.Add(2) // receiver and heartbeat
	go m.runReceiver()
	go m.runHeartbeat()

	return m.fsm.Start(ctx)
}

// Send sends raw data to the BrandMeister server
func (m *Manager) Send(data []byte) error {
	if m.conn == nil {
		return fmt.Errorf("connection not established")
	}
	_, err := m.conn.Write(data)
	return err
}

// runHeartbeat sends YSFP packets periodically
func (m *Manager) runHeartbeat() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.config.HeartbeatFreq)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if m.fsm.State == StateConnected {
				// Send YSFP (Ping)
				// Standard YSF Poll is 14 bytes: YSFP + 10 bytes "REFLECTOR "
				packet := make([]byte, 14)
				copy(packet[0:4], HeaderYSFP)
				copy(packet[4:14], "REFLECTOR ")
				_ = m.Send(packet)
			}
		case <-m.stopChan:
			return
		}
	}
}

// runReceiver listens for incoming packets from BrandMeister
func (m *Manager) runReceiver() {
	defer m.wg.Done()
	buf := make([]byte, 2048)

	for {
		select {
		case <-m.stopChan:
			return
		default:
			m.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, _, err := m.conn.ReadFromUDP(buf)
			if err != nil {
				if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
					continue
				}
				return
			}

			if n >= 4 {
				header := string(buf[:4])
				if header == HeaderYSFD {
					if m.handler != nil {
						m.handler(buf[:n], m.conn.RemoteAddr().(*net.UDPAddr))
					}
				} else {
					_ = m.fsm.HandlePacket(buf[:n])
				}
			}
		}
	}
}

// SetHandler sets the packet handler for inbound data
func (m *Manager) SetHandler(h PacketHandler) {
	m.handler = h
}

// IsBMAddress checks if the given address is the BrandMeister server address
func (m *Manager) IsBMAddress(addr *net.UDPAddr) bool {
	if m.conn == nil || addr == nil {
		return false
	}
	remote := m.conn.RemoteAddr().(*net.UDPAddr)
	return remote.IP.Equal(addr.IP) && remote.Port == addr.Port
}

// GetStatus returns the current status of the BrandMeister connection
func (m *Manager) GetStatus() Status {
	return Status{
		Connected: m.fsm.State == StateConnected,
		State:     m.fsm.State,
	}
}

// Stop stops the manager and its heartbeat loop
func (m *Manager) Stop() {
	close(m.stopChan)
	if m.conn != nil {
		m.conn.Close()
	}
	m.wg.Wait()
}
