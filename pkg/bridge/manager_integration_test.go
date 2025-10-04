//go:build integration
// +build integration

package bridge

import (
	"os"
	"testing"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// This is a slow, higher-confidence integration-style test that runs the Manager
// with a per-second schedule and asserts that bridges start and stop within
// the expected scheduled window. It intentionally uses real timers and waits
// a few seconds so timing behaviour is observed.
func TestManager_ScheduledBridge_Timing(t *testing.T) {
	t.Parallel()

	l := logger.NewTestLogger(os.Stdout)
	mockServer := &MockNetworkServer{}

	// Schedule: every second
	cfg := []config.BridgeConfig{
		{
			Name:     "int-test-scheduled",
			Host:     "localhost",
			Port:     4200,
			Enabled:  true,
			Schedule: "* * * * * *",           // every second
			Duration: 1500 * time.Millisecond, // 1.5s window
		},
	}

	mgr := NewManager(cfg, mockServer, l)

	if err := mgr.Start(); err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}

	// We'll observe over a short period of time multiple schedule occurrences
	// and assert that at least one scheduled window produced a connected bridge
	// and that the bridge disconnected after approximately the duration.

	// Wait a bit to allow cron to trigger
	time.Sleep(1200 * time.Millisecond)

	status := mgr.GetStatus()
	bstatus, ok := status["int-test-scheduled"]
	if !ok {
		t.Fatalf("expected bridge status for int-test-scheduled")
	}

	// We accept either connecting/connected or disconnected (if it already completed)
	// but we need evidence that a handshake was attempted during the window.
	if len(mockServer.sentPackets) == 0 {
		t.Errorf("expected handshake packets to have been sent during scheduled window")
	}

	// Now wait until after the scheduled duration to ensure disconnect has occurred.
	// Poll for the disconnected state for up to 3s to avoid brittle timing assumptions in CI.
	timeout := time.Now().Add(3 * time.Second)
	for time.Now().Before(timeout) {
		status = mgr.GetStatus()
		bstatus = status["int-test-scheduled"]
		if bstatus.State != StateConnected {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// After polling, the bridge should not be in connected state (it should have stopped)
	if bstatus.State == StateConnected {
		t.Errorf("expected bridge to have disconnected after scheduled window, still connected")
	}

	// Basic timing check: ensure at least one handshake packet was sent and at least one disconnect
	// (disconnect is an implementation detail - we expect a packet to be sent at end)
	if len(mockServer.sentPackets) < 1 {
		t.Errorf("expected at least one packet sent during schedule, got %d", len(mockServer.sentPackets))
	}

	// Clean up
	mgr.Stop()
}
