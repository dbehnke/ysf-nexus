package dmr

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
)

// Packet types for DMR network protocol
const (
	// Authentication packets
	PacketTypeRPTL   = "RPTL"   // Login request
	PacketTypeRPTK   = "RPTK"   // Password/key response
	PacketTypeRPTC   = "RPTC"   // Configuration
	PacketTypeRPTA   = "RPTA"   // ACK from server (legacy)
	PacketTypeRPTACK = "RPTACK" // ACK from server (modern/TGIF)
	PacketTypeMSTAK  = "MSTAK"  // Server ACK (after RPTL)
	PacketTypeMSTNAK = "MSTNAK" // Server NAK (authentication reject)
	PacketTypeMSTP   = "MSTP"   // Ping from server (MSTPING)
	PacketTypeRPTP   = "RPTP"   // Pong response to server (RPTPONG)
	PacketTypeMSTC   = "MSTC"   // Server closing
	PacketTypeMSTN   = "MSTN"   // Alias for MSTNAK

	// Voice/Data packets
	PacketTypeDMRD = "DMRD" // DMR data (voice/data)
)

// Frame types for DMRD packets
const (
	FrameTypeVoiceHeader     = 0x01
	FrameTypeVoiceSync       = 0x02
	FrameTypeVoiceData       = 0x03
	FrameTypeVoiceTerminator = 0x04
	FrameTypeDataHeader      = 0x05
	FrameTypeDataSync        = 0x06
	FrameTypeData            = 0x07
)

// Call types
const (
	CallTypeGroup   = 0x00
	CallTypePrivate = 0x03
)

// Packet represents a generic DMR network packet
type Packet struct {
	Type     string
	Data     []byte
	Sequence uint32
}

// RPTLPacket represents a repeater login packet
type RPTLPacket struct {
	RepeaterID uint32
}

// NewRPTLPacket creates a new login packet
func NewRPTLPacket(repeaterID uint32) *RPTLPacket {
	return &RPTLPacket{
		RepeaterID: repeaterID,
	}
}

// Serialize converts the login packet to bytes
func (p *RPTLPacket) Serialize() []byte {
	packet := make([]byte, 8)
	copy(packet[0:4], PacketTypeRPTL)
	binary.BigEndian.PutUint32(packet[4:8], p.RepeaterID)
	return packet
}

// RPTKPacket represents a repeater key/password packet
type RPTKPacket struct {
	RepeaterID uint32
	Hash       [32]byte // Full SHA256 hash (server validates first 4 bytes)
}

// NewRPTKPacket creates a new password packet with hashed credentials
// Hash format: Full SHA256(salt + password) - 32 bytes
// MMDVMHost reference: DMRDirectNetwork.cpp writeAuthorisation()
// Server validates first 4 bytes as uint32 but expects full 32-byte hash in packet
func NewRPTKPacket(repeaterID uint32, password string, salt []byte) *RPTKPacket {
	// Create hash: SHA256(salt + password)
	hasher := sha256.New()
	hasher.Write(salt)
	hasher.Write([]byte(password))
	fullHash := hasher.Sum(nil)

	var hash [32]byte
	copy(hash[:], fullHash)

	return &RPTKPacket{
		RepeaterID: repeaterID,
		Hash:       hash,
	}
}

// NewRPTKPacketBytes creates a new password packet with hashed credentials from password bytes
// This version accepts pre-processed password bytes (e.g., hex-decoded)
func NewRPTKPacketBytes(repeaterID uint32, passwordBytes []byte, salt []byte) *RPTKPacket {
	// Create hash: SHA256(salt + passwordBytes)
	hasher := sha256.New()
	hasher.Write(salt)
	hasher.Write(passwordBytes)
	fullHash := hasher.Sum(nil)

	var hash [32]byte
	copy(hash[:], fullHash)

	return &RPTKPacket{
		RepeaterID: repeaterID,
		Hash:       hash,
	}
}

// Serialize converts the key packet to bytes
// Format: RPTK (4 bytes) + RepeaterID (4 bytes) + Hash (32 bytes) = 40 bytes total
// Matches MMDVMHost DMRDirectNetwork.cpp writeAuthorisation()
func (p *RPTKPacket) Serialize() []byte {
	packet := make([]byte, 40)
	copy(packet[0:4], PacketTypeRPTK)
	binary.BigEndian.PutUint32(packet[4:8], p.RepeaterID)
	copy(packet[8:40], p.Hash[:])
	return packet
}

// RPTCPacket represents a repeater configuration packet
type RPTCPacket struct {
	RepeaterID  uint32
	Callsign    string // Up to 8 characters
	RXFreq      uint32 // In Hz
	TXFreq      uint32 // In Hz
	TXPower     uint8  // In dBm (00-99) - NOT watts!
	ColorCode   uint8  // 01-15
	Latitude    float32
	Longitude   float32
	Height      int32  // In meters (000-999)
	Location    string // Up to 20 characters
	Description string // Up to 20 characters
	URL         string // Up to 124 characters
	SoftwareID  string // Up to 40 characters
	PackageID   string // Up to 40 characters
}

