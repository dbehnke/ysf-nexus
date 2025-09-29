package network

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

// YSF packet types
const (
	PacketTypePoll   = "YSFP"
	PacketTypeData   = "YSFD"
	PacketTypeUnlink = "YSFU"
	PacketTypeStatus = "YSFS"
	PacketTypeOption = "YSFO"
	PacketTypeInfo   = "YSFI"
)

// Packet sizes
const (
	DataPacketSize   = 155
	PollPacketSize   = 14
	StatusPacketSize = 42
)

// Packet represents a YSF network packet
type Packet struct {
	Type      string
	Data      []byte
	Source    *net.UDPAddr
	Timestamp time.Time
	Callsign  string
}

// YSFHeader represents the common YSF packet header
type YSFHeader struct {
	Type     [4]byte  // Packet type (YSFP, YSFD, etc.)
	Callsign [10]byte // Callsign
}

// DataPacket represents a YSF data packet (YSFD)
type DataPacket struct {
	Header YSFHeader
	Data   [145]byte // Remaining data
}

// PollPacket represents a YSF poll packet (YSFP)
type PollPacket struct {
	Header    YSFHeader
	Reflector [10]byte // "REFLECTOR" string
}

// StatusRequest represents a status request packet
type StatusRequest struct {
	Type [4]byte // "YSFS"
	Data [10]byte
}

// StatusResponse represents a status response packet
type StatusResponse struct {
	Type        [4]byte  // "YSFS"
	Hash        [5]byte  // 5-digit hash
	Name        [16]byte // Reflector name
	Description [14]byte // Reflector description
	Count       [3]byte  // Connection count
}

// ParsePacket parses a raw UDP packet into a structured Packet
func ParsePacket(data []byte, addr *net.UDPAddr) (*Packet, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("packet too small: %d bytes", len(data))
	}

	packet := &Packet{
		Type:      string(data[:4]),
		Data:      data,
		Source:    addr,
		Timestamp: time.Now(),
	}

	// Extract callsign if present
	if len(data) >= 14 {
		callsign := strings.TrimSpace(string(data[4:14]))
		packet.Callsign = strings.TrimRight(callsign, "\x00")
	}

	// Validate packet type and size
	switch packet.Type {
	case PacketTypePoll:
		if len(data) != PollPacketSize {
			return nil, fmt.Errorf("invalid poll packet size: %d", len(data))
		}
	case PacketTypeData:
		if len(data) != DataPacketSize {
			return nil, fmt.Errorf("invalid data packet size: %d", len(data))
		}
	case PacketTypeUnlink:
		if len(data) != PollPacketSize {
			return nil, fmt.Errorf("invalid unlink packet size: %d", len(data))
		}
	case PacketTypeStatus:
		// Status packets can vary in size. Accept minimal 4-byte 'YSFS' requests
		if len(data) < 4 {
			return nil, fmt.Errorf("invalid status packet size: %d", len(data))
		}
	case PacketTypeOption, PacketTypeInfo:
		// These are typically discarded
		return nil, fmt.Errorf("unsupported packet type: %s", packet.Type)
	default:
		return nil, fmt.Errorf("unknown packet type: %s", packet.Type)
	}

	return packet, nil
}

// CreatePollResponse creates a poll response packet
func CreatePollResponse() []byte {
	packet := make([]byte, PollPacketSize)
	copy(packet[0:4], PacketTypePoll)
	// Some implementations might be sensitive to the exact format
	// Ensure exactly 10 bytes for the reflector field with space-padding
	reflectorBytes := make([]byte, 10)
	copy(reflectorBytes, "REFLECTOR")
	for i := len("REFLECTOR"); i < 10; i++ {
		reflectorBytes[i] = ' '
	}
	copy(packet[4:14], reflectorBytes)
	return packet
}

// CreateStatusResponse creates a status response packet
func CreateStatusResponse(name, description string, count int) []byte {
	packet := make([]byte, StatusPacketSize)

	// Type
	copy(packet[0:4], PacketTypeStatus)

	// Hash (5-digit hash based on name) - some implementations expect this to be more specific
	hash := fmt.Sprintf("%05d", simpleHash(name)%100000)
	copy(packet[4:9], hash)

	// Name (16 bytes, space-padded like pYSFReflector)
	nameBytes := make([]byte, 16)
	copy(nameBytes, name)
	for i := len(name); i < 16; i++ {
		nameBytes[i] = ' '
	}
	copy(packet[9:25], nameBytes)

	// Description (14 bytes, space-padded)
	descBytes := make([]byte, 14)
	copy(descBytes, description)
	for i := len(description); i < 14; i++ {
		descBytes[i] = ' '
	}
	copy(packet[25:39], descBytes)

	// Count (3 bytes, zero-padded)
	countStr := fmt.Sprintf("%03d", count)
	copy(packet[39:42], countStr)

	return packet
}

// IsDataPacket checks if the packet is a data packet
func (p *Packet) IsDataPacket() bool {
	return p.Type == PacketTypeData
}

// IsPollPacket checks if the packet is a poll packet
func (p *Packet) IsPollPacket() bool {
	return p.Type == PacketTypePoll
}

// IsUnlinkPacket checks if the packet is an unlink packet
func (p *Packet) IsUnlinkPacket() bool {
	return p.Type == PacketTypeUnlink
}

// IsStatusRequest checks if the packet is a status request
func (p *Packet) IsStatusRequest() bool {
	// Accept both full 14-byte status requests and minimal 4-byte 'YSFS' probes
	return p.Type == PacketTypeStatus && (len(p.Data) == 14 || len(p.Data) == 4)
}

// GetSequence extracts sequence number from data packet
func (p *Packet) GetSequence() uint32 {
	if !p.IsDataPacket() || len(p.Data) < 18 {
		return 0
	}
	return binary.BigEndian.Uint32(p.Data[14:18])
}

// String returns a string representation of the packet
func (p *Packet) String() string {
	return fmt.Sprintf("Packet{Type: %s, Callsign: %s, Source: %s, Size: %d}",
		p.Type, p.Callsign, p.Source, len(p.Data))
}

// simpleHash creates a simple hash from a string
func simpleHash(s string) int {
	hash := 0
	for _, c := range s {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}
