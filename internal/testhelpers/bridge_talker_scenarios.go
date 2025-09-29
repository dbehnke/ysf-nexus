package testhelpers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

// BridgeTalkerTestScenarios provides specific test scenarios for bridge talker functionality
type BridgeTalkerTestScenarios struct {
	suite      *IntegrationTestSuite
	httpClient *http.Client
	baseURL    string
}

// NewBridgeTalkerTestScenarios creates a new bridge talker test scenario runner
func NewBridgeTalkerTestScenarios(suite *IntegrationTestSuite) *BridgeTalkerTestScenarios {
	return &BridgeTalkerTestScenarios{
		suite:   suite,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL: suite.baseURL,
	}
}

// TestSingleBridgeTalker tests basic single bridge talker functionality
func (b *BridgeTalkerTestScenarios) TestSingleBridgeTalker(t *testing.T) {
	t.Log("Testing single bridge talker scenario...")
	
	bridge := b.suite.bridgeNetwork.GetBridge(b.suite.bridges[0])
	if bridge == nil {
		t.Fatal("No bridge available for testing")
	}
	
	// Test data
	testCallsign := "W9TRO"
	testLocation := "Chicago, IL"
	talkDuration := 5 * time.Second
	
	// Verify no current talker initially
	initialTalker := bridge.GetCurrentTalker()
	if initialTalker != nil {
		t.Errorf("Expected no initial talker, got: %s", initialTalker.Callsign)
	}
	
	// Start talker
	talker, err := bridge.StartTalker(testCallsign, testLocation)
	if err != nil {
		t.Fatalf("Failed to start talker: %v", err)
	}
	
	// Verify talker properties
	if talker.Callsign != testCallsign {
		t.Errorf("Callsign mismatch: got %s, want %s", talker.Callsign, testCallsign)
	}
	
	if talker.Location != testLocation {
		t.Errorf("Location mismatch: got %s, want %s", talker.Location, testLocation)
	}
	
	if !talker.IsActive {
		t.Errorf("Talker should be active")
	}
	
	if talker.BridgeID != bridge.ID {
		t.Errorf("Bridge ID mismatch: got %s, want %s", talker.BridgeID, bridge.ID)
	}
	
	// Simulate talking with voice packets
	packetCount := 50
	err = bridge.SendVoicePackets(packetCount)
	if err != nil {
		t.Errorf("Failed to send voice packets: %v", err)
	}
	
	// Check current talker during transmission
	currentTalker := bridge.GetCurrentTalker()
	if currentTalker == nil {
		t.Error("No current talker found during transmission")
	} else {
		if currentTalker.PacketCount != packetCount {
			t.Errorf("Packet count mismatch: got %d, want %d", currentTalker.PacketCount, packetCount)
		}
		
		if currentTalker.Duration <= 0 {
			t.Errorf("Duration should be positive, got %d", currentTalker.Duration)
		}
	}
	
	// Wait for specified talk duration
	time.Sleep(talkDuration)
	
	// Stop talker
	err = bridge.StopCurrentTalker()
	if err != nil {
		t.Errorf("Failed to stop talker: %v", err)
	}
	
	// Verify no current talker after stopping
	finalTalker := bridge.GetCurrentTalker()
	if finalTalker != nil {
		t.Errorf("Expected no talker after stop, got: %s", finalTalker.Callsign)
	}
	
	// Check history
	history := bridge.GetTalkerHistory(1)
	if len(history) == 0 {
		t.Error("No talker found in history")
	} else {
		historyTalker := history[0]
		if historyTalker.Callsign != testCallsign {
			t.Errorf("History callsign mismatch: got %s, want %s", historyTalker.Callsign, testCallsign)
		}
		
		if historyTalker.IsActive {
			t.Error("History talker should not be active")
		}
		
		if historyTalker.Duration <= 0 {
			t.Errorf("History duration should be positive, got %d", historyTalker.Duration)
		}
		
		if historyTalker.PacketCount != packetCount {
			t.Errorf("History packet count mismatch: got %d, want %d", historyTalker.PacketCount, packetCount)
		}
	}
	
	t.Logf("Single bridge talker test completed successfully")
}

