package bridge

import (
	"net"
	"testing"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// MockSimpleServer implements NetworkServer for testing (keeps a different name
// to avoid colliding with other test mocks in the package).
type MockSimpleServer struct{}

// SendPacket satisfies the NetworkServer interface used by the bridge manager.
func (m *MockSimpleServer) SendPacket(data []byte, addr *net.UDPAddr) error {
	// No-op for testing
	return nil
}

// GetListenAddress satisfies the NetworkServer interface used by the bridge manager.
func (m *MockSimpleServer) GetListenAddress() *net.UDPAddr {
	addr, _ := net.ResolveUDPAddr("udp", ":4200")
	return addr
}

// TestBridgeTypeDetection tests that the manager correctly identifies bridge types
func TestBridgeTypeDetection(t *testing.T) {
	tests := []struct {
		name       string
		config     config.BridgeConfig
		expectType string
	}{
		{
			name: "YSF bridge with explicit type",
			config: config.BridgeConfig{
				Name:    "Test-YSF",
				Type:    "ysf",
				Host:    "localhost",
				Port:    42000,
				Enabled: true,
			},
			expectType: "ysf",
		},
		{
			name: "YSF bridge without type (defaults to ysf)",
			config: config.BridgeConfig{
				Name:    "Test-YSF-Default",
				Host:    "localhost",
				Port:    42000,
				Enabled: true,
			},
			expectType: "ysf",
		},
		{
			name: "DMR bridge",
			config: config.BridgeConfig{
				Name:    "Test-DMR",
				Type:    "dmr",
				Enabled: true,
				DMR: &config.DMRBridgeConfig{
					ID:        1234567,
					Network:   "Test",
					Address:   "localhost",
					Port:      62031,
					Password:  "test",
					TalkGroup: 91,
					Slot:      2,
					ColorCode: 1,
				},
			},
			expectType: "dmr",
		},
	}

	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "text",
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &MockSimpleServer{}
			manager := NewManager([]config.BridgeConfig{tt.config}, server, log)

			err := manager.Start()
			if err != nil {
				t.Fatalf("Failed to start manager: %v", err)
			}
			defer manager.Stop()

			// Give it a moment to initialize
			time.Sleep(100 * time.Millisecond)

			status := manager.GetStatus()
			if len(status) != 1 {
				t.Fatalf("Expected 1 bridge, got %d", len(status))
			}

			bridgeStatus := status[tt.config.Name]
			if bridgeStatus.Type != tt.expectType {
				t.Errorf("Expected type %s, got %s", tt.expectType, bridgeStatus.Type)
			}
		})
	}
}

// TestMixedBridgeConfiguration tests a configuration with both YSF and DMR bridges
func TestMixedBridgeConfiguration(t *testing.T) {
	configs := []config.BridgeConfig{
		{
			Name:    "YSF-Bridge-1",
			Type:    "ysf",
			Host:    "localhost",
			Port:    42001,
			Enabled: true,
		},
		{
			Name:    "DMR-Bridge-1",
			Type:    "dmr",
			Enabled: true,
			DMR: &config.DMRBridgeConfig{
				ID:        1234567,
				Network:   "BrandMeister",
				Address:   "localhost",
				Port:      62031,
				Password:  "test",
				TalkGroup: 91,
				Slot:      2,
				ColorCode: 1,
			},
		},
		{
			Name:    "YSF-Bridge-2",
			Type:    "ysf",
			Host:    "localhost",
			Port:    42002,
			Enabled: true,
		},
		{
			Name:    "DMR-Bridge-2",
			Type:    "dmr",
			Enabled: true,
			DMR: &config.DMRBridgeConfig{
				ID:        7654321,
				Network:   "TGIF",
				Address:   "localhost",
				Port:      62032,
				Password:  "test",
				TalkGroup: 310,
				Slot:      2,
				ColorCode: 1,
			},
		},
	}

	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "text",
	})

	server := &MockSimpleServer{}
	manager := NewManager(configs, server, log)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Give it a moment to initialize
	time.Sleep(100 * time.Millisecond)

	status := manager.GetStatus()

	if len(status) != 4 {
		t.Fatalf("Expected 4 bridges, got %d", len(status))
	}

	// Count bridge types
	ysfCount := 0
	dmrCount := 0
	for _, bridgeStatus := range status {
		switch bridgeStatus.Type {
		case "ysf":
			ysfCount++
		case "dmr":
			dmrCount++
		default:
			t.Errorf("Unknown bridge type: %s", bridgeStatus.Type)
		}
	}

	if ysfCount != 2 {
		t.Errorf("Expected 2 YSF bridges, got %d", ysfCount)
	}
	if dmrCount != 2 {
		t.Errorf("Expected 2 DMR bridges, got %d", dmrCount)
	}
}

