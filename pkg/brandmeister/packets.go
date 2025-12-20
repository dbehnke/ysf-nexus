package brandmeister

import (
	"encoding/binary"
	"fmt"
)

// BrandMeister YSF Direct packet headers
const (
	HeaderYSFL   = "YSFL"
	HeaderYSFK   = "YSFK"
	HeaderYSFO   = "YSFO"
	HeaderYSFACK = "YSFACK"
	HeaderRPTK   = "RPTK"
	HeaderYSFP   = "YSFP"
	HeaderYSFD   = "YSFD"
)

// YSFLPacket represents a Login packet
type YSFLPacket struct {
	Callsign string // 10 characters, space padded
}

// Marshal serializes the YSFL packet
func (p *YSFLPacket) Marshal() ([]byte, error) {
	data := make([]byte, 14)
	copy(data[0:4], HeaderYSFL)

	cs := p.Callsign
	if len(cs) < 10 {
		cs = fmt.Sprintf("%-10s", cs)
	} else if len(cs) > 10 {
		cs = cs[:10]
	}
	copy(data[4:14], cs)

	return data, nil
}

// YSFKPacket represents a Challenge packet
type YSFKPacket struct {
	Challenge uint32
}

// Unmarshal deserializes the YSFK packet
func (p *YSFKPacket) Unmarshal(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("packet too small for YSFK: %d bytes", len(data))
	}
	if string(data[:4]) != HeaderYSFK {
		return fmt.Errorf("not a YSFK packet: %s", string(data[:4]))
	}

	p.Challenge = binary.LittleEndian.Uint32(data[4:8])
	return nil
}

// YSFOPacket represents an Options packet
type YSFOPacket struct {
	Options string
}

// Marshal serializes the YSFO packet
func (p *YSFOPacket) Marshal() ([]byte, error) {
	data := make([]byte, 4+len(p.Options))
	copy(data[0:4], HeaderYSFO)
	copy(data[4:], p.Options)
	return data, nil
}

// YSFACKPacket represents an Acknowledge packet
type YSFACKPacket struct{}

// Marshal serializes the YSFACK packet
func (p *YSFACKPacket) Marshal() ([]byte, error) {
	return []byte(HeaderYSFACK), nil
}

// RPTKPacket represents an Authentication Response packet
type RPTKPacket struct {
	RepeaterID uint32
	Hash       [32]byte
}

// Marshal serializes the RPTK packet
func (p *RPTKPacket) Marshal() ([]byte, error) {
	data := make([]byte, 40)
	copy(data[0:4], HeaderRPTK)
	binary.LittleEndian.PutUint32(data[4:8], p.RepeaterID)
	copy(data[8:40], p.Hash[:])
	return data, nil
}