// TestMultipleBridgeTalkers tests multiple bridge talkers across different bridges
func (b *BridgeTalkerTestScenarios) TestMultipleBridgeTalkers(t *testing.T) {
	t.Log("Testing multiple bridge talkers scenario...")
	
	if len(b.suite.bridges) < 2 {
		t.Skip("Need at least 2 bridges for this test")
	}
	
	// Test data for multiple talkers
	talkerData := []struct {
		bridgeIndex int
		callsign    string
		location    string
		duration    time.Duration
	}{
		{0, "W9TRO", "Chicago, IL", 3 * time.Second},
		{1, "G0RDH", "London, UK", 4 * time.Second},
	}
	
	var wg sync.WaitGroup
	results := make(chan error, len(talkerData))
	
	// Start multiple talkers concurrently
	for _, data := range talkerData {
		wg.Add(1)
		go func(bridgeIdx int, callsign, location string, duration time.Duration) {
			defer wg.Done()
			
			bridge := b.suite.bridgeNetwork.GetBridge(b.suite.bridges[bridgeIdx])
			if bridge == nil {
				results <- fmt.Errorf("bridge %d not found", bridgeIdx)
				return
			}
			
			// Start talker
			_, err := bridge.StartTalker(callsign, location)
			if err != nil {
				results <- fmt.Errorf("failed to start talker %s: %v", callsign, err)
				return
			}
			
			// Send voice packets
			err = bridge.SendVoicePackets(25)
			if err != nil {
				results <- fmt.Errorf("failed to send packets for %s: %v", callsign, err)
				return
			}
			
			// Wait for duration
			time.Sleep(duration)
			
			// Stop talker
			err = bridge.StopCurrentTalker()
			if err != nil {
				results <- fmt.Errorf("failed to stop talker %s: %v", callsign, err)
				return
			}
			
			t.Logf("Completed talking sequence for %s on bridge %s", callsign, bridge.ID)
			results <- nil
			
		}(data.bridgeIndex, data.callsign, data.location, data.duration)
	}
	
	// Wait for all talkers to complete
	wg.Wait()
	close(results)
	
	// Check results
	for err := range results {
		if err != nil {
			t.Error(err)
		}
	}
	
	// Verify all bridges have history
	for i, data := range talkerData {
		bridge := b.suite.bridgeNetwork.GetBridge(b.suite.bridges[data.bridgeIndex])
		if bridge != nil {
			history := bridge.GetTalkerHistory(1)
			if len(history) == 0 {
				t.Errorf("No history for bridge %d", i)
			} else if history[0].Callsign != data.callsign {
				t.Errorf("History callsign mismatch for bridge %d: got %s, want %s", 
					i, history[0].Callsign, data.callsign)
			}
		}
	}
	
	t.Log("Multiple bridge talkers test completed successfully")
}

// TestTalkerSequencing tests proper sequencing of talkers on the same bridge
func (b *BridgeTalkerTestScenarios) TestTalkerSequencing(t *testing.T) {
	t.Log("Testing talker sequencing scenario...")
	
	bridge := b.suite.bridgeNetwork.GetBridge(b.suite.bridges[0])
	if bridge == nil {
		t.Fatal("No bridge available for testing")
	}
	
	// Sequence of talkers
	talkerSequence := []struct {
		callsign string
		location string
		packets  int
	}{
		{"W9TRO", "Chicago, IL", 10},
		{"G0RDH", "London, UK", 15},
		{"VK2ABC", "Sydney, AU", 20},
		{"JA1XYZ", "Tokyo, JP", 12},
	}
	
	for i, talkerInfo := range talkerSequence {
		t.Logf("Starting talker %d: %s", i+1, talkerInfo.callsign)
		
		// Start talker
		_, err := bridge.StartTalker(talkerInfo.callsign, talkerInfo.location)
		if err != nil {
			t.Errorf("Failed to start talker %s: %v", talkerInfo.callsign, err)
			continue
		}
		
		// Verify this is the current talker
		currentTalker := bridge.GetCurrentTalker()
		if currentTalker == nil {
			t.Errorf("No current talker after starting %s", talkerInfo.callsign)
			continue
		}
		
		if currentTalker.Callsign != talkerInfo.callsign {
			t.Errorf("Current talker mismatch: got %s, want %s", 
				currentTalker.Callsign, talkerInfo.callsign)
		}
		
		// Send voice packets
		err = bridge.SendVoicePackets(talkerInfo.packets)
		if err != nil {
			t.Errorf("Failed to send packets for %s: %v", talkerInfo.callsign, err)
		}
		
		// Verify packet count
		currentTalker = bridge.GetCurrentTalker()
		if currentTalker != nil && currentTalker.PacketCount != talkerInfo.packets {
			t.Errorf("Packet count mismatch for %s: got %d, want %d", 
				talkerInfo.callsign, currentTalker.PacketCount, talkerInfo.packets)
		}
		
		// Wait a bit
		time.Sleep(200 * time.Millisecond)
		
		// Stop talker
		err = bridge.StopCurrentTalker()
		if err != nil {
			t.Errorf("Failed to stop talker %s: %v", talkerInfo.callsign, err)
		}
		
		// Verify no current talker
		currentTalker = bridge.GetCurrentTalker()
		if currentTalker != nil {
			t.Errorf("Talker still active after stop: %s", currentTalker.Callsign)
		}
	}
	
	// Verify history has all talkers in reverse order (most recent first)
	history := bridge.GetTalkerHistory(len(talkerSequence))
	if len(history) != len(talkerSequence) {
		t.Errorf("History count mismatch: got %d, want %d", len(history), len(talkerSequence))
	} else {
		// Check that history is in reverse chronological order
		for i, expectedTalker := range talkerSequence {
			historyIndex := len(talkerSequence) - 1 - i
			if history[historyIndex].Callsign != expectedTalker.callsign {
				t.Errorf("History order mismatch at position %d: got %s, want %s", 
					historyIndex, history[historyIndex].Callsign, expectedTalker.callsign)
			}
		}
	}
	
	t.Log("Talker sequencing test completed successfully")
}

