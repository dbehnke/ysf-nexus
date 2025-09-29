# YSF Nexus Integration Testing Framework

This directory contains a comprehensive testing framework for YSF Nexus that allows testing all system components without requiring real network infrastructure.

## Overview

The testing framework provides:

- **Mock Network Infrastructure**: UDP servers, connections, and packet handling
- **Mock YSF Repeaters**: Simulated repeaters with client connections and packet routing
- **Mock Bridge Endpoints**: Simulated bridge connections with talker management
- **Integration Test Suite**: Comprehensive test scenarios for all components
- **Bridge Talker Test Scenarios**: Specific testing for bridge talker functionality

## Architecture

### Core Components

1. **`mock_network.go`**: Foundation UDP networking simulation
   - `MockUDPConn`: Simulates UDP connections with packet injection
   - `MockUDPServer`: Manages multiple mock connections

2. **`mock_repeater.go`**: YSF repeater simulation
   - `MockYSFRepeater`: Complete repeater with client management
   - `MockRepeaterClient`: Simulated radio clients
   - `MockYSFRepeaterNetwork`: Network of interconnected repeaters

3. **`mock_bridge.go`**: Bridge endpoint simulation
   - `MockBridgeEndpoint`: Bridge with talker management
   - `BridgeTalker`: Represents active/historical talkers
   - `MockBridgeNetwork`: Multiple bridge coordination

4. **`integration_test_suite.go`**: Comprehensive test orchestration
   - `IntegrationTestSuite`: Main test coordinator
   - Event tracking and monitoring
   - API integration testing

5. **`bridge_talker_scenarios.go`**: Specialized bridge talker tests
   - Single/multiple talker scenarios
   - Duration tracking validation
   - Talker interruption handling
   - High-frequency activity testing

## Quick Start

### Basic Usage

```go
func TestYourFeature(t *testing.T) {
    // Create test configuration
    config := testhelpers.DefaultTestConfig()
    config.RepeaterCount = 2
    config.BridgeCount = 1
    config.VerboseLogging = true
    
    // Set up test suite
    suite := testhelpers.NewIntegrationTestSuite(config)
    err := suite.Setup(t)
    if err != nil {
        t.Fatalf("Setup failed: %v", err)
    }
    defer suite.Teardown(t)
    
    // Run your tests...
    suite.TestBasicConnectivity(t)
    suite.TestBridgeFunctionality(t)
}
```

### Bridge Talker Testing

```go
func TestBridgeTalkers(t *testing.T) {
    config := testhelpers.DefaultTestConfig()
    config.BridgeCount = 2
    
    suite := testhelpers.NewIntegrationTestSuite(config)
    suite.Setup(t)
    defer suite.Teardown(t)
    
    // Create bridge talker scenario tester
    scenarios := testhelpers.NewBridgeTalkerTestScenarios(suite)
    scenarios.RunAllBridgeTalkerTests(t)
}
```

## Test Scenarios

### Basic Connectivity
- Verifies all mock components start correctly
- Tests component status reporting
- Validates network addresses and configurations

### Repeater Functionality
- Client connections and disconnections
- Voice packet routing and forwarding
- Repeater linking and unlinking
- Multi-client talking sequences

### Bridge Functionality
- Bridge talker start/stop cycles
- Duration tracking accuracy
- Talker history management
- Concurrent bridge operations

### Bridge Talker Scenarios
- **Single Talker**: Basic talker lifecycle
- **Multiple Talkers**: Concurrent bridge activities
- **Sequencing**: Proper talker ordering
- **Interruption**: Talker preemption handling
- **Duration Tracking**: Accurate timing measurements
- **High Frequency**: Rapid talker changes

### API Integration
- Tests API endpoints with mock data
- Validates JSON response formats
- Checks error handling and timeouts

### Complex Scenarios
- Multi-component interactions
- Linked repeater networks
- Bridge network coordination

## Configuration Options

```go
type TestConfig struct {
    RepeaterCount    int           // Number of mock repeaters
    BridgeCount      int           // Number of mock bridges  
    BasePort         int           // Starting port for mock services
    APIBaseURL       string        // YSF Nexus API endpoint
    APITimeout       time.Duration // API call timeout
    ActivityDuration time.Duration // Random activity duration
    MaxTalkTime      time.Duration // Maximum talker duration
    MinTalkTime      time.Duration // Minimum talker duration
    VerboseLogging   bool          // Enable detailed logging
}
```

## Running Tests

