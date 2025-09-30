package bridge

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// MockNetworkServer implements NetworkServer for testing
type MockNetworkServer struct {
	sentPackets [][]byte
	sentAddrs   []*net.UDPAddr
}

func (m *MockNetworkServer) SendPacket(data []byte, addr *net.UDPAddr) error {
	m.sentPackets = append(m.sentPackets, data)
	m.sentAddrs = append(m.sentAddrs, addr)
	return nil
}

func (m *MockNetworkServer) GetListenAddress() *net.UDPAddr {
	addr, _ := net.ResolveUDPAddr("udp", ":4200")
	return addr
}

func TestBridgeManager_PermanentBridge(t *testing.T) {
	logger := logger.NewTestLogger(os.Stdout)
	mockServer := &MockNetworkServer{}
	
	// Configure a permanent bridge
	config := []config.BridgeConfig{
		{
			Name:       "test-permanent",
			Host:       "localhost",
			Port:       4200,
			Enabled:    true,
			Permanent:  true,
			MaxRetries: 3,
			RetryDelay: 1 * time.Second,
		},
	}
	
	manager := NewManager(config, mockServer, logger)
	
	// Start the manager
	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	
	// Wait a moment for the permanent bridge to start
	time.Sleep(100 * time.Millisecond)
	
	// Check that the bridge exists and is in connecting/connected state
	status := manager.GetStatus()
	bridgeStatus, exists := status["test-permanent"]
	if !exists {
		t.Fatalf("Expected bridge 'test-permanent' to exist")
	}
	
	if bridgeStatus.State != StateConnecting && bridgeStatus.State != StateConnected {
		t.Errorf("Expected bridge to be connecting or connected, got %s", bridgeStatus.State)
	}
	
	// Verify that packets were sent (handshake)
	if len(mockServer.sentPackets) == 0 {
		t.Errorf("Expected at least one packet to be sent for handshake")
	}
	
	// Stop the manager
	manager.Stop()
}

func TestBridgeManager_ScheduledBridge(t *testing.T) {
	logger := logger.NewTestLogger(os.Stdout)
	mockServer := &MockNetworkServer{}
	
	// Configure a scheduled bridge that runs every second for 2 seconds
	config := []config.BridgeConfig{
		{
			Name:     "test-scheduled",
			Host:     "localhost",
			Port:     4200,
			Enabled:  true,
			Schedule: "* * * * * *", // Every second
			Duration: 2 * time.Second,
		},
	}
	
	manager := NewManager(config, mockServer, logger)
	
	// Start the manager
	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	
	// Wait for the schedule to trigger
	time.Sleep(1200 * time.Millisecond)
	
	// Check that the bridge exists
	status := manager.GetStatus()
	bridgeStatus, exists := status["test-scheduled"]
	if !exists {
		t.Fatalf("Expected bridge 'test-scheduled' to exist")
	}
	
	// The bridge should have been triggered by now
	if bridgeStatus.State == StateDisconnected {
		t.Logf("Bridge state: %s (may have already completed)", bridgeStatus.State)
	}
	
	// Stop the manager
	manager.Stop()
}

func TestBridge_ConnectionRetry(t *testing.T) {
	logger := logger.NewTestLogger(os.Stdout)
	mockServer := &MockNetworkServer{}
	
	// Configure a bridge with retry settings
	config := config.BridgeConfig{
		Name:       "test-retry",
		Host:       "invalid-host", // This will fail to resolve
		Port:       4200,
		MaxRetries: 2,
		RetryDelay: 100 * time.Millisecond,
	}
	
	bridge := NewBridge(config, mockServer, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	
	// This should fail and retry
	go bridge.RunPermanent(ctx)
	
	// Wait for retries to occur
	time.Sleep(300 * time.Millisecond)
	
	status := bridge.GetStatus()
	
	// Should have failed after retries
	if status.State != StateFailed {
		t.Errorf("Expected bridge state to be failed after retries, got %s", status.State)
	}
	
	if status.RetryCount == 0 {
		t.Errorf("Expected retry count to be > 0, got %d", status.RetryCount)
	}
}

func TestBridge_Statistics(t *testing.T) {
	logger := logger.NewTestLogger(os.Stdout)
	mockServer := &MockNetworkServer{}

	config := config.BridgeConfig{
		Name: "test-stats",
		Host: "localhost",
		Port: 4200,
	}

	bridge := NewBridge(config, mockServer, logger)

	// Simulate receiving packets
	bridge.IncrementRxStats(100)
	bridge.IncrementRxStats(200)

	status := bridge.GetStatus()

	if status.PacketsRx != 2 {
		t.Errorf("Expected 2 RX packets, got %d", status.PacketsRx)
	}

	if status.BytesRx != 300 {
		t.Errorf("Expected 300 RX bytes, got %d", status.BytesRx)
	}
}

// TestBridge_ScheduledDisconnectDeadlock tests for the double-lock deadlock bug
// where disconnect() holds b.mu and calls sendDisconnect() which tries to acquire b.mu again
func TestBridge_ScheduledDisconnectDeadlock(t *testing.T) {
	logger := logger.NewTestLogger(os.Stdout)
	mockServer := &MockNetworkServer{}

	config := config.BridgeConfig{
		Name:     "test-deadlock",
		Host:     "localhost",
		Port:     4200,
		Schedule: "",
		Duration: 200 * time.Millisecond, // Short duration to trigger disconnect quickly
	}

	bridge := NewBridge(config, mockServer, logger)

	// Create a context with a timeout
	ctx := context.Background()

	// Start the scheduled bridge in a goroutine
	done := make(chan bool)
	go func() {
		bridge.RunScheduled(ctx, config.Duration)
		done <- true
	}()

	// Wait for the bridge to complete or timeout
	select {
	case <-done:
		// Success - bridge completed without deadlock
		t.Log("Bridge completed successfully without deadlock")
	case <-time.After(2 * time.Second):
		// This indicates a deadlock - the bridge never completed
		t.Fatal("Bridge disconnect deadlocked - timeout waiting for completion")
	}

	// Verify that disconnect packet was sent
	if len(mockServer.sentPackets) == 0 {
		t.Error("Expected disconnect packet to be sent")
	}
}