package ysf2dmr

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// Sample DMR ID database for testing
const testDMRDatabase = `DMRID,Callsign,Name,City,State,Country
1234567,W1ABC,John Doe,Boston,MA,United States
2345678,K2XYZ,Jane Smith,New York,NY,United States
`

func TestNewBridge(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})

	cfg := config.YSF2DMRConfig{
		Enabled: true,
		YSF: config.YSF2DMRYSFConfig{
			Callsign:     "TEST",
			LocalAddress: "127.0.0.1",
			LocalPort:    42001,
			HangTime:     5 * time.Second,
		},
		DMR: config.YSF2DMRDMRConfig{
			Enabled:   false, // Don't actually connect for unit test
			ID:        1234567,
			Network:   "Test",
			StartupTG: 91,
			Slot:      2,
		},
		Lookup: config.DMRLookupConfig{
			Enabled: false, // Disabled for basic test
		},
	}

	bridge, err := NewBridge(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	if bridge == nil {
		t.Fatal("Bridge should not be nil")
	}

	if !bridge.IsRunning() {
		// Bridge not started yet, this is expected
	}
}

func TestBridgeWithLookup(t *testing.T) {
	// Create temp database file
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "dmrids.csv")
	if err := os.WriteFile(dbFile, []byte(testDMRDatabase), 0644); err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})

	cfg := config.YSF2DMRConfig{
		Enabled: true,
		YSF: config.YSF2DMRYSFConfig{
			Callsign:     "TEST",
			LocalAddress: "127.0.0.1",
			LocalPort:    42002,
			HangTime:     5 * time.Second,
		},
		DMR: config.YSF2DMRDMRConfig{
			Enabled:   false,
			ID:        1234567,
			StartupTG: 91,
			Slot:      2,
		},
		Lookup: config.DMRLookupConfig{
			Enabled:      true,
			DMRIDFile:    dbFile,
			AutoDownload: false,
		},
	}

	bridge, err := NewBridge(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := bridge.Start(ctx); err != nil {
		t.Fatalf("Failed to start bridge: %v", err)
	}

	// Verify lookup is working
	if bridge.lookup == nil {
		t.Fatal("Lookup should be initialized")
	}

	if count := bridge.lookup.Count(); count != 2 {
		t.Errorf("Expected 2 entries in lookup, got %d", count)
	}

	// Stop bridge
	if err := bridge.Stop(); err != nil {
		t.Errorf("Failed to stop bridge: %v", err)
	}
}

func TestCallStateTracking(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})

	cfg := config.YSF2DMRConfig{
		YSF: config.YSF2DMRYSFConfig{
			HangTime: 5 * time.Second,
		},
		DMR: config.YSF2DMRDMRConfig{
			ID:        1234567,
			StartupTG: 91,
			Slot:      2,
		},
	}

	bridge, _ := NewBridge(cfg, log)

	// Initially no active call
	if call := bridge.GetActiveCall(); call != nil {
		t.Error("Should have no active call initially")
	}

	// Simulate starting a call
	bridge.mu.Lock()
	bridge.activeCall = &CallState{
		Direction:    DirectionYSFToDMR,
		YSFCallsign:  "W1ABC",
		DMRID:        1234567,
		TalkGroup:    91,
		Slot:         2,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		StreamID:     12345,
	}
	bridge.mu.Unlock()

	// Verify call is active
	call := bridge.GetActiveCall()
	if call == nil {
		t.Fatal("Should have active call")
	}

	if call.Direction != DirectionYSFToDMR {
		t.Errorf("Expected YSF→DMR direction, got %v", call.Direction)
	}

	if call.YSFCallsign != "W1ABC" {
		t.Errorf("Expected callsign W1ABC, got %s", call.YSFCallsign)
	}

	// End the call
	bridge.endCall()

	// Verify call ended
	if call := bridge.GetActiveCall(); call != nil {
		t.Error("Call should be ended")
	}
}