// TestDMRBridgeMetadata tests that DMR bridges expose metadata correctly
func TestDMRBridgeMetadata(t *testing.T) {
	cfg := config.BridgeConfig{
		Name:    "Test-DMR",
		Type:    "dmr",
		Enabled: true,
		DMR: &config.DMRBridgeConfig{
			ID:        1234567,
			Network:   "BrandMeister",
			Address:   "localhost",
			Port:      62031,
			Password:  "test",
			TalkGroup: 91,
			Slot:      2,
			ColorCode: 1,
		},
	}

	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "text",
	})

	server := &MockSimpleServer{}
	manager := NewManager([]config.BridgeConfig{cfg}, server, log)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Give it a moment to initialize
	time.Sleep(100 * time.Millisecond)

	status := manager.GetStatus()
	bridgeStatus := status["Test-DMR"]

	if bridgeStatus.Type != "dmr" {
		t.Errorf("Expected type dmr, got %s", bridgeStatus.Type)
	}

	if bridgeStatus.Metadata == nil {
		t.Fatal("Expected metadata to be non-nil for DMR bridge")
	}

	// Check metadata fields
	expectedFields := map[string]interface{}{
		"dmr_network": "BrandMeister",
		"talk_group":  uint32(91),
		"dmr_id":      uint32(1234567),
		"slot":        uint8(2),
	}

	for field, expectedValue := range expectedFields {
		if value, ok := bridgeStatus.Metadata[field]; !ok {
			t.Errorf("Expected metadata field %s to exist", field)
		} else if value != expectedValue {
			t.Errorf("Expected metadata field %s to be %v, got %v", field, expectedValue, value)
		}
	}
}

