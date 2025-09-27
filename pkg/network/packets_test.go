package network

import (
	"net"
	"testing"
	"time"
)

func TestParsePacket(t *testing.T) {
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 42000}

	tests := []struct {
		name      string
		data      []byte
		expectErr bool
		expectType string
	}{
		{
			name:       "Valid poll packet",
			data:       []byte("YSFPW1ABC     "),
			expectErr:  false,
			expectType: PacketTypePoll,
		},
		{
			name:       "Too small packet",
			data:       []byte("YSF"),
			expectErr:  true,
		},
		{
			name:       "Invalid poll packet size",
			data:       []byte("YSFPW1ABC"),
			expectErr:  true,
		},
		{
			name:       "Unknown packet type",
			data:       []byte("UNKNW1ABC     "),
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packet, err := ParsePacket(tt.data, addr)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if packet.Type != tt.expectType {
				t.Errorf("Expected type %s, got %s", tt.expectType, packet.Type)
			}

			if packet.Source != addr {
				t.Errorf("Expected source %v, got %v", addr, packet.Source)
			}

			if time.Since(packet.Timestamp) > time.Second {
				t.Errorf("Timestamp seems too old")
			}
		})
	}
}

func TestCreatePollResponse(t *testing.T) {
	response := CreatePollResponse()

	if len(response) != PollPacketSize {
		t.Errorf("Expected response size %d, got %d", PollPacketSize, len(response))
	}

	if string(response[:4]) != PacketTypePoll {
		t.Errorf("Expected type %s, got %s", PacketTypePoll, string(response[:4]))
	}
}

func TestCreateStatusResponse(t *testing.T) {
	name := "Test Reflector"
	description := "Test Desc"
	count := 42

	response := CreateStatusResponse(name, description, count)

	if len(response) != StatusPacketSize {
		t.Errorf("Expected response size %d, got %d", StatusPacketSize, len(response))
	}

	if string(response[:4]) != PacketTypeStatus {
		t.Errorf("Expected type %s, got %s", PacketTypeStatus, string(response[:4]))
	}

	// Check count is encoded correctly
	countStr := string(response[39:42])
	if countStr != "042" {
		t.Errorf("Expected count '042', got '%s'", countStr)
	}
}

func TestPacketMethods(t *testing.T) {
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 42000}

	// Test data packet
	dataPacket := make([]byte, DataPacketSize)
	copy(dataPacket[:4], PacketTypeData)
	copy(dataPacket[4:14], "W1ABC     ")

	packet, err := ParsePacket(dataPacket, addr)
	if err != nil {
		t.Fatalf("Failed to parse data packet: %v", err)
	}

	if !packet.IsDataPacket() {
		t.Errorf("Expected IsDataPacket() to be true")
	}

	if packet.IsPollPacket() {
		t.Errorf("Expected IsPollPacket() to be false")
	}

	// Test poll packet
	pollData := []byte("YSFPW1ABC     ")
	pollPacket, err := ParsePacket(pollData, addr)
	if err != nil {
		t.Fatalf("Failed to parse poll packet: %v", err)
	}

	if !pollPacket.IsPollPacket() {
		t.Errorf("Expected IsPollPacket() to be true")
	}

	if pollPacket.IsDataPacket() {
		t.Errorf("Expected IsDataPacket() to be false")
	}
}

func TestPacketString(t *testing.T) {
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 42000}
	data := []byte("YSFPW1ABC     ")

	packet, err := ParsePacket(data, addr)
	if err != nil {
		t.Fatalf("Failed to parse packet: %v", err)
	}

	str := packet.String()
	if str == "" {
		t.Errorf("Expected non-empty string representation")
	}

	// Should contain key information
	if !contains(str, "YSFP") {
		t.Errorf("String representation should contain packet type")
	}
	if !contains(str, "W1ABC") {
		t.Errorf("String representation should contain callsign")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		   (s == substr ||
		    (len(s) > len(substr) &&
		     (s[:len(substr)] == substr ||
		      s[len(s)-len(substr):] == substr ||
		      containsSubstr(s, substr))))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}