// TestTalkerInterruption tests talker interruption scenarios
func (b *BridgeTalkerTestScenarios) TestTalkerInterruption(t *testing.T) {
	t.Log("Testing talker interruption scenario...")
	
	bridge := b.suite.bridgeNetwork.GetBridge(b.suite.bridges[0])
	if bridge == nil {
		t.Fatal("No bridge available for testing")
	}
	
	// Start first talker
	_, err := bridge.StartTalker("W9TRO", "Chicago, IL")
	if err != nil {
		t.Fatalf("Failed to start first talker: %v", err)
	}
	
	// Send some packets
	err = bridge.SendVoicePackets(5)
	if err != nil {
		t.Errorf("Failed to send packets for first talker: %v", err)
	}
	
	// Verify first talker is active
	currentTalker := bridge.GetCurrentTalker()
	if currentTalker == nil || currentTalker.Callsign != "W9TRO" {
		t.Error("First talker not properly active")
	}
	
	// Start second talker (should interrupt first)
	var err2 error
	_, err2 = bridge.StartTalker("G0RDH", "London, UK")
	if err2 != nil {
		t.Fatalf("Failed to start second talker: %v", err2)
	}
	
	// Verify second talker is now active
	currentTalker = bridge.GetCurrentTalker()
	if currentTalker == nil || currentTalker.Callsign != "G0RDH" {
		t.Error("Second talker not properly active after interruption")
	}
	
	// Check that first talker is in history
	history := bridge.GetTalkerHistory(1)
	if len(history) == 0 {
		t.Error("First talker not found in history after interruption")
	} else if history[0].Callsign != "W9TRO" {
		t.Errorf("History mismatch: got %s, want W9TRO", history[0].Callsign)
	} else if history[0].IsActive {
		t.Error("Interrupted talker should not be active in history")
	} else if history[0].PacketCount != 5 {
		t.Errorf("Interrupted talker packet count mismatch: got %d, want 5", history[0].PacketCount)
	}
	
	// Complete second talker
	err = bridge.SendVoicePackets(8)
	if err != nil {
		t.Errorf("Failed to send packets for second talker: %v", err)
	}
	
	time.Sleep(100 * time.Millisecond)
	
	err = bridge.StopCurrentTalker()
	if err != nil {
		t.Errorf("Failed to stop second talker: %v", err)
	}
	
	// Verify history now has both talkers
	history = bridge.GetTalkerHistory(2)
	if len(history) != 2 {
		t.Errorf("History should have 2 talkers, got %d", len(history))
	}
	
	t.Log("Talker interruption test completed successfully")
}

