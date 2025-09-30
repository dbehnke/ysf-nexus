package testhelpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"testing"
	"time"
)

// IntegrationTestSuite provides a comprehensive testing framework
type IntegrationTestSuite struct {
	// Network infrastructure
	repeaterNetwork *MockYSFRepeaterNetwork
	bridgeNetwork   *MockBridgeNetwork

	// Individual components for direct access
	repeaters []string
	bridges   []string

	// Test configuration
	config TestConfig

	// Event tracking
	events     []TestEvent
	eventsChan chan TestEvent
	eventsLock sync.RWMutex

	// HTTP client for API testing
	httpClient *http.Client
	baseURL    string

	// Context for cleanup
	ctx    context.Context
	cancel context.CancelFunc
}

// TestConfig holds configuration for the test suite
type TestConfig struct {
	// Network settings
	RepeaterCount int
	BridgeCount   int
	BasePort      int

	// API settings
	APIBaseURL string
	APITimeout time.Duration

	// Test behavior
	ActivityDuration time.Duration
	MaxTalkTime      time.Duration
	MinTalkTime      time.Duration

	// Logging
	VerboseLogging bool
}

// TestEvent represents events that occur during testing
type TestEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	Type        string                 `json:"type"`
	Source      string                 `json:"source"`
	Data        map[string]interface{} `json:"data"`
	Description string                 `json:"description"`
}

// DefaultTestConfig returns a default test configuration
func DefaultTestConfig() TestConfig {
	return TestConfig{
		RepeaterCount:    3,
		BridgeCount:      2,
		BasePort:         10000,
		APIBaseURL:       "http://localhost:8080",
		APITimeout:       5 * time.Second,
		ActivityDuration: 30 * time.Second,
		MaxTalkTime:      10 * time.Second,
		MinTalkTime:      2 * time.Second,
		VerboseLogging:   true,
	}
}