func TestStatistics(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})

	cfg := config.YSF2DMRConfig{
		DMR: config.YSF2DMRDMRConfig{
			ID:        1234567,
			StartupTG: 91,
		},
	}

	bridge, _ := NewBridge(cfg, log)

	// Initial stats should be zero
	stats := bridge.GetStatistics()
	if stats.TotalCalls != 0 {
		t.Errorf("Expected 0 total calls, got %d", stats.TotalCalls)
	}

	// Simulate some activity
	bridge.stats.mu.Lock()
	bridge.stats.TotalCalls = 5
	bridge.stats.YSFToDMRCalls = 3
	bridge.stats.DMRToYSFCalls = 2
	bridge.stats.YSFPackets = 100
	bridge.stats.DMRPackets = 150
	bridge.stats.mu.Unlock()

	// Verify stats
	stats = bridge.GetStatistics()
	if stats.TotalCalls != 5 {
		t.Errorf("Expected 5 total calls, got %d", stats.TotalCalls)
	}
	if stats.YSFToDMRCalls != 3 {
		t.Errorf("Expected 3 YSF→DMR calls, got %d", stats.YSFToDMRCalls)
	}
	if stats.DMRToYSFCalls != 2 {
		t.Errorf("Expected 2 DMR→YSF calls, got %d", stats.DMRToYSFCalls)
	}
	if stats.YSFPackets != 100 {
		t.Errorf("Expected 100 YSF packets, got %d", stats.YSFPackets)
	}
	if stats.DMRPackets != 150 {
		t.Errorf("Expected 150 DMR packets, got %d", stats.DMRPackets)
	}
}

func TestExtractYSFCallsign(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "Valid callsign",
			data:     []byte("YSFDW1ABC     extra data"),
			expected: "W1ABC",
		},
		{
			name:     "Callsign with spaces",
			data:     []byte("YSFDK2XYZ   extra"),
			expected: "K2XYZ",
		},
		{
			name:     "Short packet",
			data:     []byte("YSF"),
			expected: "",
		},
		{
			name:     "Callsign with null",
			data:     append([]byte("YSFDN3QRS\x00\x00\x00\x00"), []byte("extra")...),
			expected: "N3QRS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callsign := extractYSFCallsign(tt.data)
			if callsign != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, callsign)
			}
		})
	}
}

func TestCallDirectionString(t *testing.T) {
	tests := []struct {
		direction CallDirection
		expected  string
	}{
		{DirectionNone, "None"},
		{DirectionYSFToDMR, "YSF→DMR"},
		{DirectionDMRToYSF, "DMR→YSF"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.direction.String(); got != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestCallTimeout(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})

	cfg := config.YSF2DMRConfig{
		YSF: config.YSF2DMRYSFConfig{
			HangTime: 100 * time.Millisecond, // Short timeout for testing
		},
		DMR: config.YSF2DMRDMRConfig{
			ID:        1234567,
			StartupTG: 91,
			Slot:      2,
		},
	}

	bridge, _ := NewBridge(cfg, log)

	// Start a call
	bridge.mu.Lock()
	bridge.activeCall = &CallState{
		Direction:    DirectionYSFToDMR,
		YSFCallsign:  "W1ABC",
		DMRID:        1234567,
		StartTime:    time.Now(),
		LastActivity: time.Now().Add(-200 * time.Millisecond), // Activity in the past
	}
	bridge.mu.Unlock()

	// Check for timeout
	bridge.checkCallTimeout()

	// Call should be ended
	if call := bridge.GetActiveCall(); call != nil {
		t.Error("Call should have timed out")
	}
}

func TestConcurrentStatistics(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	cfg := config.YSF2DMRConfig{
		DMR: config.YSF2DMRDMRConfig{ID: 1234567},
	}

	bridge, _ := NewBridge(cfg, log)

	// Run concurrent statistics updates
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				bridge.stats.mu.Lock()
				bridge.stats.TotalCalls++
				bridge.stats.YSFPackets++
				bridge.stats.mu.Unlock()

				// Also read stats concurrently
				_ = bridge.GetStatistics()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify final count
	stats := bridge.GetStatistics()
	if stats.TotalCalls != 500 {
		t.Errorf("Expected 500 total calls, got %d", stats.TotalCalls)
	}
	if stats.YSFPackets != 500 {
		t.Errorf("Expected 500 YSF packets, got %d", stats.YSFPackets)
	}
}

func BenchmarkGetStatistics(b *testing.B) {
	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	cfg := config.YSF2DMRConfig{
		DMR: config.YSF2DMRDMRConfig{ID: 1234567},
	}

	bridge, _ := NewBridge(cfg, log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bridge.GetStatistics()
	}
}

func BenchmarkGetActiveCall(b *testing.B) {
	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	cfg := config.YSF2DMRConfig{
		DMR: config.YSF2DMRDMRConfig{ID: 1234567},
	}

	bridge, _ := NewBridge(cfg, log)

	// Set up an active call
	bridge.mu.Lock()
	bridge.activeCall = &CallState{
		Direction:   DirectionYSFToDMR,
		YSFCallsign: "W1ABC",
		DMRID:       1234567,
		StartTime:   time.Now(),
	}
	bridge.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bridge.GetActiveCall()
	}
}