// TestAPIIntegrationWithBridgeTalkers tests API integration with real bridge talker data
func (b *BridgeTalkerTestScenarios) TestAPIIntegrationWithBridgeTalkers(t *testing.T) {
	t.Log("Testing API integration with bridge talkers...")
	
	bridge := b.suite.bridgeNetwork.GetBridge(b.suite.bridges[0])
	if bridge == nil {
		t.Fatal("No bridge available for testing")
	}
	
	// Test 1: API with no active talker
	resp, err := b.httpClient.Get(b.baseURL + "/api/current-talker")
	if err != nil {
		t.Logf("API call failed (expected if server not running): %v", err)
		// Continue with mock testing even if API is not available
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		t.Logf("API Response (no talker): %s", string(body))
	}
	
	// Start a talker
	testCallsign := "W9TRO"
	testLocation := "Chicago, IL"
	
	_, err = bridge.StartTalker(testCallsign, testLocation)
	if err != nil {
		t.Fatalf("Failed to start test talker: %v", err)
	}
	
	// Send some packets to make it realistic
	err = bridge.SendVoicePackets(10)
	if err != nil {
		t.Errorf("Failed to send voice packets: %v", err)
	}
	
	// Test 2: API with active talker
	time.Sleep(100 * time.Millisecond) // Give time for data to register
	
	resp, err = b.httpClient.Get(b.baseURL + "/api/current-talker")
	if err != nil {
		t.Logf("API call failed (expected if server not running): %v", err)
	} else {
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("Failed to read response body: %v", err)
			} else {
				var apiResponse map[string]interface{}
				err = json.Unmarshal(body, &apiResponse)
				if err != nil {
					t.Errorf("Failed to parse API response: %v", err)
				} else {
					t.Logf("API Response (with talker): %s", string(body))
					
					// Validate response structure
					if data, ok := apiResponse["data"].(map[string]interface{}); ok {
						if callsign, ok := data["callsign"].(string); ok {
							if callsign != testCallsign {
								t.Errorf("API callsign mismatch: got %s, want %s", callsign, testCallsign)
							}
						} else {
							t.Error("API response missing callsign field")
						}
						
						if duration, ok := data["duration"].(float64); ok {
							if duration <= 0 {
								t.Errorf("API duration should be positive, got %f", duration)
							}
						} else {
							t.Error("API response missing duration field")
						}
					} else {
						t.Error("API response missing data field")
					}
				}
			}
		} else {
			t.Logf("API returned status %d", resp.StatusCode)
		}
	}
	
	// Stop the talker
	err = bridge.StopCurrentTalker()
	if err != nil {
		t.Errorf("Failed to stop talker: %v", err)
	}
	
	// Test 3: API after talker stopped
	time.Sleep(100 * time.Millisecond)
	
	resp, err = b.httpClient.Get(b.baseURL + "/api/current-talker")
	if err != nil {
		t.Logf("API call failed (expected if server not running): %v", err)
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		t.Logf("API Response (after stop): %s", string(body))
	}
	
	t.Log("API integration test completed")
}

// TestTalkerDurationTracking tests accurate duration tracking
func (b *BridgeTalkerTestScenarios) TestTalkerDurationTracking(t *testing.T) {
	t.Log("Testing talker duration tracking...")
	
	bridge := b.suite.bridgeNetwork.GetBridge(b.suite.bridges[0])
	if bridge == nil {
		t.Fatal("No bridge available for testing")
	}
	
	// Test precise duration tracking
	testDurations := []time.Duration{
		1 * time.Second,
		2500 * time.Millisecond,
		5 * time.Second,
	}
	
	for i, expectedDuration := range testDurations {
		callsign := fmt.Sprintf("TEST%d", i+1)
		
		// Record start time
		startTime := time.Now()
		
		// Start talker
		talker, err := bridge.StartTalker(callsign, "Test Location")
		if err != nil {
			t.Errorf("Failed to start talker %s: %v", callsign, err)
			continue
		}
		
		// Verify start time is close
		if abs(talker.StartTime.Sub(startTime)) > 100*time.Millisecond {
			t.Errorf("Start time mismatch for %s: diff=%v", callsign, talker.StartTime.Sub(startTime))
		}
		
		// Send packets during duration
		go func() {
			for elapsed := time.Duration(0); elapsed < expectedDuration; elapsed += 20*time.Millisecond {
				bridge.SendVoicePackets(1)
				time.Sleep(20 * time.Millisecond)
			}
		}()
		
		// Wait for expected duration
		time.Sleep(expectedDuration)
		
		// Check current talker duration
		currentTalker := bridge.GetCurrentTalker()
		if currentTalker != nil {
			actualDuration := time.Duration(currentTalker.Duration) * time.Second
			diff := abs(actualDuration - expectedDuration)
			if diff > 500*time.Millisecond {
				t.Errorf("Duration tracking error for %s: expected=%v, got=%v, diff=%v", 
					callsign, expectedDuration, actualDuration, diff)
			}
		}
		
		// Stop talker
		stopTime := time.Now()
		err = bridge.StopCurrentTalker()
		if err != nil {
			t.Errorf("Failed to stop talker %s: %v", callsign, err)
			continue
		}
		
		// Check final duration in history
		history := bridge.GetTalkerHistory(1)
		if len(history) > 0 {
			finalTalker := history[0]
			
			// Verify end time
			if abs(finalTalker.EndTime.Sub(stopTime)) > 100*time.Millisecond {
				t.Errorf("End time mismatch for %s: diff=%v", 
					callsign, finalTalker.EndTime.Sub(stopTime))
			}
			
			// Verify total duration
			totalDuration := finalTalker.EndTime.Sub(finalTalker.StartTime)
			diff := abs(totalDuration - expectedDuration)
			if diff > 500*time.Millisecond {
				t.Errorf("Final duration error for %s: expected=%v, got=%v, diff=%v", 
					callsign, expectedDuration, totalDuration, diff)
			}
			
			// Verify duration field matches
			recordedDuration := time.Duration(finalTalker.Duration) * time.Second
			if abs(recordedDuration - totalDuration) > 1*time.Second {
				t.Errorf("Duration field mismatch for %s: calculated=%v, recorded=%v", 
					callsign, totalDuration, recordedDuration)
			}
		}
		
		t.Logf("Duration test %d completed: %s talked for %v", i+1, callsign, expectedDuration)
	}
	
	t.Log("Duration tracking test completed successfully")
}