// NewIntegrationTestSuite creates a new integration test suite
func NewIntegrationTestSuite(config TestConfig) *IntegrationTestSuite {
	ctx, cancel := context.WithCancel(context.Background())

	suite := &IntegrationTestSuite{
		repeaterNetwork: NewMockYSFRepeaterNetwork(),
		bridgeNetwork:   NewMockBridgeNetwork(),
		config:          config,
		events:          make([]TestEvent, 0),
		eventsChan:      make(chan TestEvent, 1000),
		httpClient: &http.Client{
			Timeout: config.APITimeout,
		},
		baseURL: config.APIBaseURL,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start event collector
	go suite.collectEvents()

	return suite
}

// Setup initializes the test environment
func (s *IntegrationTestSuite) Setup(t *testing.T) error {
	t.Log("Setting up integration test suite...")

	// Create mock repeaters
	for i := 0; i < s.config.RepeaterCount; i++ {
		repeaterID := fmt.Sprintf("REP%03d", i+1)
		address := fmt.Sprintf("127.0.0.1:%d", s.config.BasePort+i)

		repeater, err := NewMockYSFRepeater(
			repeaterID,
			fmt.Sprintf("Mock Repeater %d", i+1),
			address,
		)
		if err != nil {
			return fmt.Errorf("failed to create repeater %s: %v", repeaterID, err)
		}

		// Configure repeater
		repeater.Description = fmt.Sprintf("Test repeater %d for integration testing", i+1)
		repeater.Frequency = fmt.Sprintf("%.3f MHz", 144.0+float64(i)*0.025)
		repeater.Location = fmt.Sprintf("Test Location %d", i+1)

		// Set up event handlers
		repeater.SetEventHandlers(
			func(packet []byte, from string) {
				s.recordEvent("packet_received", repeaterID, map[string]interface{}{
					"from":        from,
					"packet_size": len(packet),
					"packet_type": string(packet[0:4]),
				}, fmt.Sprintf("Packet received from %s", from))
			},
			func(clientID string) {
				s.recordEvent("client_connected", repeaterID, map[string]interface{}{
					"client_id": clientID,
				}, fmt.Sprintf("Client %s connected", clientID))
			},
			func(clientID string) {
				s.recordEvent("client_disconnected", repeaterID, map[string]interface{}{
					"client_id": clientID,
				}, fmt.Sprintf("Client %s disconnected", clientID))
			},
		)

		s.repeaterNetwork.AddRepeater(repeater)
		s.repeaters = append(s.repeaters, repeaterID)
	}

	// Create mock bridges
	for i := 0; i < s.config.BridgeCount; i++ {
		bridgeID := fmt.Sprintf("BR%03d", i+1)
		localAddr := fmt.Sprintf("127.0.0.1:%d", s.config.BasePort+1000+i)
		remoteAddr := fmt.Sprintf("127.0.0.1:%d", s.config.BasePort+2000+i)

		bridge, err := NewMockBridgeEndpoint(
			bridgeID,
			fmt.Sprintf("Mock Bridge %d", i+1),
			localAddr,
			remoteAddr,
		)
		if err != nil {
			return fmt.Errorf("failed to create bridge %s: %v", bridgeID, err)
		}

		bridge.Description = fmt.Sprintf("Test bridge %d for integration testing", i+1)

		// Set up event handlers
		bridge.SetEventHandlers(
			func(talker *BridgeTalker) {
				s.recordEvent("bridge_talker_start", bridgeID, map[string]interface{}{
					"callsign":  talker.Callsign,
					"location":  talker.Location,
					"bridge_id": talker.BridgeID,
				}, fmt.Sprintf("Bridge talker %s started", talker.Callsign))
			},
			func(talker *BridgeTalker) {
				s.recordEvent("bridge_talker_stop", bridgeID, map[string]interface{}{
					"callsign":     talker.Callsign,
					"duration":     talker.Duration,
					"packet_count": talker.PacketCount,
					"bridge_id":    talker.BridgeID,
				}, fmt.Sprintf("Bridge talker %s stopped (duration: %ds)", talker.Callsign, talker.Duration))
			},
			func(packet []byte) {
				s.recordEvent("bridge_packet", bridgeID, map[string]interface{}{
					"packet_size": len(packet),
					"packet_type": string(packet[0:4]),
				}, "Bridge packet sent")
			},
		)

		s.bridgeNetwork.AddBridge(bridge)
		s.bridges = append(s.bridges, bridgeID)
	}

	// Start all components
	if err := s.repeaterNetwork.StartAll(); err != nil {
		return fmt.Errorf("failed to start repeaters: %v", err)
	}

	if err := s.bridgeNetwork.ConnectAll(); err != nil {
		return fmt.Errorf("failed to connect bridges: %v", err)
	}

	t.Logf("Integration test suite setup complete: %d repeaters, %d bridges",
		len(s.repeaters), len(s.bridges))

	return nil
}

// Teardown cleans up the test environment
func (s *IntegrationTestSuite) Teardown(t *testing.T) {
	t.Log("Tearing down integration test suite...")

	// Stop all components
	s.repeaterNetwork.StopAll()
	s.bridgeNetwork.DisconnectAll()

	// Cancel context and cleanup
	s.cancel()

	// Log final statistics
	s.eventsLock.RLock()
	eventCount := len(s.events)
	s.eventsLock.RUnlock()

	t.Logf("Test completed with %d recorded events", eventCount)
}

// TestBasicConnectivity tests basic connectivity of all components
func (s *IntegrationTestSuite) TestBasicConnectivity(t *testing.T) {
	t.Log("Testing basic connectivity...")

	// Test repeater connectivity
	for _, repeaterID := range s.repeaters {
		repeater := s.repeaterNetwork.GetRepeater(repeaterID)
		if repeater == nil {
			t.Errorf("Repeater %s not found", repeaterID)
			continue
		}

		status := repeater.GetStatus()
		if status["id"] != repeaterID {
			t.Errorf("Repeater %s status mismatch", repeaterID)
		}

		t.Logf("Repeater %s: %s - OK", repeaterID, status["name"])
	}

	// Test bridge connectivity
	for _, bridgeID := range s.bridges {
		bridge := s.bridgeNetwork.GetBridge(bridgeID)
		if bridge == nil {
			t.Errorf("Bridge %s not found", bridgeID)
			continue
		}

		status := bridge.GetStatus()
		if !status["is_connected"].(bool) {
			t.Errorf("Bridge %s not connected", bridgeID)
		}

		t.Logf("Bridge %s: %s - OK", bridgeID, status["name"])
	}
}

// TestRepeaterFunctionality tests repeater operations
func (s *IntegrationTestSuite) TestRepeaterFunctionality(t *testing.T) {
	t.Log("Testing repeater functionality...")

	repeater := s.repeaterNetwork.GetRepeater(s.repeaters[0])
	if repeater == nil {
		t.Fatal("No repeaters available")
	}

	// Connect mock clients
	clients := []struct{ callsign, location, address string }{
		{"W9TRO", "Chicago, IL", "127.0.0.1:20001"},
		{"G0RDH", "London, UK", "127.0.0.1:20002"},
		{"VK2ABC", "Sydney, AU", "127.0.0.1:20003"},
	}

	var clientIDs []string
	for _, client := range clients {
		mockClient, err := repeater.ConnectClient(client.callsign, client.location, client.address)
		if err != nil {
			t.Errorf("Failed to connect client %s: %v", client.callsign, err)
			continue
		}
		clientIDs = append(clientIDs, mockClient.ID)
		t.Logf("Connected client: %s from %s", client.callsign, client.location)
	}

	// Test talking sequence
	for _, clientID := range clientIDs {
		err := repeater.StartTalking(clientID)
		if err != nil {
			t.Errorf("Failed to start talking for %s: %v", clientID, err)
			continue
		}

		// Send some voice packets
		err = repeater.SendVoicePackets(clientID, 10)
		if err != nil {
			t.Errorf("Failed to send voice packets for %s: %v", clientID, err)
		}

		time.Sleep(100 * time.Millisecond)

		err = repeater.StopTalking(clientID)
		if err != nil {
			t.Errorf("Failed to stop talking for %s: %v", clientID, err)
		}

		t.Logf("Completed talking sequence for client: %s", clientID)
	}

	// Cleanup
	for _, clientID := range clientIDs {
		if err := repeater.DisconnectClient(clientID); err != nil {
			t.Logf("Failed to disconnect client %s: %v", clientID, err)
		}
	}
}

// TestBridgeFunctionality tests bridge operations
func (s *IntegrationTestSuite) TestBridgeFunctionality(t *testing.T) {
	t.Log("Testing bridge functionality...")

	bridge := s.bridgeNetwork.GetBridge(s.bridges[0])
	if bridge == nil {
		t.Fatal("No bridges available")
	}

	// Test talker sequences
	testTalkers := []struct{ callsign, location string }{
		{"W9TRO", "Chicago, IL"},
		{"G0RDH", "London, UK"},
		{"JA1XYZ", "Tokyo, JP"},
	}

	for _, talkerInfo := range testTalkers {
		// Start talker
		talker, err := bridge.StartTalker(talkerInfo.callsign, talkerInfo.location)
		if err != nil {
			t.Errorf("Failed to start bridge talker %s: %v", talkerInfo.callsign, err)
			continue
		}

		if talker.Callsign != talkerInfo.callsign {
			t.Errorf("Talker callsign mismatch: got %s, want %s", talker.Callsign, talkerInfo.callsign)
		}

		if !talker.IsActive {
			t.Errorf("Talker should be active")
		}

		// Send voice packets
		err = bridge.SendVoicePackets(5)
		if err != nil {
			t.Errorf("Failed to send voice packets: %v", err)
		}

		// Verify current talker
		currentTalker := bridge.GetCurrentTalker()
		if currentTalker == nil {
			t.Errorf("No current talker found")
		} else if currentTalker.Callsign != talkerInfo.callsign {
			t.Errorf("Current talker mismatch: got %s, want %s", currentTalker.Callsign, talkerInfo.callsign)
		}

		time.Sleep(100 * time.Millisecond)

		// Stop talker
		err = bridge.StopCurrentTalker()
		if err != nil {
			t.Errorf("Failed to stop talker: %v", err)
		}

		// Verify talker stopped
		currentTalker = bridge.GetCurrentTalker()
		if currentTalker != nil {
			t.Errorf("Talker should have stopped")
		}

		t.Logf("Completed bridge talking sequence for: %s", talkerInfo.callsign)
	}

	// Check talker history
	history := bridge.GetTalkerHistory(10)
	if len(history) != len(testTalkers) {
		t.Errorf("History count mismatch: got %d, want %d", len(history), len(testTalkers))
	}
}

// TestAPIIntegration tests API endpoints with mock data
func (s *IntegrationTestSuite) TestAPIIntegration(t *testing.T) {
	t.Log("Testing API integration...")

	// Start bridge activity
	bridge := s.bridgeNetwork.GetBridge(s.bridges[0])
	if bridge == nil {
		t.Fatal("No bridges available")
	}

	// Start a talker to create API data
	_, err := bridge.StartTalker("W9TRO", "Test Location")
	if err != nil {
		t.Fatalf("Failed to start test talker: %v", err)
	}

	// Give it a moment to register
	time.Sleep(100 * time.Millisecond)

	// Test current talker API
	resp, err := s.httpClient.Get(s.baseURL + "/api/current-talker")
	if err != nil {
		t.Errorf("Failed to call current-talker API: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("API returned status %d, want %d", resp.StatusCode, http.StatusOK)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("Failed to read response body: %v", err)
		return
	}

	var apiResponse map[string]interface{}
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		t.Errorf("Failed to parse API response: %v", err)
		return
	}

	t.Logf("API Response: %s", string(body))

	// Stop the talker
	if err := bridge.StopCurrentTalker(); err != nil {
		t.Errorf("Failed to stop talker: %v", err)
	}

	// Test that API now returns no current talker
	time.Sleep(100 * time.Millisecond)

	resp2, err := s.httpClient.Get(s.baseURL + "/api/current-talker")
	if err != nil {
		t.Errorf("Failed to call current-talker API (second call): %v", err)
		return
	}
	defer func() { _ = resp2.Body.Close() }()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Errorf("Failed to read second response body: %v", err)
		return
	}

	t.Logf("API Response (after stop): %s", string(body2))
}