// TestScheduledBridgeConfiguration tests scheduled bridge setup
func TestScheduledBridgeConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		config      config.BridgeConfig
		expectType  string
		hasSchedule bool
		isPermanent bool
	}{
		{
			name: "Scheduled YSF bridge",
			config: config.BridgeConfig{
				Name:     "Scheduled-YSF",
				Type:     "ysf",
				Host:     "localhost",
				Port:     42000,
				Schedule: "0 */1 * * *",
				Duration: 10 * time.Minute,
				Enabled:  true,
			},
			expectType:  "ysf",
			hasSchedule: true,
			isPermanent: false,
		},
		{
			name: "Permanent YSF bridge",
			config: config.BridgeConfig{
				Name:      "Permanent-YSF",
				Type:      "ysf",
				Host:      "localhost",
				Port:      42000,
				Permanent: true,
				Enabled:   true,
			},
			expectType:  "ysf",
			hasSchedule: false,
			isPermanent: true,
		},
		{
			name: "Scheduled DMR bridge",
			config: config.BridgeConfig{
				Name:     "Scheduled-DMR",
				Type:     "dmr",
				Schedule: "0 */2 * * *",
				Duration: 15 * time.Minute,
				Enabled:  true,
				DMR: &config.DMRBridgeConfig{
					ID:        1234567,
					Network:   "Test",
					Address:   "localhost",
					Port:      62031,
					Password:  "test",
					TalkGroup: 91,
					Slot:      2,
					ColorCode: 1,
				},
			},
			expectType:  "dmr",
			hasSchedule: true,
			isPermanent: false,
		},
		{
			name: "Permanent DMR bridge",
			config: config.BridgeConfig{
				Name:      "Permanent-DMR",
				Type:      "dmr",
				Permanent: true,
				Enabled:   true,
				DMR: &config.DMRBridgeConfig{
					ID:        7654321,
					Network:   "Test",
					Address:   "localhost",
					Port:      62031,
					Password:  "test",
					TalkGroup: 310,
					Slot:      2,
					ColorCode: 1,
				},
			},
			expectType:  "dmr",
			hasSchedule: false,
			isPermanent: true,
		},
	}

	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "text",
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &MockSimpleServer{}
			manager := NewManager([]config.BridgeConfig{tt.config}, server, log)

			err := manager.Start()
			if err != nil {
				t.Fatalf("Failed to start manager: %v", err)
			}
			defer manager.Stop()

			// Give it a moment to initialize
			time.Sleep(100 * time.Millisecond)

			status := manager.GetStatus()
			bridgeStatus := status[tt.config.Name]

			if bridgeStatus.Type != tt.expectType {
				t.Errorf("Expected type %s, got %s", tt.expectType, bridgeStatus.Type)
			}

			if tt.hasSchedule {
				if bridgeStatus.NextSchedule == nil {
					t.Error("Expected NextSchedule to be set for scheduled bridge")
				}
				if bridgeStatus.Duration != tt.config.Duration {
					t.Errorf("Expected duration %v, got %v", tt.config.Duration, bridgeStatus.Duration)
				}
			}

			if tt.isPermanent {
				// Permanent bridges may not have NextSchedule set
				if bridgeStatus.Duration != 0 {
					t.Error("Expected duration to be 0 for permanent bridge")
				}
			}
		})
	}
}

// TestBridgeRunnerInterface tests that both YSF and DMR bridges implement the interface
func TestBridgeRunnerInterface(t *testing.T) {
	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "text",
	})

	// Create a YSF bridge
	ysfConfig := config.BridgeConfig{
		Name:    "Test-YSF",
		Type:    "ysf",
		Host:    "localhost",
		Port:    42000,
		Enabled: true,
	}

	server := &MockSimpleServer{}
	ysfBridge := NewBridge(ysfConfig, server, log)

	// Test YSF bridge implements interface
	var _ BridgeRunner = ysfBridge

	if ysfBridge.GetType() != "ysf" {
		t.Errorf("Expected YSF bridge type to be 'ysf', got '%s'", ysfBridge.GetType())
	}

	if ysfBridge.GetName() != "Test-YSF" {
		t.Errorf("Expected name 'Test-YSF', got '%s'", ysfBridge.GetName())
	}

	// Create a DMR bridge
	dmrConfig := config.BridgeConfig{
		Name:    "Test-DMR",
		Type:    "dmr",
		Enabled: true,
		DMR: &config.DMRBridgeConfig{
			ID:        1234567,
			Network:   "Test",
			Address:   "localhost",
			Port:      62031,
			Password:  "test",
			TalkGroup: 91,
			Slot:      2,
			ColorCode: 1,
		},
	}

	dmrBridge, err := NewDMRBridgeAdapter(dmrConfig, log)
	if err != nil {
		t.Fatalf("Failed to create DMR bridge: %v", err)
	}

	// Test DMR bridge implements interface
	var _ BridgeRunner = dmrBridge

	if dmrBridge.GetType() != "dmr" {
		t.Errorf("Expected DMR bridge type to be 'dmr', got '%s'", dmrBridge.GetType())
	}

	if dmrBridge.GetName() != "Test-DMR" {
		t.Errorf("Expected name 'Test-DMR', got '%s'", dmrBridge.GetName())
	}
}