// NewRPTCPacket creates a new configuration packet
func NewRPTCPacket(repeaterID uint32) *RPTCPacket {
	return &RPTCPacket{
		RepeaterID: repeaterID,
		Callsign:   "",
		ColorCode:  1,
		TXPower:    25, // 25 dBm = ~316 watts (typical repeater power)
	}
}

// Serialize converts the configuration packet to bytes
// Format per DMRHub implementation: internal/db/models/repeater_configuration.go
func (p *RPTCPacket) Serialize() []byte {
	packet := make([]byte, 302)

	// Signature (4 bytes ASCII)
	copy(packet[0:4], PacketTypeRPTC)

	// Repeater ID (4 bytes binary) - offset 4
	binary.BigEndian.PutUint32(packet[4:8], p.RepeaterID)

	// Callsign (8 bytes ASCII, left-aligned) - offset 8
	copy(packet[8:16], fmt.Sprintf("%-8.8s", p.Callsign))

	// RX Frequency (9 ASCII digits, zero-padded)
	copy(packet[16:25], fmt.Sprintf("%09d", p.RXFreq))

	// TX Frequency (9 ASCII digits, zero-padded)
	copy(packet[25:34], fmt.Sprintf("%09d", p.TXFreq))

	// TX Power (2 ASCII digits, zero-padded)
	copy(packet[34:36], fmt.Sprintf("%02d", p.TXPower))

	// Color Code (2 ASCII digits, zero-padded)
	copy(packet[36:38], fmt.Sprintf("%02d", p.ColorCode))

	// Latitude (8 chars ASCII) - matches MMDVMHost format: sprintf("%08f") then truncate to 8 chars
	// Example: 85.000000 → 85.00000 (8 chars)
	latStr := fmt.Sprintf("%08f", p.Latitude)
	if len(latStr) > 8 {
		latStr = latStr[:8]
	}
	copy(packet[38:46], fmt.Sprintf("%8.8s", latStr))

	// Longitude (9 chars ASCII) - matches MMDVMHost format: sprintf("%09f") then truncate to 9 chars
	// Example: -83.000000 → -83.00000 (9 chars)
	lonStr := fmt.Sprintf("%09f", p.Longitude)
	if len(lonStr) > 9 {
		lonStr = lonStr[:9]
	}
	copy(packet[46:55], fmt.Sprintf("%9.9s", lonStr))

	// Height (3 ASCII digits, zero-padded)
	copy(packet[55:58], fmt.Sprintf("%03d", p.Height))

	// Location (20 chars, left-aligned) - offset 58
	copy(packet[58:78], fmt.Sprintf("%-20.20s", p.Location))

	// Description (19 chars, left-aligned) - offset 78
	copy(packet[78:97], fmt.Sprintf("%-19.19s", p.Description))

	// Slots (1 byte ASCII) - offset 97
	// Per MMDVMHost: '4' for simplex hotspot, '3' for duplex both slots, '1' slot 1 only, '2' slot 2 only
	// Default to '4' (simplex hotspot) for maximum compatibility
	packet[97] = '4'

	// URL (124 chars, left-aligned) - offset 98
	copy(packet[98:222], fmt.Sprintf("%-124.124s", p.URL))

	// Software ID (40 chars, left-aligned)
	copy(packet[222:262], fmt.Sprintf("%-40.40s", p.SoftwareID))

	// Package ID (40 chars, left-aligned)
	copy(packet[262:302], fmt.Sprintf("%-40.40s", p.PackageID))

	return packet
}

// MSTPPacket represents a ping response packet (RPTPONG)
type MSTPPacket struct {
	RepeaterID uint32
}

// NewMSTPPacket creates a new ping response packet (RPTPONG)
func NewMSTPPacket(repeaterID uint32) *MSTPPacket {
	return &MSTPPacket{
		RepeaterID: repeaterID,
	}
}

// Serialize converts the ping response to bytes (RPTPONG format)
func (p *MSTPPacket) Serialize() []byte {
	// RPTPONG packet: "RPTPONG" (7 bytes) + RepeaterID (4 bytes) = 11 bytes
	packet := make([]byte, 11)
	copy(packet[0:7], "RPTPONG")
	binary.BigEndian.PutUint32(packet[7:11], p.RepeaterID)
	return packet
}

// DMRDPacket represents a DMR data packet (voice or data)
type DMRDPacket struct {
	Sequence   uint8
	SrcID      uint32
	DstID      uint32
	RepeaterID uint32
	Slot       uint8 // 1 or 2
	CallType   uint8 // Group or Private
	FrameType  uint8 // Voice header, sync, data, terminator
	StreamID   uint32
	Data       []byte // 33 bytes of voice/data
	BER        uint8  // Bit Error Rate
	RSSI       uint8  // Signal strength
}