// TestComplexScenarios runs complex multi-component scenarios
func (s *IntegrationTestSuite) TestComplexScenarios(t *testing.T) {
	t.Log("Testing complex scenarios...")

	// Scenario 1: Multiple concurrent bridge talkers
	t.Log("Scenario 1: Multiple concurrent bridge activities")

	scenarios := []struct {
		bridgeIndex int
		callsign    string
		location    string
		duration    time.Duration
	}{
		{0, "W9TRO", "Chicago, IL", 2 * time.Second},
		{1, "G0RDH", "London, UK", 3 * time.Second},
	}

	if len(s.bridges) >= 2 {
		var wg sync.WaitGroup

		for _, scenario := range scenarios {
			wg.Add(1)
			go func(bridgeIdx int, callsign, location string, duration time.Duration) {
				defer wg.Done()

				bridge := s.bridgeNetwork.GetBridge(s.bridges[bridgeIdx])
				if bridge != nil {
					bridge.SimulateTalkingSequence(callsign, location, duration)
					t.Logf("Started talking sequence: %s on bridge %s", callsign, s.bridges[bridgeIdx])
				}
			}(scenario.bridgeIndex, scenario.callsign, scenario.location, scenario.duration)
		}

		wg.Wait()
		time.Sleep(1 * time.Second) // Wait for completion

		t.Log("Scenario 1 completed")
	}

	// Scenario 2: Repeater linking
	t.Log("Scenario 2: Repeater linking and activity")

	if len(s.repeaters) >= 2 {
		repeater1 := s.repeaterNetwork.GetRepeater(s.repeaters[0])
		repeater2 := s.repeaterNetwork.GetRepeater(s.repeaters[1])

		if repeater1 != nil && repeater2 != nil {
			// Link repeaters
			err := repeater1.LinkToRepeater(s.repeaters[1])
			if err != nil {
				t.Errorf("Failed to link repeaters: %v", err)
			} else {
				t.Logf("Linked %s to %s", s.repeaters[0], s.repeaters[1])
			}

			// Add clients and generate activity
			client, err := repeater1.ConnectClient("K5ABC", "Dallas, TX", "127.0.0.1:21001")
			if err == nil {
				if err := repeater1.StartTalking(client.ID); err != nil {
					t.Logf("failed to start talking for client %s: %v", client.ID, err)
				}
				if err := repeater1.SendVoicePackets(client.ID, 5); err != nil {
					t.Logf("failed to send voice packets for client %s: %v", client.ID, err)
				}
				if err := repeater1.StopTalking(client.ID); err != nil {
					t.Logf("failed to stop talking for client %s: %v", client.ID, err)
				}
				if err := repeater1.DisconnectClient(client.ID); err != nil {
					t.Logf("failed to disconnect client %s: %v", client.ID, err)
				}

				t.Log("Generated linked repeater activity")
			}

			// Unlink
			if err := repeater1.UnlinkFromRepeater(); err != nil {
				t.Logf("failed to unlink repeater: %v", err)
			}
		}
	}

	t.Log("Complex scenarios completed")
}

