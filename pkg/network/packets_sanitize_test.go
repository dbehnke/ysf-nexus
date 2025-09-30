package network

import (
	"testing"
)

func TestSanitizeCallsign(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "dash suffix",
			input:    "KF8S-DAVE",
			expected: "KF8S",
		},
		{
			name:     "slash suffix",
			input:    "N8ZA/CHUCK",
			expected: "N8ZA",
		},
		{
			name:     "space suffix",
			input:    "M0FXB AND",
			expected: "M0FXB",
		},
		{
			name:     "no suffix",
			input:    "W1ABC",
			expected: "W1ABC",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "with leading/trailing spaces",
			input:    "  KB1TEST  ",
			expected: "KB1TEST",
		},
		{
			name:     "multiple delimiters",
			input:    "W2XYZ-RPT/B",
			expected: "W2XYZ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeCallsign(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeCallsign(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeDataPacket(t *testing.T) {
	// Create a test YSFD packet
	packet := make([]byte, DataPacketSize)
	copy(packet[0:4], "YSFD")

	// Set gateway callsign (bytes 4-14): "KF8S-DAVE "
	copy(packet[4:14], "KF8S-DAVE ")

	// Set source callsign (bytes 14-24): "N8ZA/CHUCK"
	copy(packet[14:24], "N8ZA/CHUCK")

	// Set destination callsign (bytes 24-34): "M0FXB AND "
	copy(packet[24:34], "M0FXB AND ")

	// Sanitize the packet
	sanitized := SanitizeDataPacket(packet)

	// Verify gateway callsign was cleaned (should be "KF8S      ")
	gatewayCS := string(sanitized[4:14])
	if gatewayCS[:4] != "KF8S" {
		t.Errorf("Gateway callsign not sanitized correctly: got %q, want 'KF8S      '", gatewayCS)
	}

	// Verify source callsign was cleaned (should be "N8ZA      ")
	sourceCS := string(sanitized[14:24])
	if sourceCS[:4] != "N8ZA" {
		t.Errorf("Source callsign not sanitized correctly: got %q, want 'N8ZA      '", sourceCS)
	}

	// Verify destination callsign was cleaned (should be "M0FXB     ")
	destCS := string(sanitized[24:34])
	if destCS[:5] != "M0FXB" {
		t.Errorf("Destination callsign not sanitized correctly: got %q, want 'M0FXB     '", destCS)
	}

	// Verify original packet was not modified
	if string(packet[4:14]) != "KF8S-DAVE " {
		t.Error("Original packet was modified")
	}
}

func TestSanitizeDataPacket_NonDataPacket(t *testing.T) {
	// Create a YSFP packet (not data)
	packet := make([]byte, PollPacketSize)
	copy(packet[0:4], "YSFP")
	copy(packet[4:14], "TEST-CS   ")

	// Sanitize should return unchanged for non-data packets
	sanitized := SanitizeDataPacket(packet)

	if string(sanitized[4:14]) != "TEST-CS   " {
		t.Error("Non-data packet was modified when it shouldn't be")
	}
}

func TestSanitizeDataPacket_TooSmall(t *testing.T) {
	// Create a packet that's too small
	packet := []byte{0x59, 0x53, 0x46}

	// Should return unchanged
	sanitized := SanitizeDataPacket(packet)

	if len(sanitized) != len(packet) {
		t.Error("Packet size changed for too-small packet")
	}
}
