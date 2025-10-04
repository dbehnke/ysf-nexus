package testhelpers

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// MockBridgeEndpoint simulates a YSF bridge endpoint for testing
type MockBridgeEndpoint struct {
	ID          string
	Name        string
	Description string
	RemoteAddr  string
	server      *MockUDPServer
	connection  *MockUDPConn
	isConnected bool

	// Talker simulation
	currentTalker *BridgeTalker
	talkerHistory []*BridgeTalker
	mu            sync.RWMutex

	// Event handlers
	onTalkerStart func(talker *BridgeTalker)
	onTalkerStop  func(talker *BridgeTalker)
	onPacket      func(packet []byte)

	// Statistics
	packetsReceived int
	packetsSent     int
	lastActivity    time.Time
}

// BridgeTalker represents a talker on the bridge
type BridgeTalker struct {
	Callsign    string    `json:"callsign"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time,omitempty"`
	Duration    int       `json:"duration"` // in seconds
	IsActive    bool      `json:"is_active"`
	BridgeID    string    `json:"bridge_id"`
	Location    string    `json:"location,omitempty"`
	PacketCount int       `json:"packet_count"`
}

// NewMockBridgeEndpoint creates a new mock bridge endpoint
func NewMockBridgeEndpoint(id, name, localAddr, remoteAddr string) (*MockBridgeEndpoint, error) {
	server, err := NewMockUDPServer(localAddr)
	if err != nil {
		return nil, err
	}

	conn, err := server.AddConnection(remoteAddr)
	if err != nil {
		return nil, err
	}

	return &MockBridgeEndpoint{
		ID:            id,
		Name:          name,
		RemoteAddr:    remoteAddr,
		server:        server,
		connection:    conn,
		talkerHistory: make([]*BridgeTalker, 0),
		lastActivity:  time.Now(),
	}, nil
}

// Connect simulates connecting to the bridge
func (b *MockBridgeEndpoint) Connect() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.isConnected = true
	b.server.Start()

	log.Printf("Mock Bridge %s (%s) connected to %s", b.ID, b.Name, b.RemoteAddr)
	return nil
}

// Disconnect simulates disconnecting from the bridge
func (b *MockBridgeEndpoint) Disconnect() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.currentTalker != nil && b.currentTalker.IsActive {
		b.stopCurrentTalker()
	}

	b.isConnected = false
	b.server.Stop()

	log.Printf("Mock Bridge %s disconnected", b.ID)
	return nil
}

// StartTalker simulates a new talker starting on the bridge
func (b *MockBridgeEndpoint) StartTalker(callsign, location string) (*BridgeTalker, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isConnected {
		return nil, fmt.Errorf("bridge not connected")
	}

	// Stop current talker if any
	if b.currentTalker != nil && b.currentTalker.IsActive {
		b.stopCurrentTalker()
	}

	// Create new talker
	talker := &BridgeTalker{
		Callsign:    callsign,
		StartTime:   time.Now(),
		IsActive:    true,
		BridgeID:    b.ID,
		Location:    location,
		PacketCount: 0,
	}

	b.currentTalker = talker
	b.lastActivity = time.Now()

	// Generate and send voice header packet
	packet := b.generateBridgeVoiceHeader(callsign)
	b.sendPacket(packet)

	if b.onTalkerStart != nil {
		b.onTalkerStart(talker)
	}

	log.Printf("Bridge %s: Talker %s started talking from %s", b.ID, callsign, location)
	return talker, nil
}

// StopCurrentTalker stops the current talker
func (b *MockBridgeEndpoint) StopCurrentTalker() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.currentTalker == nil || !b.currentTalker.IsActive {
		return fmt.Errorf("no active talker")
	}

	b.stopCurrentTalker()
	return nil
}

// stopCurrentTalker internal method (assumes mutex is held)
func (b *MockBridgeEndpoint) stopCurrentTalker() {
	if b.currentTalker != nil && b.currentTalker.IsActive {
		b.currentTalker.EndTime = time.Now()
		b.currentTalker.Duration = int(b.currentTalker.EndTime.Sub(b.currentTalker.StartTime).Seconds())
		b.currentTalker.IsActive = false

		// Generate and send voice terminator packet
		packet := b.generateBridgeVoiceTerminator(b.currentTalker.Callsign)
		b.sendPacket(packet)

		// Add to history
		b.talkerHistory = append(b.talkerHistory, b.currentTalker)

		if b.onTalkerStop != nil {
			b.onTalkerStop(b.currentTalker)
		}

		log.Printf("Bridge %s: Talker %s stopped talking (duration: %ds)",
			b.ID, b.currentTalker.Callsign, b.currentTalker.Duration)

		b.currentTalker = nil
	}
}

// SendVoicePackets simulates sending voice packets for the current talker
func (b *MockBridgeEndpoint) SendVoicePackets(count int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.currentTalker == nil || !b.currentTalker.IsActive {
		return fmt.Errorf("no active talker")
	}

	for i := 0; i < count; i++ {
		packet := b.generateBridgeVoicePacket(b.currentTalker.Callsign, b.currentTalker.PacketCount)
		b.sendPacket(packet)
		b.currentTalker.PacketCount++

		time.Sleep(20 * time.Millisecond) // YSF frame rate
	}

	return nil
}

