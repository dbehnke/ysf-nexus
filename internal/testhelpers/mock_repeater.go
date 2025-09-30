package testhelpers

import (
	"fmt"
	"log"
	"sync"
	"time"
	"math/rand"
)

// MockYSFRepeater simulates a YSF repeater for testing
type MockYSFRepeater struct {
	ID           string
	Name         string
	Description  string
	Frequency    string
	Location     string
	server       *MockUDPServer
	clients      map[string]*MockRepeaterClient
	isLinked     bool
	linkTarget   string
	mu           sync.RWMutex
	
	// Packet simulation
	packetCount  int
	lastActivity time.Time
	
	// Event handlers
	onPacketReceived func(packet []byte, from string)
	onClientConnect  func(clientID string)
	onClientDisconnect func(clientID string)
}

// MockRepeaterClient represents a client connected to the mock repeater
type MockRepeaterClient struct {
	ID        string
	Callsign  string
	Location  string
	conn      *MockUDPConn
	lastSeen  time.Time
	isTalking bool
	talkStart time.Time
}

// NewMockYSFRepeater creates a new mock YSF repeater
func NewMockYSFRepeater(id, name, address string) (*MockYSFRepeater, error) {
	server, err := NewMockUDPServer(address)
	if err != nil {
		return nil, err
	}
	
	return &MockYSFRepeater{
		ID:          id,
		Name:        name,
		server:      server,
		clients:     make(map[string]*MockRepeaterClient),
		lastActivity: time.Now(),
	}, nil
}

// Start starts the mock repeater
func (r *MockYSFRepeater) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.server.Start()
	log.Printf("Mock YSF Repeater %s (%s) started", r.ID, r.Name)
	return nil
}

// Stop stops the mock repeater
func (r *MockYSFRepeater) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.server.Stop()
	log.Printf("Mock YSF Repeater %s (%s) stopped", r.ID, r.Name)
	return nil
}

// ConnectClient connects a mock client to the repeater
func (r *MockYSFRepeater) ConnectClient(callsign, location, address string) (*MockRepeaterClient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	conn, err := r.server.AddConnection(address)
	if err != nil {
		return nil, err
	}
	
	client := &MockRepeaterClient{
		ID:       fmt.Sprintf("%s-%s", callsign, address),
		Callsign: callsign,
		Location: location,
		conn:     conn,
		lastSeen: time.Now(),
	}
	
	r.clients[client.ID] = client
	
	if r.onClientConnect != nil {
		r.onClientConnect(client.ID)
	}
	
	log.Printf("Client %s connected to repeater %s", callsign, r.ID)
	return client, nil
}

// DisconnectClient disconnects a client from the repeater
func (r *MockYSFRepeater) DisconnectClient(clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	client, exists := r.clients[clientID]
	if !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}
	
	if err := client.conn.Close(); err != nil {
		log.Printf("mock repeater: failed to close client conn: %v", err)
	}
	delete(r.clients, clientID)
	
	if r.onClientDisconnect != nil {
		r.onClientDisconnect(clientID)
	}
	
	log.Printf("Client %s disconnected from repeater %s", client.Callsign, r.ID)
	return nil
}

// StartTalking simulates a client starting to talk
func (r *MockYSFRepeater) StartTalking(clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	client, exists := r.clients[clientID]
	if !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}
	
	client.isTalking = true
	client.talkStart = time.Now()
	r.lastActivity = time.Now()
	
	// Generate YSF voice header packet
	packet := r.generateYSFVoiceHeader(client.Callsign)
	r.broadcastPacket(packet, clientID)
	
	log.Printf("Client %s started talking on repeater %s", client.Callsign, r.ID)
	return nil
}

// StopTalking simulates a client stopping talking
func (r *MockYSFRepeater) StopTalking(clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	client, exists := r.clients[clientID]
	if !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}
	
	if !client.isTalking {
		return fmt.Errorf("client %s is not talking", clientID)
	}
	
	duration := time.Since(client.talkStart)
	client.isTalking = false
	r.lastActivity = time.Now()
	
	// Generate YSF voice terminator packet
	packet := r.generateYSFVoiceTerminator(client.Callsign)
	r.broadcastPacket(packet, clientID)
	
	log.Printf("Client %s stopped talking on repeater %s (duration: %v)", client.Callsign, r.ID, duration)
	return nil
}

