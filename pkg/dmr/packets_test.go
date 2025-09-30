package dmr

import (
	"bytes"
	"testing"
)

func TestRPTLPacket(t *testing.T) {
	repeaterID := uint32(1234567)

	packet := NewRPTLPacket(repeaterID)
	data := packet.Serialize()

	// Check packet type
	if string(data[0:4]) != PacketTypeRPTL {
		t.Errorf("Expected packet type %s, got %s", PacketTypeRPTL, string(data[0:4]))
	}

	// Check length
	if len(data) != 8 {
		t.Errorf("Expected packet length 8, got %d", len(data))
	}

	// Parse and verify
	parsed, err := ParsePacket(data)
	if err != nil {
		t.Fatalf("Failed to parse RPTL packet: %v", err)
	}

	if parsed.Type != PacketTypeRPTL {
		t.Errorf("Parsed packet type mismatch: expected %s, got %s", PacketTypeRPTL, parsed.Type)
	}
}

func TestRPTKPacket(t *testing.T) {
	repeaterID := uint32(1234567)
	password := "testpassword"
	salt := []byte("somesalt")

	packet := NewRPTKPacket(repeaterID, password, salt)
	data := packet.Serialize()

	// Check packet type
	if string(data[0:4]) != PacketTypeRPTK {
		t.Errorf("Expected packet type %s, got %s", PacketTypeRPTK, string(data[0:4]))
	}

	// Check length
	if len(data) != 40 {
		t.Errorf("Expected packet length 40, got %d", len(data))
	}

	// Verify hash is present (non-zero)
	hashZero := true
	for i := 8; i < 40; i++ {
		if data[i] != 0 {
			hashZero = false
			break
		}
	}
	if hashZero {
		t.Error("Hash should not be all zeros")
	}
}

func TestRPTCPacket(t *testing.T) {
	repeaterID := uint32(1234567)

	packet := NewRPTCPacket(repeaterID)
	packet.Callsign = "W1ABC"
	packet.RXFreq = 446000000
	packet.TXFreq = 446000000
	packet.TXPower = 10
	packet.ColorCode = 1
	packet.Latitude = 40.7128
	packet.Longitude = -74.0060
	packet.Height = 100
	packet.Location = "New York, NY"
	packet.Description = "Test Station"
	packet.URL = "https://example.com"
	packet.SoftwareID = "YSF-Nexus"
	packet.PackageID = "YSF-Nexus-DMR"

	data := packet.Serialize()

	// Check packet type
	if string(data[0:4]) != PacketTypeRPTC {
		t.Errorf("Expected packet type %s, got %s", PacketTypeRPTC, string(data[0:4]))
	}

	// Check length
	if len(data) != 302 {
		t.Errorf("Expected packet length 302, got %d", len(data))
	}

	// Verify callsign (bytes 8-16)
	callsign := string(bytes.TrimSpace(data[8:16]))
	if callsign != "W1ABC" {
		t.Errorf("Expected callsign W1ABC, got %s", callsign)
	}
}

func TestMSTPPacket(t *testing.T) {
	repeaterID := uint32(1234567)

	packet := NewMSTPPacket(repeaterID)
	data := packet.Serialize()

	// Check packet type
	if string(data[0:4]) != PacketTypeMSTP {
		t.Errorf("Expected packet type %s, got %s", PacketTypeMSTP, string(data[0:4]))
	}

	// Check length
	if len(data) != 11 {
		t.Errorf("Expected packet length 11, got %d", len(data))
	}
}

func TestDMRDPacket(t *testing.T) {
	packet := NewDMRDPacket()
	packet.Sequence = 5
	packet.SrcID = 1234567
	packet.DstID = 91
	packet.RepeaterID = 1234567
	packet.Slot = 2
	packet.CallType = CallTypeGroup
	packet.FrameType = FrameTypeVoiceData
	packet.StreamID = 12345
	packet.BER = 3
	packet.RSSI = 10

	// Fill data with test pattern
	for i := range packet.Data {
		packet.Data[i] = byte(i)
	}

	data := packet.Serialize()

	// Check packet type
	if string(data[0:4]) != PacketTypeDMRD {
		t.Errorf("Expected packet type %s, got %s", PacketTypeDMRD, string(data[0:4]))
	}

	// Check length
	if len(data) != 55 {
		t.Errorf("Expected packet length 55, got %d", len(data))
	}

	// Parse back
	parsed, err := ParseDMRDPacket(data)
	if err != nil {
		t.Fatalf("Failed to parse DMRD packet: %v", err)
	}

	// Verify fields
	if parsed.Sequence != packet.Sequence {
		t.Errorf("Sequence mismatch: expected %d, got %d", packet.Sequence, parsed.Sequence)
	}

	if parsed.SrcID != packet.SrcID {
		t.Errorf("SrcID mismatch: expected %d, got %d", packet.SrcID, parsed.SrcID)
	}

	if parsed.DstID != packet.DstID {
		t.Errorf("DstID mismatch: expected %d, got %d", packet.DstID, parsed.DstID)
	}

	if parsed.RepeaterID != packet.RepeaterID {
		t.Errorf("RepeaterID mismatch: expected %d, got %d", packet.RepeaterID, parsed.RepeaterID)
	}

	if parsed.Slot != packet.Slot {
		t.Errorf("Slot mismatch: expected %d, got %d", packet.Slot, parsed.Slot)
	}

	if parsed.CallType != packet.CallType {
		t.Errorf("CallType mismatch: expected %d, got %d", packet.CallType, parsed.CallType)
	}

	if parsed.FrameType != packet.FrameType {
		t.Errorf("FrameType mismatch: expected %d, got %d", packet.FrameType, parsed.FrameType)
	}

	if parsed.StreamID != packet.StreamID {
		t.Errorf("StreamID mismatch: expected %d, got %d", packet.StreamID, parsed.StreamID)
	}

	if !bytes.Equal(parsed.Data, packet.Data) {
		t.Error("Data mismatch")
	}

	if parsed.BER != packet.BER {
		t.Errorf("BER mismatch: expected %d, got %d", packet.BER, parsed.BER)
	}

	if parsed.RSSI != packet.RSSI {
		t.Errorf("RSSI mismatch: expected %d, got %d", packet.RSSI, parsed.RSSI)
	}
}

