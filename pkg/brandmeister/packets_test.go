package brandmeister

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestYSFLPacket(t *testing.T) {
	// YSFL is Login.
	// BrandMeister YSF Direct handshake starts with YSFL.
	// We expect a packet header "YSFL" followed by the callsign (10 chars space padded).

	callsign := "W1ABC     " // 10 chars
	expected := append([]byte("YSFL"), []byte(callsign)...)

	packet := &YSFLPacket{
		Callsign: callsign,
	}

	data, err := packet.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal YSFL: %v", err)
	}

	if !bytes.Equal(data, expected) {
		t.Errorf("Expected %x, got %x", expected, data)
	}
}

func TestYSFKPacket(t *testing.T) {
	// YSFK is typically the challenge (MSTACK in some docs, but for YSF Direct we look for 4 bytes challenge)
	// Actually, BrandMeister sends a challenge.

	challenge := uint32(0x12345678)
	// We'll define how we expect to unmarshal it.
	data := append([]byte("YSFK"), 0x78, 0x56, 0x34, 0x12) // Little endian assumed for now, will verify with refs

	packet := &YSFKPacket{}
	err := packet.Unmarshal(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal YSFK: %v", err)
	}

	if packet.Challenge != challenge {
		t.Errorf("Expected challenge %v, got %v", challenge, packet.Challenge)
	}
}

func TestYSFOPacket(t *testing.T) {
	options := "group=3100"
	expected := append([]byte("YSFO"), []byte(options)...)

	packet := &YSFOPacket{
		Options: options,
	}

	data, err := packet.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal YSFO: %v", err)
	}

	if !bytes.Equal(data, expected) {
		t.Errorf("Expected %x, got %x", expected, data)
	}
}

func TestYSFACKPacket(t *testing.T) {
	expected := []byte("YSFACK")

	packet := &YSFACKPacket{}
	data, err := packet.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal YSFACK: %v", err)
	}

	if !bytes.Equal(data, expected) {
		t.Errorf("Expected %x, got %x", expected, data)
	}
}

func TestRPTKPacket(t *testing.T) {
	repeaterID := uint32(123456)
	hash := [32]byte{0x01, 0x02} // truncated for test

	expected := make([]byte, 40)
	copy(expected[0:4], "RPTK")
	binary.LittleEndian.PutUint32(expected[4:8], repeaterID)
	copy(expected[8:40], hash[:])

	packet := &RPTKPacket{
		RepeaterID: repeaterID,
		Hash:       hash,
	}

	data, err := packet.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal RPTK: %v", err)
	}

	if !bytes.Equal(data, expected) {
		t.Errorf("Expected %x, got %x", expected, data)
	}
}