// TestInvalidBridgeType tests that invalid bridge types are rejected
func TestInvalidBridgeType(t *testing.T) {
	cfg := config.BridgeConfig{
		Name:    "Invalid-Bridge",
		Type:    "invalid",
		Enabled: true,
	}

	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "text",
	})

	server := &MockSimpleServer{}
	manager := NewManager([]config.BridgeConfig{cfg}, server, log)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Manager.Start() failed: %v", err)
	}
	defer manager.Stop()

	// Give it a moment to try initialization
	time.Sleep(100 * time.Millisecond)

	status := manager.GetStatus()

	// Bridge with invalid type should not be created
	if len(status) != 0 {
		t.Errorf("Expected no bridges to be created for invalid type, got %d", len(status))
	}
}

// TestDMRBridgeWithoutConfig tests that DMR bridge requires DMR config
func TestDMRBridgeWithoutConfig(t *testing.T) {
	cfg := config.BridgeConfig{
		Name:    "DMR-No-Config",
		Type:    "dmr",
		Enabled: true,
		// DMR config is nil
	}

	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "text",
	})

	server := &MockSimpleServer{}
	manager := NewManager([]config.BridgeConfig{cfg}, server, log)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Manager.Start() failed: %v", err)
	}
	defer manager.Stop()

	// Give it a moment to try initialization
	time.Sleep(100 * time.Millisecond)

	status := manager.GetStatus()

	// Bridge should not be created without DMR config
	if len(status) != 0 {
		t.Errorf("Expected no bridges to be created without DMR config, got %d", len(status))
	}
}

// TestBridgeLifecycle tests basic lifecycle operations
func TestBridgeLifecycle(t *testing.T) {
	cfg := config.BridgeConfig{
		Name:    "Test-YSF-Lifecycle",
		Type:    "ysf",
		Host:    "localhost",
		Port:    42000,
		Enabled: true,
	}

	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "text",
	})

	server := &MockSimpleServer{}
	manager := NewManager([]config.BridgeConfig{cfg}, server, log)

	// Start manager
	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	status := manager.GetStatus()
	if len(status) != 1 {
		t.Fatalf("Expected 1 bridge, got %d", len(status))
	}

	// Stop manager
	manager.Stop()

	// After stop, status should still be available
	status = manager.GetStatus()
	if len(status) != 1 {
		t.Errorf("Expected status to still be available after stop, got %d bridges", len(status))
	}
}

// TestConfigurationLoading tests loading config from file
func TestConfigurationLoading(t *testing.T) {
	cfg, err := config.Load("../../configs/test-mixed-bridges.yaml")
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	if len(cfg.Bridges) == 0 {
		t.Fatal("Expected bridges to be configured")
	}

	// Count bridge types
	ysfCount := 0
	dmrCount := 0
	for _, bridge := range cfg.Bridges {
		bridgeType := bridge.Type
		if bridgeType == "" {
			bridgeType = "ysf" // default
		}

		switch bridgeType {
		case "ysf":
			ysfCount++
		case "dmr":
			dmrCount++
		default:
			t.Errorf("Unknown bridge type in config: %s", bridgeType)
		}
	}

	if ysfCount == 0 {
		t.Error("Expected at least one YSF bridge in test config")
	}
	if dmrCount == 0 {
		t.Error("Expected at least one DMR bridge in test config")
	}

	t.Logf("Loaded config with %d YSF bridges and %d DMR bridges", ysfCount, dmrCount)
}

// Benchmark tests
func BenchmarkMixedBridgeCreation(b *testing.B) {
	configs := []config.BridgeConfig{
		{
			Name:    "YSF-1",
			Type:    "ysf",
			Host:    "localhost",
			Port:    42000,
			Enabled: true,
		},
		{
			Name:    "DMR-1",
			Type:    "dmr",
			Enabled: true,
			DMR: &config.DMRBridgeConfig{
				ID:        1234567,
				Network:   "Test",
				Address:   "localhost",
				Port:      62031,
				Password:  "test",
				TalkGroup: 91,
				Slot:      2,
				ColorCode: 1,
			},
		},
	}

	log, _ := logger.New(logger.Config{
		Level:  "error",
		Format: "text",
	})

	server := &MockSimpleServer{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager := NewManager(configs, server, log)
		_ = manager.Start()
		manager.Stop()
	}
}