func TestParseRPTAPacket(t *testing.T) {
	// Create a sample RPTA packet with salt
	data := make([]byte, 12)
	copy(data[0:4], PacketTypeRPTA)
	data[4] = 0x00
	data[5] = 0x12
	data[6] = 0xD6
	data[7] = 0x87 // RepeaterID = 1234567
	copy(data[8:12], []byte("salt"))

	repeaterID, salt, err := ParseRPTAPacket(data)
	if err != nil {
		t.Fatalf("Failed to parse RPTA packet: %v", err)
	}

	if repeaterID != 1234567 {
		t.Errorf("Expected repeater ID 1234567, got %d", repeaterID)
	}

	if string(salt) != "salt" {
		t.Errorf("Expected salt 'salt', got '%s'", string(salt))
	}
}

func TestParsePacketTooShort(t *testing.T) {
	data := []byte{0x01, 0x02}

	_, err := ParsePacket(data)
	if err == nil {
		t.Error("Expected error for packet too short, got nil")
	}
}

func TestParseDMRDPacketTooShort(t *testing.T) {
	data := make([]byte, 30) // Too short
	copy(data[0:4], PacketTypeDMRD)

	_, err := ParseDMRDPacket(data)
	if err == nil {
		t.Error("Expected error for DMRD packet too short, got nil")
	}
}

func TestSlotEncoding(t *testing.T) {
	tests := []struct {
		slot     uint8
		callType uint8
		expected uint8
	}{
		{1, CallTypeGroup, 0x00},
		{2, CallTypeGroup, 0x80},
		{1, CallTypePrivate, 0x03},
		{2, CallTypePrivate, 0x83},
	}

	for _, test := range tests {
		packet := NewDMRDPacket()
		packet.Slot = test.slot
		packet.CallType = test.callType

		data := packet.Serialize()
		slotByte := data[15]

		if slotByte != test.expected {
			t.Errorf("Slot encoding failed: slot=%d, callType=%d, expected=0x%02X, got=0x%02X",
				test.slot, test.callType, test.expected, slotByte)
		}

		// Parse back and verify
		parsed, err := ParseDMRDPacket(data)
		if err != nil {
			t.Fatalf("Failed to parse: %v", err)
		}

		if parsed.Slot != test.slot {
			t.Errorf("Slot decode failed: expected %d, got %d", test.slot, parsed.Slot)
		}

		if parsed.CallType != test.callType {
			t.Errorf("CallType decode failed: expected %d, got %d", test.callType, parsed.CallType)
		}
	}
}

func TestFloatConversion(t *testing.T) {
	tests := []float32{
		0.0,
		1.0,
		-1.0,
		40.7128,  // NYC latitude
		-74.0060, // NYC longitude
		123.456,
		-999.999,
	}

	for _, f := range tests {
		u := floatToUint32(f)
		result := uint32ToFloat(u)

		if result != f {
			t.Errorf("Float conversion failed: input=%f, uint=%d, result=%f", f, u, result)
		}
	}
}

func BenchmarkDMRDPacketSerialize(b *testing.B) {
	packet := NewDMRDPacket()
	packet.Sequence = 5
	packet.SrcID = 1234567
	packet.DstID = 91
	packet.RepeaterID = 1234567
	packet.Slot = 2
	packet.CallType = CallTypeGroup
	packet.FrameType = FrameTypeVoiceData
	packet.StreamID = 12345

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = packet.Serialize()
	}
}

func BenchmarkDMRDPacketParse(b *testing.B) {
	packet := NewDMRDPacket()
	packet.Sequence = 5
	packet.SrcID = 1234567
	packet.DstID = 91
	packet.RepeaterID = 1234567
	packet.Slot = 2
	packet.CallType = CallTypeGroup
	packet.FrameType = FrameTypeVoiceData
	packet.StreamID = 12345

	data := packet.Serialize()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseDMRDPacket(data)
	}
}