// GetCurrentTalker returns the current active talker
func (b *MockBridgeEndpoint) GetCurrentTalker() *BridgeTalker {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.currentTalker != nil && b.currentTalker.IsActive {
		// Return a copy with updated duration
		talker := *b.currentTalker
		talker.Duration = int(time.Since(talker.StartTime).Seconds())
		return &talker
	}

	return nil
}

// GetTalkerHistory returns the history of talkers
func (b *MockBridgeEndpoint) GetTalkerHistory(limit int) []*BridgeTalker {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 || limit > len(b.talkerHistory) {
		limit = len(b.talkerHistory)
	}

	// Return most recent entries in reverse chronological order (most recent first)
	history := make([]*BridgeTalker, limit)
	for i := 0; i < limit; i++ {
		// take from the end of the slice backwards
		history[i] = b.talkerHistory[len(b.talkerHistory)-1-i]
	}

	return history
}

// GetStatus returns the current status of the bridge
func (b *MockBridgeEndpoint) GetStatus() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var currentTalkerInfo map[string]interface{}
	if b.currentTalker != nil && b.currentTalker.IsActive {
		currentTalkerInfo = map[string]interface{}{
			"callsign":     b.currentTalker.Callsign,
			"start_time":   b.currentTalker.StartTime,
			"duration":     int(time.Since(b.currentTalker.StartTime).Seconds()),
			"location":     b.currentTalker.Location,
			"packet_count": b.currentTalker.PacketCount,
		}
	}

	return map[string]interface{}{
		"id":               b.ID,
		"name":             b.Name,
		"description":      b.Description,
		"remote_addr":      b.RemoteAddr,
		"is_connected":     b.isConnected,
		"current_talker":   currentTalkerInfo,
		"packets_received": b.packetsReceived,
		"packets_sent":     b.packetsSent,
		"last_activity":    b.lastActivity,
		"talker_count":     len(b.talkerHistory),
	}
}

// SetEventHandlers sets event handlers for the bridge
func (b *MockBridgeEndpoint) SetEventHandlers(
	onTalkerStart func(talker *BridgeTalker),
	onTalkerStop func(talker *BridgeTalker),
	onPacket func(packet []byte),
) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.onTalkerStart = onTalkerStart
	b.onTalkerStop = onTalkerStop
	b.onPacket = onPacket
}

// SimulateTalkingSequence simulates a complete talking sequence
func (b *MockBridgeEndpoint) SimulateTalkingSequence(callsign, location string, duration time.Duration) {
	go func() {
		_, err := b.StartTalker(callsign, location)
		if err != nil {
			log.Printf("Failed to start talker: %v", err)
			return
		}

		// Send voice packets during the talking period
		packetInterval := 20 * time.Millisecond
		totalPackets := int(duration / packetInterval)

		for i := 0; i < totalPackets; i++ {
			if err := b.SendVoicePackets(1); err != nil {
				log.Printf("Failed to send voice packet: %v", err)
				break
			}
		}

		if err := b.StopCurrentTalker(); err != nil {
			log.Printf("mock bridge: StopCurrentTalker returned error: %v", err)
		}
	}()
}

// SimulateRandomActivity generates random bridge activity
func (b *MockBridgeEndpoint) SimulateRandomActivity(duration time.Duration, callsigns []string) {
	if len(callsigns) == 0 {
		callsigns = []string{"W9TRO", "G0RDH", "VK2ABC", "JA1XYZ", "DL1ABC"}
	}

	go func() {
		end := time.Now().Add(duration)

		for time.Now().Before(end) {
			// Random pause between activities (5-30 seconds)
			pause := time.Duration(5+rand.Intn(25)) * time.Second
			time.Sleep(pause)

			if time.Now().After(end) {
				break
			}

			// Pick random callsign and talking duration
			callsign := callsigns[rand.Intn(len(callsigns))]
			talkDuration := time.Duration(2+rand.Intn(8)) * time.Second
			location := fmt.Sprintf("Location-%d", rand.Intn(100))

			b.SimulateTalkingSequence(callsign, location, talkDuration)

			// Wait for the talking to finish before next activity
			time.Sleep(talkDuration + time.Second)
		}
	}()
}

// Helper methods for packet generation
func (b *MockBridgeEndpoint) generateBridgeVoiceHeader(callsign string) []byte {
	packet := make([]byte, 155)

	// Bridge packet header
	copy(packet[0:4], "BRDG")
	packet[4] = 0x00 // Voice header type

	// Source callsign
	copy(packet[14:24], fmt.Sprintf("%-10s", callsign))

	// Bridge ID
	copy(packet[24:34], fmt.Sprintf("%-10s", b.ID))

	// Random data
	for i := 34; i < len(packet); i++ {
		packet[i] = byte(rand.Intn(256))
	}

	return packet
}