// NewDMRDPacket creates a new DMR data packet
func NewDMRDPacket() *DMRDPacket {
	return &DMRDPacket{
		Data: make([]byte, 33),
	}
}

// Serialize converts the DMR data packet to bytes
func (p *DMRDPacket) Serialize() []byte {
	packet := make([]byte, 55)

	// Packet type
	copy(packet[0:4], PacketTypeDMRD)

	// Sequence number
	packet[4] = p.Sequence

	// Source ID (3 bytes)
	packet[5] = byte(p.SrcID >> 16)
	packet[6] = byte(p.SrcID >> 8)
	packet[7] = byte(p.SrcID)

	// Destination ID (3 bytes)
	packet[8] = byte(p.DstID >> 16)
	packet[9] = byte(p.DstID >> 8)
	packet[10] = byte(p.DstID)

	// Repeater ID (4 bytes)
	binary.BigEndian.PutUint32(packet[11:15], p.RepeaterID)

	// Slot number (bit 7 = slot, bits 0-6 = call type)
	slotByte := p.CallType & 0x7F
	if p.Slot == 2 {
		slotByte |= 0x80
	}
	packet[15] = slotByte

	// Frame type
	packet[16] = p.FrameType

	// Stream ID
	binary.BigEndian.PutUint32(packet[17:21], p.StreamID)

	// Voice/Data payload (33 bytes)
	copy(packet[21:54], p.Data)

	// BER and RSSI
	packet[54] = (p.BER << 4) | (p.RSSI & 0x0F)

	return packet
}

// ParsePacket parses a raw packet into a typed packet
func ParsePacket(data []byte) (*Packet, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("packet too short: %d bytes", len(data))
	}

	packetType := string(data[0:4])

	return &Packet{
		Type: packetType,
		Data: data,
	}, nil
}

// ParseRPTAPacket parses an ACK packet from server
// Multiple formats exist:
// 1. RPTACK + salt (4 bytes) = 10 bytes (TGIF/modern: challenge with salt, no repeater ID echo)
// 2. RPTA + repeater_id (4 bytes) + salt (N bytes) = 8+N bytes (legacy with salt)
// 3. RPTA + repeater_id (4 bytes) = 8 bytes (legacy ACK only)
func ParseRPTAPacket(data []byte) (uint32, []byte, error) {
	if len(data) < 8 {
		return 0, nil, fmt.Errorf("RPTA packet too short: %d bytes", len(data))
	}

	// Check if this is RPTACK format (TGIF/modern)
	if len(data) >= 6 && string(data[0:6]) == "RPTACK" {
		// RPTACK format: "RPTACK" (6 bytes) + salt (4 bytes) = 10 bytes
		// TGIF doesn't echo back the repeater ID, salt starts at byte 6
		var salt []byte
		if len(data) > 6 {
			salt = data[6:]
		}

		return 0, salt, nil
	}

	// Legacy RPTA format: "RPTA" (4 bytes) + repeater_id (4 bytes) + optional salt
	repeaterID := binary.BigEndian.Uint32(data[4:8])

	var salt []byte
	if len(data) > 8 {
		salt = data[8:]
	}

	return repeaterID, salt, nil
}

// ParseDMRDPacket parses a DMRD packet
func ParseDMRDPacket(data []byte) (*DMRDPacket, error) {
	if len(data) < 55 {
		return nil, fmt.Errorf("DMRD packet too short: %d bytes", len(data))
	}

	packet := NewDMRDPacket()

	// Sequence
	packet.Sequence = data[4]

	// Source ID (3 bytes)
	packet.SrcID = uint32(data[5])<<16 | uint32(data[6])<<8 | uint32(data[7])

	// Destination ID (3 bytes)
	packet.DstID = uint32(data[8])<<16 | uint32(data[9])<<8 | uint32(data[10])

	// Repeater ID (4 bytes)
	packet.RepeaterID = binary.BigEndian.Uint32(data[11:15])

	// Slot and call type
	slotByte := data[15]
	packet.Slot = 1
	if slotByte&0x80 != 0 {
		packet.Slot = 2
	}
	packet.CallType = slotByte & 0x7F

	// Frame type
	packet.FrameType = data[16]

	// Stream ID
	packet.StreamID = binary.BigEndian.Uint32(data[17:21])

	// Voice/Data payload
	copy(packet.Data, data[21:54])

	// BER and RSSI
	packet.BER = (data[54] >> 4) & 0x0F
	packet.RSSI = data[54] & 0x0F

	return packet, nil
}

// Helper function to convert float32 to uint32 for serialization
func floatToUint32(f float32) uint32 {
	return math.Float32bits(f)
}

// Helper function to convert uint32 to float32 for deserialization
func uint32ToFloat(u uint32) float32 {
	return math.Float32frombits(u)
}
