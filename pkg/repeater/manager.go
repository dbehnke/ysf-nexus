package repeater

import (
	"context"
	"log"
	"net"
	"sync"
	"time"
)

// Manager manages multiple YSF repeaters
type Manager struct {
	repeaters   sync.Map
	timeout     time.Duration
	blocklist   *Blocklist
	events      chan<- Event
	maxRepeaters int
	mu          sync.RWMutex
	metrics     ManagerMetrics
}

// ManagerMetrics holds manager statistics
type ManagerMetrics struct {
	TotalConnections    uint64
	ActiveConnections   uint64
	BlockedConnections  uint64
	TimeoutConnections  uint64
	TotalPackets        uint64
	TotalBytesRx        uint64
	TotalBytesTx        uint64
}

// Event represents a repeater event
type Event struct {
	Type      string        `json:"type"`
	Callsign  string        `json:"callsign"`
	Address   string        `json:"address"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// Event types
const (
	EventConnect    = "connect"
	EventDisconnect = "disconnect"
	EventTalkStart  = "talk_start"
	EventTalkEnd    = "talk_end"
	EventTimeout    = "timeout"
	EventBlocked    = "blocked"
)

// NewManager creates a new repeater manager
func NewManager(timeout time.Duration, maxRepeaters int, eventChan chan<- Event) *Manager {
	return &Manager{
		timeout:      timeout,
		maxRepeaters: maxRepeaters,
		events:       eventChan,
		blocklist:    NewBlocklist(),
	}
}

// AddRepeater adds or updates a repeater
func (m *Manager) AddRepeater(callsign string, addr *net.UDPAddr) (*Repeater, bool) {
	// Check blocklist
	if m.blocklist.IsBlocked(callsign) {
		m.metrics.BlockedConnections++
		m.sendEvent(EventBlocked, callsign, addr.String(), 0)
		return nil, false
	}

	key := addr.String()

	if existing, ok := m.repeaters.Load(key); ok {
		repeater := existing.(*Repeater)
		repeater.UpdateLastSeen()
		return repeater, false // Existing repeater
	}

	// Check max connections
	count := m.Count()
	if count >= m.maxRepeaters {
		log.Printf("Maximum repeater limit reached (%d), rejecting %s", m.maxRepeaters, callsign)
		return nil, false
	}

	// Create new repeater
	repeater := NewRepeater(callsign, addr)
	m.repeaters.Store(key, repeater)

	m.mu.Lock()
	m.metrics.TotalConnections++
	m.metrics.ActiveConnections++
	m.mu.Unlock()

	m.sendEvent(EventConnect, callsign, addr.String(), 0)
	log.Printf("New repeater connected: %s from %s", callsign, addr)

	return repeater, true // New repeater
}

// GetRepeater retrieves a repeater by address
func (m *Manager) GetRepeater(addr *net.UDPAddr) *Repeater {
	if repeater, ok := m.repeaters.Load(addr.String()); ok {
		return repeater.(*Repeater)
	}
	return nil
}

// RemoveRepeater removes a repeater
func (m *Manager) RemoveRepeater(addr *net.UDPAddr) bool {
	key := addr.String()
	if repeater, ok := m.repeaters.LoadAndDelete(key); ok {
		r := repeater.(*Repeater)

		// Stop talking if active
		if r.IsTalking() {
			duration := r.StopTalking()
			m.sendEvent(EventTalkEnd, r.Callsign(), addr.String(), duration)
		}

		m.mu.Lock()
		m.metrics.ActiveConnections--
		m.mu.Unlock()

		m.sendEvent(EventDisconnect, r.Callsign(), addr.String(), 0)
		log.Printf("Repeater disconnected: %s from %s (uptime: %v)",
			r.Callsign(), addr, r.Uptime())

		return true
	}
	return false
}

// GetAllRepeaters returns all active repeaters
func (m *Manager) GetAllRepeaters() []*Repeater {
	var repeaters []*Repeater
	m.repeaters.Range(func(key, value interface{}) bool {
		if repeater, ok := value.(*Repeater); ok {
			repeaters = append(repeaters, repeater)
		}
		return true
	})
	return repeaters
}

// GetAllAddresses returns all repeater addresses
func (m *Manager) GetAllAddresses() []*net.UDPAddr {
	var addresses []*net.UDPAddr
	m.repeaters.Range(func(key, value interface{}) bool {
		if repeater, ok := value.(*Repeater); ok {
			addresses = append(addresses, repeater.Address())
		}
		return true
	})
	return addresses
}

// Count returns the number of active repeaters
func (m *Manager) Count() int {
	count := 0
	m.repeaters.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// ProcessPacket processes a packet and updates repeater state
func (m *Manager) ProcessPacket(callsign string, addr *net.UDPAddr, packetType string, dataSize int) {
	repeater := m.GetRepeater(addr)
	if repeater == nil {
		return
	}

	repeater.UpdateLastSeen()
	repeater.IncrementPacketCount()
	repeater.AddBytesReceived(uint64(dataSize))

	m.mu.Lock()
	m.metrics.TotalPackets++
	m.metrics.TotalBytesRx += uint64(dataSize)
	m.mu.Unlock()

	// Handle talk state changes for data packets
	if packetType == "YSFD" {
		if !repeater.IsTalking() {
			repeater.StartTalking()
			m.sendEvent(EventTalkStart, callsign, addr.String(), 0)
			log.Printf("Repeater %s started talking", callsign)
		} else {
			// Update talk data timestamp for ongoing transmission
			repeater.UpdateTalkData()
		}
	}
}

// ProcessTransmit updates transmit statistics
func (m *Manager) ProcessTransmit(addr *net.UDPAddr, dataSize int) {
	repeater := m.GetRepeater(addr)
	if repeater != nil {
		repeater.AddBytesTransmitted(uint64(dataSize))
	}

	m.mu.Lock()
	m.metrics.TotalBytesTx += uint64(dataSize)
	m.mu.Unlock()
}

// StartCleanup starts the cleanup goroutine for timed-out repeaters
func (m *Manager) StartCleanup(ctx context.Context) {
	// Cleanup timed-out repeaters every 30 seconds
	cleanupTicker := time.NewTicker(30 * time.Second)
	defer cleanupTicker.Stop()

	// Check for talk timeouts every 2 seconds for responsiveness
	talkTicker := time.NewTicker(2 * time.Second)
	defer talkTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanupTicker.C:
			m.cleanupTimedOut()
		case <-talkTicker.C:
			m.checkTalkTimeouts()
		}
	}
}

// cleanupTimedOut removes timed-out repeaters
func (m *Manager) cleanupTimedOut() {
	var toRemove []*net.UDPAddr

	m.repeaters.Range(func(key, value interface{}) bool {
		repeater := value.(*Repeater)
		if repeater.IsTimedOut(m.timeout) {
			toRemove = append(toRemove, repeater.Address())
		}
		return true
	})

	for _, addr := range toRemove {
		if repeater := m.GetRepeater(addr); repeater != nil {
			// Handle ongoing talk
			if repeater.IsTalking() {
				duration := repeater.StopTalking()
				m.sendEvent(EventTalkEnd, repeater.Callsign(), addr.String(), duration)
			}

			m.mu.Lock()
			m.metrics.TimeoutConnections++
			m.mu.Unlock()

			m.sendEvent(EventTimeout, repeater.Callsign(), addr.String(), 0)
		}
		m.RemoveRepeater(addr)
	}

	if len(toRemove) > 0 {
		log.Printf("Cleaned up %d timed-out repeaters", len(toRemove))
	}
}

// checkTalkTimeouts checks for and handles talk session timeouts
func (m *Manager) checkTalkTimeouts() {
	// Talk timeout duration (3 seconds without data packets)
	talkTimeout := 3 * time.Second

	m.repeaters.Range(func(key, value interface{}) bool {
		repeater := value.(*Repeater)
		if repeater.IsTalkTimedOut(talkTimeout) {
			duration := repeater.StopTalking()
			m.sendEvent(EventTalkEnd, repeater.Callsign(), repeater.Address().String(), duration)
			log.Printf("Repeater %s stopped talking (timeout after %v)", repeater.Callsign(), duration)
		}
		return true
	})
}

// GetStats returns current manager statistics
func (m *Manager) GetStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var repeaterStats []RepeaterStats
	m.repeaters.Range(func(key, value interface{}) bool {
		if repeater, ok := value.(*Repeater); ok {
			repeaterStats = append(repeaterStats, repeater.Stats())
		}
		return true
	})

	return ManagerStats{
		ActiveRepeaters:     len(repeaterStats),
		TotalConnections:    m.metrics.TotalConnections,
		BlockedConnections:  m.metrics.BlockedConnections,
		TimeoutConnections:  m.metrics.TimeoutConnections,
		TotalPackets:        m.metrics.TotalPackets,
		TotalBytesReceived:  m.metrics.TotalBytesRx,
		TotalBytesTransmitted: m.metrics.TotalBytesTx,
		Repeaters:           repeaterStats,
	}
}

// ManagerStats represents manager statistics
type ManagerStats struct {
	ActiveRepeaters       int             `json:"active_repeaters"`
	TotalConnections      uint64          `json:"total_connections"`
	BlockedConnections    uint64          `json:"blocked_connections"`
	TimeoutConnections    uint64          `json:"timeout_connections"`
	TotalPackets          uint64          `json:"total_packets"`
	TotalBytesReceived    uint64          `json:"total_bytes_received"`
	TotalBytesTransmitted uint64          `json:"total_bytes_transmitted"`
	Repeaters             []RepeaterStats `json:"repeaters"`
}

// GetBlocklist returns the blocklist
func (m *Manager) GetBlocklist() *Blocklist {
	return m.blocklist
}

// DumpRepeaters logs all current repeaters
func (m *Manager) DumpRepeaters() {
	repeaters := m.GetAllRepeaters()
	log.Printf("=== Repeater Dump ===")
	log.Printf("Active repeaters: %d", len(repeaters))

	for _, repeater := range repeaters {
		log.Printf("  %s", repeater.String())
	}
	log.Printf("=== End Dump ===")
}

// sendEvent sends an event to the event channel
func (m *Manager) sendEvent(eventType, callsign, address string, duration time.Duration) {
	if m.events == nil {
		return
	}

	event := Event{
		Type:      eventType,
		Callsign:  callsign,
		Address:   address,
		Timestamp: time.Now(),
		Duration:  duration,
	}

	select {
	case m.events <- event:
	default:
		// Don't block if event channel is full
		log.Printf("Event channel full, dropping event: %+v", event)
	}
}