func (b *MockBridgeEndpoint) generateBridgeVoicePacket(callsign string, sequence int) []byte {
	packet := make([]byte, 155)

	copy(packet[0:4], "BRDG")
	packet[4] = 0x01 // Voice frame
	packet[5] = byte(sequence & 0x7F)

	copy(packet[14:24], fmt.Sprintf("%-10s", callsign))
	copy(packet[24:34], fmt.Sprintf("%-10s", b.ID))

	// Random voice data
	for i := 34; i < len(packet); i++ {
		packet[i] = byte(rand.Intn(256))
	}

	return packet
}

func (b *MockBridgeEndpoint) generateBridgeVoiceTerminator(callsign string) []byte {
	packet := make([]byte, 155)

	copy(packet[0:4], "BRDG")
	packet[4] = 0x02 // Voice terminator

	copy(packet[14:24], fmt.Sprintf("%-10s", callsign))
	copy(packet[24:34], fmt.Sprintf("%-10s", b.ID))

	return packet
}

func (b *MockBridgeEndpoint) sendPacket(packet []byte) {
	b.lastActivity = time.Now()

	if b.onPacket != nil {
		b.onPacket(packet)
	}

	// Send through the connection and only count on success
	if _, err := b.connection.Write(packet); err != nil {
		log.Printf("mock bridge: failed to write packet: %v", err)
	} else {
		b.packetsSent++
	}
}

// MockBridgeNetwork manages multiple bridge endpoints
type MockBridgeNetwork struct {
	bridges map[string]*MockBridgeEndpoint
	mu      sync.RWMutex

	// Global event handlers
	onTalkerEvent func(bridgeID string, talker *BridgeTalker, event string)
}

// NewMockBridgeNetwork creates a new bridge network
func NewMockBridgeNetwork() *MockBridgeNetwork {
	return &MockBridgeNetwork{
		bridges: make(map[string]*MockBridgeEndpoint),
	}
}

// AddBridge adds a bridge to the network
func (n *MockBridgeNetwork) AddBridge(bridge *MockBridgeEndpoint) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Set up event forwarding
	bridge.SetEventHandlers(
		func(talker *BridgeTalker) {
			if n.onTalkerEvent != nil {
				n.onTalkerEvent(bridge.ID, talker, "start")
			}
		},
		func(talker *BridgeTalker) {
			if n.onTalkerEvent != nil {
				n.onTalkerEvent(bridge.ID, talker, "stop")
			}
		},
		nil,
	)

	n.bridges[bridge.ID] = bridge
}

// GetBridge returns a bridge by ID
func (n *MockBridgeNetwork) GetBridge(id string) *MockBridgeEndpoint {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.bridges[id]
}

// ConnectAll connects all bridges in the network
func (n *MockBridgeNetwork) ConnectAll() error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, bridge := range n.bridges {
		if err := bridge.Connect(); err != nil {
			return err
		}
	}

	return nil
}

// DisconnectAll disconnects all bridges in the network
func (n *MockBridgeNetwork) DisconnectAll() {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, bridge := range n.bridges {
		if err := bridge.Disconnect(); err != nil {
			log.Printf("mock bridge network: failed to disconnect bridge %s: %v", bridge.ID, err)
		}
	}
}

// GetCurrentTalkers returns all current active talkers across all bridges
func (n *MockBridgeNetwork) GetCurrentTalkers() map[string]*BridgeTalker {
	n.mu.RLock()
	defer n.mu.RUnlock()

	talkers := make(map[string]*BridgeTalker)
	for id, bridge := range n.bridges {
		if talker := bridge.GetCurrentTalker(); talker != nil {
			talkers[id] = talker
		}
	}

	return talkers
}

// GetNetworkStatus returns status of all bridges
func (n *MockBridgeNetwork) GetNetworkStatus() map[string]interface{} {
	n.mu.RLock()
	defer n.mu.RUnlock()

	status := make(map[string]interface{})
	for id, bridge := range n.bridges {
		status[id] = bridge.GetStatus()
	}

	return status
}

// SetTalkerEventHandler sets a global talker event handler
func (n *MockBridgeNetwork) SetTalkerEventHandler(handler func(bridgeID string, talker *BridgeTalker, event string)) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.onTalkerEvent = handler
}

// StartRandomActivityOnAllBridges starts random activity on all bridges
func (n *MockBridgeNetwork) StartRandomActivityOnAllBridges(duration time.Duration) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	callsigns := []string{"W9TRO", "G0RDH", "VK2ABC", "JA1XYZ", "DL1ABC", "K5ABC", "M0XYZ", "PY2DEF"}

	for _, bridge := range n.bridges {
		// Randomize callsigns for each bridge
		bridgeCallsigns := make([]string, 3+rand.Intn(3))
		for i := range bridgeCallsigns {
			bridgeCallsigns[i] = callsigns[rand.Intn(len(callsigns))]
		}

		bridge.SimulateRandomActivity(duration, bridgeCallsigns)
	}
}
