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
	PacketTypeRPTL = "RPTL" // Login request
	PacketTypeRPTK = "RPTK" // Password/key response
	PacketTypeRPTC = "RPTC" // Configuration
	PacketTypeRPTA = "RPTA" // ACK from server
	PacketTypeMSTN = "MSTN" // Server NAK
	PacketTypeMSTP = "MSTP" // Ping from server
	PacketTypeMSTC = "MSTC" // Server closing

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
	Hash       [32]byte // SHA256 hash
}

// NewRPTKPacket creates a new password packet with hashed credentials
func NewRPTKPacket(repeaterID uint32, password string, salt []byte) *RPTKPacket {
	// Create hash: SHA256(password + salt)
	hasher := sha256.New()
	hasher.Write([]byte(password))
	hasher.Write(salt)

	var hash [32]byte
	copy(hash[:], hasher.Sum(nil))

	return &RPTKPacket{
		RepeaterID: repeaterID,
		Hash:       hash,
	}
}

// Serialize converts the key packet to bytes
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
	TXPower     uint32 // In watts
	ColorCode   uint8
	Latitude    float32
	Longitude   float32
	Height      int32
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
		TXPower:    1,
	}
}

// Serialize converts the configuration packet to bytes
func (p *RPTCPacket) Serialize() []byte {
	packet := make([]byte, 302)

	// Packet type
	copy(packet[0:4], PacketTypeRPTC)

	// Repeater ID
	binary.BigEndian.PutUint32(packet[4:8], p.RepeaterID)

	// Callsign (8 bytes, space-padded)
	callsign := fmt.Sprintf("%-8s", p.Callsign)
	if len(callsign) > 8 {
		callsign = callsign[:8]
	}
	copy(packet[8:16], callsign)

	// RX Frequency
	binary.BigEndian.PutUint32(packet[16:20], p.RXFreq)

	// TX Frequency
	binary.BigEndian.PutUint32(packet[20:24], p.TXFreq)

	// TX Power
	binary.BigEndian.PutUint32(packet[24:28], p.TXPower)

	// Color Code
	packet[28] = p.ColorCode

	// Latitude (float32)
	binary.BigEndian.PutUint32(packet[29:33], floatToUint32(p.Latitude))

	// Longitude (float32)
	binary.BigEndian.PutUint32(packet[33:37], floatToUint32(p.Longitude))

	// Height
	binary.BigEndian.PutUint32(packet[37:41], uint32(p.Height))

	// Location (20 bytes)
	location := fmt.Sprintf("%-20s", p.Location)
	if len(location) > 20 {
		location = location[:20]
	}
	copy(packet[41:61], location)

	// Description (20 bytes)
	description := fmt.Sprintf("%-20s", p.Description)
	if len(description) > 20 {
		description = description[:20]
	}
	copy(packet[61:81], description)

	// Slots (4 bytes - we support both slots by default)
	packet[81] = 0x03 // Both slots enabled

	// URL (124 bytes)
	url := fmt.Sprintf("%-124s", p.URL)
	if len(url) > 124 {
		url = url[:124]
	}
	copy(packet[82:206], url)

	// Software ID (40 bytes)
	softwareID := fmt.Sprintf("%-40s", p.SoftwareID)
	if len(softwareID) > 40 {
		softwareID = softwareID[:40]
	}
	copy(packet[206:246], softwareID)

	// Package ID (40 bytes)
	packageID := fmt.Sprintf("%-40s", p.PackageID)
	if len(packageID) > 40 {
		packageID = packageID[:40]
	}
	copy(packet[246:286], packageID)

	return packet
}

// MSTPPacket represents a ping response packet
type MSTPPacket struct {
	RepeaterID uint32
}

// NewMSTPPacket creates a new ping response packet
func NewMSTPPacket(repeaterID uint32) *MSTPPacket {
	return &MSTPPacket{
		RepeaterID: repeaterID,
	}
}

// Serialize converts the ping response to bytes
func (p *MSTPPacket) Serialize() []byte {
	packet := make([]byte, 11)
	copy(packet[0:4], PacketTypeMSTP)
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
func ParseRPTAPacket(data []byte) (uint32, []byte, error) {
	if len(data) < 8 {
		return 0, nil, fmt.Errorf("RPTA packet too short")
	}

	repeaterID := binary.BigEndian.Uint32(data[4:8])

	// Salt is everything after byte 8
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
