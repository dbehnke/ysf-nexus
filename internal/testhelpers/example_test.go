//go:build integration
// +build integration

package testhelpers

import (
	"testing"
	"time"
)

// IntegrationTestUsageExample shows how to use the integration test framework
func IntegrationTestUsageExample() {
	// This is an example of how to use the testing framework in your tests

	// Create test configuration
	config := DefaultTestConfig()
	config.RepeaterCount = 3
	config.BridgeCount = 2
	config.APIBaseURL = "http://localhost:8080"
	config.VerboseLogging = true

	// Create test suite
	_ = NewIntegrationTestSuite(config)

	// Use in your test function like this:
	// func TestYSFNexusIntegration(t *testing.T) {
	//     err := suite.Setup(t)
	//     if err != nil {
	//         t.Fatalf("Failed to setup test suite: %v", err)
	//     }
	//     defer suite.Teardown(t)
	//
	//     // Run your tests...
	//     suite.TestBasicConnectivity(t)
	//     suite.TestRepeaterFunctionality(t)
	//     suite.TestBridgeFunctionality(t)
	//     suite.TestAPIIntegration(t)
	//     suite.TestComplexScenarios(t)
	// }
}

// TestIntegrationFramework demonstrates the complete testing framework
func TestIntegrationFramework(t *testing.T) {
	// Create test configuration
	config := DefaultTestConfig()
	config.RepeaterCount = 2
	config.BridgeCount = 2
	config.VerboseLogging = false // Reduce noise in tests
	config.ActivityDuration = 5 * time.Second

	// Create and setup test suite
	suite := NewIntegrationTestSuite(config)

	err := suite.Setup(t)
	if err != nil {
		t.Fatalf("Failed to setup integration test suite: %v", err)
	}
	defer suite.Teardown(t)

	// Test basic connectivity
	t.Run("BasicConnectivity", func(t *testing.T) {
		suite.TestBasicConnectivity(t)
	})

	// Test repeater functionality
	t.Run("RepeaterFunctionality", func(t *testing.T) {
		suite.TestRepeaterFunctionality(t)
	})

	// Test bridge functionality
	t.Run("BridgeFunctionality", func(t *testing.T) {
		suite.TestBridgeFunctionality(t)
	})

	// Test API integration (may fail if server not running, which is OK)
	t.Run("APIIntegration", func(t *testing.T) {
		suite.TestAPIIntegration(t)
	})

	// Test complex scenarios
	t.Run("ComplexScenarios", func(t *testing.T) {
		suite.TestComplexScenarios(t)
	})

	// Print event summary
	summary := suite.GetEventSummary()
	t.Logf("Test completed with event summary: %+v", summary)
}

// TestBridgeTalkerScenarios demonstrates bridge talker specific testing
func TestBridgeTalkerScenarios(t *testing.T) {
	// Create test configuration focused on bridge testing
	config := DefaultTestConfig()
	config.RepeaterCount = 1
	config.BridgeCount = 2
	config.VerboseLogging = false

	// Create and setup test suite
	suite := NewIntegrationTestSuite(config)

	err := suite.Setup(t)
	if err != nil {
		t.Fatalf("Failed to setup bridge talker test suite: %v", err)
	}
	defer suite.Teardown(t)

	// Create bridge talker scenario tester
	scenarios := NewBridgeTalkerTestScenarios(suite)

	// Run all bridge talker tests
	t.Run("AllBridgeTalkerTests", func(t *testing.T) {
		scenarios.RunAllBridgeTalkerTests(t)
	})

	// Print final event summary
	summary := suite.GetEventSummary()
	t.Logf("Bridge talker tests completed. Events: %+v", summary)

	// Export events for analysis (optional)
	if events, err := suite.ExportEvents(); err == nil {
		t.Logf("Exported %d bytes of event data", len(events))
		// You could save this to a file for analysis:
		// os.WriteFile("test_events.json", events, 0644)
	}
}