// SendVoicePackets simulates sending voice packets while talking
func (r *MockYSFRepeater) SendVoicePackets(clientID string, count int) error {
	r.mu.RLock()
	client, exists := r.clients[clientID]
	if !exists || !client.isTalking {
		r.mu.RUnlock()
		return fmt.Errorf("client not talking: %s", clientID)
	}
	r.mu.RUnlock()
	
	for i := 0; i < count; i++ {
		packet := r.generateYSFVoicePacket(client.Callsign, i)
		r.broadcastPacket(packet, clientID)
		time.Sleep(20 * time.Millisecond) // YSF frame rate
	}
	
	return nil
}

// LinkToRepeater simulates linking to another repeater/reflector
func (r *MockYSFRepeater) LinkToRepeater(target string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.isLinked = true
	r.linkTarget = target
	
	log.Printf("Repeater %s linked to %s", r.ID, target)
	return nil
}

// UnlinkFromRepeater simulates unlinking from repeater/reflector
func (r *MockYSFRepeater) UnlinkFromRepeater() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.isLinked = false
	r.linkTarget = ""
	
	log.Printf("Repeater %s unlinked", r.ID)
	return nil
}

// GetStatus returns the current status of the repeater
func (r *MockYSFRepeater) GetStatus() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	clientList := make([]map[string]interface{}, 0, len(r.clients))
	for _, client := range r.clients {
		clientList = append(clientList, map[string]interface{}{
			"id":        client.ID,
			"callsign":  client.Callsign,
			"location":  client.Location,
			"lastSeen":  client.lastSeen,
			"isTalking": client.isTalking,
		})
	}
	
	return map[string]interface{}{
		"id":          r.ID,
		"name":        r.Name,
		"description": r.Description,
		"frequency":   r.Frequency,
		"location":    r.Location,
		"isLinked":    r.isLinked,
		"linkTarget":  r.linkTarget,
		"clients":     clientList,
		"packetCount": r.packetCount,
		"lastActivity": r.lastActivity,
	}
}

// SetEventHandlers sets event handlers for the repeater
func (r *MockYSFRepeater) SetEventHandlers(
	onPacket func(packet []byte, from string),
	onConnect func(clientID string),
	onDisconnect func(clientID string),
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.onPacketReceived = onPacket
	r.onClientConnect = onConnect
	r.onClientDisconnect = onDisconnect
}

// SimulateRandomActivity generates random activity on the repeater
func (r *MockYSFRepeater) SimulateRandomActivity(duration time.Duration) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		
		end := time.Now().Add(duration)
		
		for time.Now().Before(end) {
			<-ticker.C
			r.mu.RLock()
			clientCount := len(r.clients)
			r.mu.RUnlock()

			if clientCount > 0 && rand.Float64() < 0.1 { // 10% chance per second
				// Pick a random client to start talking
				r.mu.RLock()
				var clientIDs []string
				for id := range r.clients {
					if !r.clients[id].isTalking {
						clientIDs = append(clientIDs, id)
					}
				}
				r.mu.RUnlock()

				if len(clientIDs) > 0 {
					clientID := clientIDs[rand.Intn(len(clientIDs))]
					if err := r.StartTalking(clientID); err != nil {
						log.Printf("mock repeater: failed to start talking: %v", err)
					}
					// Talk for 2-10 seconds
					talkDuration := time.Duration(2+rand.Intn(8)) * time.Second
					go func(cID string, dur time.Duration) {
						time.Sleep(dur)
						if err := r.StopTalking(cID); err != nil {
							log.Printf("mock repeater: failed to stop talking: %v", err)
						}
					}(clientID, talkDuration)
				}
			}
		}
	}()
}