// StartRandomActivity starts random activity across all components
func (s *IntegrationTestSuite) StartRandomActivity(duration time.Duration) {

	s.recordEvent("random_activity_start", "suite", map[string]interface{}{
		"duration": duration.String(),
	}, "Starting random activity simulation")

	// Start bridge activity
	s.bridgeNetwork.StartRandomActivityOnAllBridges(duration)

	// Start repeater activity
	for _, repeaterID := range s.repeaters {
		repeater := s.repeaterNetwork.GetRepeater(repeaterID)
		if repeater != nil {
			repeater.SimulateRandomActivity(duration)
		}
	}

	// Record completion after duration
	go func() {
		time.Sleep(duration)
		s.recordEvent("random_activity_end", "suite", map[string]interface{}{
			"duration": duration.String(),
		}, "Random activity simulation completed")
	}()
}

// GetEventSummary returns a summary of recorded events
func (s *IntegrationTestSuite) GetEventSummary() map[string]int {
	s.eventsLock.RLock()
	defer s.eventsLock.RUnlock()

	summary := make(map[string]int)
	for _, event := range s.events {
		summary[event.Type]++
	}

	return summary
}

// GetEventsByType returns events filtered by type
func (s *IntegrationTestSuite) GetEventsByType(eventType string) []TestEvent {
	s.eventsLock.RLock()
	defer s.eventsLock.RUnlock()

	var filtered []TestEvent
	for _, event := range s.events {
		if event.Type == eventType {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// ExportEvents exports all events as JSON
func (s *IntegrationTestSuite) ExportEvents() ([]byte, error) {
	s.eventsLock.RLock()
	defer s.eventsLock.RUnlock()

	return json.MarshalIndent(s.events, "", "  ")
}

// Private methods

func (s *IntegrationTestSuite) collectEvents() {
	for {
		select {
		case event := <-s.eventsChan:
			s.eventsLock.Lock()
			s.events = append(s.events, event)
			s.eventsLock.Unlock()

			if s.config.VerboseLogging {
				log.Printf("Event: %s from %s - %s", event.Type, event.Source, event.Description)
			}

		case <-s.ctx.Done():
			return
		}
	}
}

func (s *IntegrationTestSuite) recordEvent(eventType, source string, data map[string]interface{}, description string) {
	event := TestEvent{
		Timestamp:   time.Now(),
		Type:        eventType,
		Source:      source,
		Data:        data,
		Description: description,
	}

	select {
	case s.eventsChan <- event:
	default:
		// Channel full, drop event
	}
}