// TestRandomActivity demonstrates random activity simulation
func TestRandomActivity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping random activity test in short mode")
	}

	// Create test configuration for activity simulation
	config := DefaultTestConfig()
	config.RepeaterCount = 3
	config.BridgeCount = 3
	config.VerboseLogging = true
	config.ActivityDuration = 10 * time.Second

	// Create and setup test suite
	suite := NewIntegrationTestSuite(config)

	err := suite.Setup(t)
	if err != nil {
		t.Fatalf("Failed to setup random activity test suite: %v", err)
	}
	defer suite.Teardown(t)

	t.Log("Starting random activity simulation...")

	// Start random activity on all components
	suite.StartRandomActivity(config.ActivityDuration)

	// Wait for activity to complete
	time.Sleep(config.ActivityDuration + 2*time.Second)

	// Analyze results
	summary := suite.GetEventSummary()
	t.Logf("Random activity completed. Event summary:")
	for eventType, count := range summary {
		t.Logf("  %s: %d", eventType, count)
	}

	// Check that we got some activity
	totalEvents := 0
	for _, count := range summary {
		totalEvents += count
	}

	if totalEvents == 0 {
		t.Error("No events recorded during random activity")
	} else {
		t.Logf("Total events recorded: %d", totalEvents)
	}

	// Check for bridge talker events specifically
	bridgeStartEvents := suite.GetEventsByType("bridge_talker_start")
	bridgeStopEvents := suite.GetEventsByType("bridge_talker_stop")

	t.Logf("Bridge talker events: %d starts, %d stops", len(bridgeStartEvents), len(bridgeStopEvents))

	if len(bridgeStartEvents) == 0 {
		t.Log("Warning: No bridge talker activity recorded - this may be expected for short duration tests")
	}
}

// TestNetworkStatus demonstrates network status monitoring
func TestNetworkStatus(t *testing.T) {
	// Simple test configuration
	config := DefaultTestConfig()
	config.RepeaterCount = 2
	config.BridgeCount = 1
	config.VerboseLogging = false

	suite := NewIntegrationTestSuite(config)

	err := suite.Setup(t)
	if err != nil {
		t.Fatalf("Failed to setup network status test: %v", err)
	}
	defer suite.Teardown(t)

	// Get initial network status
	repeaterStatus := suite.repeaterNetwork.GetNetworkStatus()
	bridgeStatus := suite.bridgeNetwork.GetNetworkStatus()

	t.Logf("Initial network status:")
	t.Logf("Repeaters: %d active", len(repeaterStatus))
	t.Logf("Bridges: %d active", len(bridgeStatus))

	// Verify all components are reported
	if len(repeaterStatus) != config.RepeaterCount {
		t.Errorf("Repeater count mismatch: got %d, want %d", len(repeaterStatus), config.RepeaterCount)
	}

	if len(bridgeStatus) != config.BridgeCount {
		t.Errorf("Bridge count mismatch: got %d, want %d", len(bridgeStatus), config.BridgeCount)
	}

	// Create some activity and check status changes
	bridge := suite.bridgeNetwork.GetBridge(suite.bridges[0])
	if bridge != nil {
		// Start a talker
		_, err := bridge.StartTalker("TEST01", "Test Location")
		if err != nil {
			t.Errorf("Failed to start test talker: %v", err)
		} else {
			// Get status with active talker
			updatedStatus := bridge.GetStatus()

			if updatedStatus["current_talker"] == nil {
				t.Error("Status should show current talker")
			}

			// Stop talker
			bridge.StopCurrentTalker()

			// Get final status
			finalStatus := bridge.GetStatus()

			if finalStatus["current_talker"] != nil {
				t.Error("Status should not show current talker after stop")
			}

			if finalStatus["talker_count"].(int) != 1 {
				t.Errorf("Status should show 1 talker in history, got %d", finalStatus["talker_count"])
			}
		}
	}
}

// BenchmarkMockNetwork benchmarks the mock network performance
func BenchmarkMockNetwork(b *testing.B) {
	// Create a simple test setup
	server, err := NewMockUDPServer("127.0.0.1:19999")
	if err != nil {
		b.Fatalf("Failed to create mock server: %v", err)
	}

	conn, err := server.AddConnection("127.0.0.1:20000")
	if err != nil {
		b.Fatalf("Failed to create connection: %v", err)
	}

	server.Start()
	defer server.Stop()

	// Benchmark packet sending
	packet := make([]byte, 155) // YSF packet size
	for i := range packet {
		packet[i] = byte(i % 256)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conn.Write(packet)
	}
}