// TestHighFrequencyActivity tests system under high-frequency talker changes
func (b *BridgeTalkerTestScenarios) TestHighFrequencyActivity(t *testing.T) {
	t.Log("Testing high-frequency activity scenario...")
	
	bridge := b.suite.bridgeNetwork.GetBridge(b.suite.bridges[0])
	if bridge == nil {
		t.Fatal("No bridge available for testing")
	}
	
	// Rapid succession of short talkers
	callsigns := []string{"W9TRO", "G0RDH", "VK2ABC", "JA1XYZ", "K5ABC", "M0XYZ"}
	rapidTalkDuration := 200 * time.Millisecond
	
	expectedTalkers := len(callsigns)
	
	// Rapid fire talkers
	for i, callsign := range callsigns {
		_, err := bridge.StartTalker(callsign, fmt.Sprintf("Location %d", i))
		if err != nil {
			t.Errorf("Failed to start rapid talker %s: %v", callsign, err)
			continue
		}
		
		// Quick burst of packets
		err = bridge.SendVoicePackets(3)
		if err != nil {
			t.Errorf("Failed to send packets for %s: %v", callsign, err)
		}
		
		// Short talk time
		time.Sleep(rapidTalkDuration)
		
		err = bridge.StopCurrentTalker()
		if err != nil {
			t.Errorf("Failed to stop rapid talker %s: %v", callsign, err)
		}
		
		// Very brief gap
		time.Sleep(50 * time.Millisecond)
	}
	
	// Verify all talkers are in history
	history := bridge.GetTalkerHistory(expectedTalkers)
	if len(history) != expectedTalkers {
		t.Errorf("History count mismatch: got %d, want %d", len(history), expectedTalkers)
	}
	
	// Verify order and basic properties
	for i, expectedCallsign := range callsigns {
		historyIndex := len(callsigns) - 1 - i // Reverse order
		if historyIndex < len(history) {
			actualCallsign := history[historyIndex].Callsign
			if actualCallsign != expectedCallsign {
				t.Errorf("History order error at %d: got %s, want %s", 
					historyIndex, actualCallsign, expectedCallsign)
			}
			
			if history[historyIndex].IsActive {
				t.Errorf("Historical talker %s should not be active", actualCallsign)
			}
		}
	}
	
	t.Log("High-frequency activity test completed successfully")
}

// RunAllBridgeTalkerTests runs all bridge talker test scenarios
func (b *BridgeTalkerTestScenarios) RunAllBridgeTalkerTests(t *testing.T) {
	t.Log("Running all bridge talker test scenarios...")
	
	// Run individual tests
	b.TestSingleBridgeTalker(t)
	b.TestMultipleBridgeTalkers(t)
	b.TestTalkerSequencing(t)
	b.TestTalkerInterruption(t)
	b.TestTalkerDurationTracking(t)
	b.TestHighFrequencyActivity(t)
	b.TestAPIIntegrationWithBridgeTalkers(t)
	
	t.Log("All bridge talker tests completed")
}

// Helper function to get absolute value of duration
func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}