// Helper methods for packet generation
func (r *MockYSFRepeater) generateYSFVoiceHeader(callsign string) []byte {
	// Simplified YSF voice header packet structure
	packet := make([]byte, 155) // YSF packet size
	
	// YSF header
	copy(packet[0:4], "YSFD")
	packet[4] = 0x00 // Voice/data type
	
	// Source callsign (10 bytes)
	copy(packet[14:24], fmt.Sprintf("%-10s", callsign))
	
	// Destination (10 bytes) 
	copy(packet[24:34], fmt.Sprintf("%-10s", "ALL"))
	
	// Add some random voice data
	for i := 34; i < len(packet); i++ {
		packet[i] = byte(rand.Intn(256))
	}
	
	return packet
}

func (r *MockYSFRepeater) generateYSFVoicePacket(callsign string, sequence int) []byte {
	packet := make([]byte, 155)
	
	copy(packet[0:4], "YSFD")
	packet[4] = 0x01 // Voice frame
	packet[5] = byte(sequence & 0x7F) // Frame sequence
	
	copy(packet[14:24], fmt.Sprintf("%-10s", callsign))
	copy(packet[24:34], fmt.Sprintf("%-10s", "ALL"))
	
	// Random voice data
	for i := 34; i < len(packet); i++ {
		packet[i] = byte(rand.Intn(256))
	}
	
	return packet
}

func (r *MockYSFRepeater) generateYSFVoiceTerminator(callsign string) []byte {
	packet := make([]byte, 155)
	
	copy(packet[0:4], "YSFD")
	packet[4] = 0x02 // Voice terminator
	
	copy(packet[14:24], fmt.Sprintf("%-10s", callsign))
	copy(packet[24:34], fmt.Sprintf("%-10s", "ALL"))
	
	return packet
}

func (r *MockYSFRepeater) broadcastPacket(packet []byte, excludeClient string) {
	r.packetCount++
	
	if r.onPacketReceived != nil {
		r.onPacketReceived(packet, excludeClient)
	}
	
	// Broadcast to all connected clients except sender
	for id, client := range r.clients {
		if id != excludeClient {
			client.conn.InjectPacket(packet, r.server.addr)
		}
	}
	
	// If linked, also forward to link target
	if r.isLinked && r.linkTarget != "" {
		// This would forward to the linked repeater/reflector
		log.Printf("Forwarding packet from %s to linked target %s", excludeClient, r.linkTarget)
	}
}

// MockYSFRepeaterNetwork manages multiple mock repeaters
type MockYSFRepeaterNetwork struct {
	repeaters map[string]*MockYSFRepeater
	mu        sync.RWMutex
}

// NewMockYSFRepeaterNetwork creates a new repeater network
func NewMockYSFRepeaterNetwork() *MockYSFRepeaterNetwork {
	return &MockYSFRepeaterNetwork{
		repeaters: make(map[string]*MockYSFRepeater),
	}
}

// AddRepeater adds a repeater to the network
func (n *MockYSFRepeaterNetwork) AddRepeater(repeater *MockYSFRepeater) {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	n.repeaters[repeater.ID] = repeater
}

// GetRepeater returns a repeater by ID
func (n *MockYSFRepeaterNetwork) GetRepeater(id string) *MockYSFRepeater {
	n.mu.RLock()
	defer n.mu.RUnlock()
	
	return n.repeaters[id]
}

// StartAll starts all repeaters in the network
func (n *MockYSFRepeaterNetwork) StartAll() error {
	n.mu.RLock()
	defer n.mu.RUnlock()
	
	for _, repeater := range n.repeaters {
		if err := repeater.Start(); err != nil {
			return err
		}
	}
	
	return nil
}

// StopAll stops all repeaters in the network
func (n *MockYSFRepeaterNetwork) StopAll() {
	n.mu.RLock()
	defer n.mu.RUnlock()
	
	for _, repeater := range n.repeaters {
		if err := repeater.Stop(); err != nil {
			log.Printf("mock repeater network: failed to stop repeater %s: %v", repeater.ID, err)
		}
	}
}

// GetNetworkStatus returns status of all repeaters
func (n *MockYSFRepeaterNetwork) GetNetworkStatus() map[string]interface{} {
	n.mu.RLock()
	defer n.mu.RUnlock()
	
	status := make(map[string]interface{})
	for id, repeater := range n.repeaters {
		status[id] = repeater.GetStatus()
	}
	
	return status
}