### All Integration Tests
```bash
go test ./internal/testhelpers -v
```

### Specific Test Suites
```bash
# Basic framework test
go test ./internal/testhelpers -v -run TestIntegrationFramework

# Bridge talker tests
go test ./internal/testhelpers -v -run TestBridgeTalkerScenarios

# Random activity simulation (longer running)
go test ./internal/testhelpers -v -run TestRandomActivity
```

### Short Test Mode
```bash
# Skip long-running tests
go test ./internal/testhelpers -v -short
```

## Mock Component Details

### Mock Repeaters

Mock repeaters simulate complete YSF repeater functionality:

```go
// Create and configure a mock repeater
repeater, err := NewMockYSFRepeater("REP001", "Test Repeater", "127.0.0.1:10000")
repeater.Start()

// Connect mock clients
client, err := repeater.ConnectClient("W9TRO", "Chicago, IL", "127.0.0.1:20001")

// Simulate talking
repeater.StartTalking(client.ID)
repeater.SendVoicePackets(client.ID, 10)  // Send 10 voice frames
repeater.StopTalking(client.ID)

// Link repeaters
repeater.LinkToRepeater("REP002")
```

### Mock Bridges

Mock bridges provide comprehensive bridge talker simulation:

```go
// Create mock bridge
bridge, err := NewMockBridgeEndpoint("BR001", "Test Bridge", 
    "127.0.0.1:11000", "127.0.0.1:12000")
bridge.Connect()

// Simulate bridge talkers
talker, err := bridge.StartTalker("W9TRO", "Chicago, IL")
bridge.SendVoicePackets(25)              // Send voice frames
bridge.StopCurrentTalker()

// Get talker information
current := bridge.GetCurrentTalker()     // Active talker
history := bridge.GetTalkerHistory(10)   // Recent talkers
```

### Random Activity Simulation

Generate realistic network activity:

```go
// Start random activity on all components
suite.StartRandomActivity(30 * time.Second)

// Or per-component activity
repeater.SimulateRandomActivity(duration)
bridge.SimulateRandomActivity(duration, []string{"W9TRO", "G0RDH"})
```

## Event Tracking

The framework tracks all network events for analysis:

```go
// Get event summary
summary := suite.GetEventSummary()
// Returns: map[string]int{"bridge_talker_start": 5, "packet_received": 120}

// Get specific event types
talkerEvents := suite.GetEventsByType("bridge_talker_start")

// Export events as JSON
events, err := suite.ExportEvents()
```

## Integration with Real System

### API Testing
The framework can test against a running YSF Nexus server:

```go
config.APIBaseURL = "http://localhost:8080"  // Your server URL

// API tests will attempt real HTTP calls
suite.TestAPIIntegration(t)
```

### Mock Data Injection
Inject mock events into real system components by implementing bridge interfaces that forward mock events to your actual bridge manager.

## Best Practices

### Test Organization
- Use subtests for logical groupings
- Set up and tear down properly
- Use appropriate timeouts for long operations

### Mock Configuration
- Start with minimal component counts for debugging
- Increase complexity gradually
- Use verbose logging during development

### Performance Testing
- Use benchmark tests for performance validation
- Monitor memory usage with large component counts
- Test timeout scenarios appropriately

### Error Handling
- Test failure scenarios (network drops, timeouts)
- Validate cleanup on abnormal termination
- Check resource leaks in long-running tests

## Examples

See `example_test.go` for complete usage examples including:
- Basic integration testing setup
- Bridge talker scenario testing
- Random activity simulation
- Network status monitoring
- Performance benchmarking

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Adjust `BasePort` if ports are in use
2. **Timing Issues**: Increase durations for slower systems
3. **Memory Usage**: Reduce component counts for resource-limited environments
4. **API Failures**: Expected when YSF Nexus server not running

### Debug Information

Enable verbose logging to see detailed component interactions:

```go
config.VerboseLogging = true
```

This will show:
- Component startup/shutdown events
- Client connections and disconnections
- Packet routing and forwarding
- Talker start/stop events
- API call attempts and responses

## Integration with CI/CD

The framework is designed for automated testing:

```bash
# Fast unit tests
go test ./internal/testhelpers -short -v

# Full integration suite (longer)
go test ./internal/testhelpers -v -timeout 5m

# Performance benchmarks
go test ./internal/testhelpers -bench=. -v
```

Set appropriate timeouts and use `-short` flag to skip long-running random activity tests in CI environments.