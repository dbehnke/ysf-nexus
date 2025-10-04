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

// Test that the manager recovers a missed schedule when starting up during a scheduled window
func TestManager_MissedSchedule_Recovers(t *testing.T) {
	t.Parallel()

	l := logger.NewTestLogger(os.Stdout)
	mockServer := &MockNetworkServer{}

	// Schedule every second with a longer duration so starting the manager falls inside the window
	cfg := []config.BridgeConfig{
		{
			Name:     "missed-recover",
			Host:     "localhost",
			Port:     4200,
			Enabled:  true,
			Schedule: "* * * * * *",
			Duration: 3 * time.Second,
		},
	}

	mgr := NewManager(cfg, mockServer, l)

	// Delay slightly so we start the manager close to a schedule boundary
	time.Sleep(900 * time.Millisecond)

	if err := mgr.Start(); err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}

	// Wait briefly to allow missed-schedule recovery to run
	time.Sleep(500 * time.Millisecond)

	// We expect the manager to have attempted to start the scheduled bridge (handshake packet)
	if len(mockServer.sentPackets) == 0 {
		t.Fatalf("expected manager to recover missed schedule and send packets")
	}

	mgr.Stop()
}

// Test that the manager does not attempt to recover a missed schedule if the bridge is already active
func TestManager_MissedSchedule_DoesNotRecoverIfActive(t *testing.T) {
	t.Parallel()

	l := logger.NewTestLogger(os.Stdout)
	mockServer := &MockNetworkServer{}

	// Create a bridge configured as permanent and also with a schedule; permanent should take precedence
	cfg := []config.BridgeConfig{
		{
			Name:      "missed-no-recover",
			Host:      "localhost",
			Port:      4200,
			Enabled:   true,
			Permanent: true,
			Schedule:  "* * * * * *",
			Duration:  2 * time.Second,
		},
	}

	mgr := NewManager(cfg, mockServer, l)
	if err := mgr.Start(); err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}

	// Wait for the permanent bridge to connect
	time.Sleep(200 * time.Millisecond)

	// Clear any packets sent during startup
	mockServer.sentPackets = nil

	// Trigger the schedule checker (it runs every minute in production, but missed schedule logic also runs
	// during setup; we'll wait briefly and then assert no new scheduled-start packets were added)
	time.Sleep(1200 * time.Millisecond)

	if len(mockServer.sentPackets) != 0 {
		t.Fatalf("expected no scheduled recovery when bridge is already active, but saw packets")
	}

	mgr.Stop()
}
