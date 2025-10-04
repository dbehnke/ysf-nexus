package codec

import (
	"fmt"
)

// AMBE frame constants
const (
	// AMBE+2 vocoder uses 49-bit frames
	AMBEFrameBits = 49

	// YSF uses 5 voice channels (VCH) per data frame
	YSFVoiceChannels = 5

	// DMR uses 3 voice frames per packet (A, B, C in superframe)
	DMRVoiceFrames = 3

	// YSF YSFD packet total size
	YSFDPacketSize = 155

	// DMR DMRD voice payload size
	DMRDVoicePayloadSize = 33
)

// AMBEFrame represents a 49-bit AMBE+2 voice frame
type AMBEFrame struct {
	Bits [49]bool
}

// NewAMBEFrame creates a new AMBE frame
func NewAMBEFrame() *AMBEFrame {
	return &AMBEFrame{}
}

// FromBytes converts a byte slice to AMBE frame bits
// Expects at least 7 bytes (49 bits = 6.125 bytes, rounded to 7)
func (f *AMBEFrame) FromBytes(data []byte) error {
	if len(data) < 7 {
		return fmt.Errorf("insufficient data for AMBE frame: need 7 bytes, got %d", len(data))
	}

	// Extract 49 bits from the byte array
	for i := 0; i < 49; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8) // MSB first
		f.Bits[i] = (data[byteIndex] & (1 << bitIndex)) != 0
	}

	return nil
}

// ToBytes converts AMBE frame bits to bytes
// Returns 7 bytes (49 bits)
func (f *AMBEFrame) ToBytes() []byte {
	data := make([]byte, 7)

	for i := 0; i < 49; i++ {
		if f.Bits[i] {
			byteIndex := i / 8
			bitIndex := 7 - (i % 8) // MSB first
			data[byteIndex] |= (1 << bitIndex)
		}
	}

	return data
}

// Clone creates a copy of the AMBE frame
func (f *AMBEFrame) Clone() *AMBEFrame {
	clone := NewAMBEFrame()
	copy(clone.Bits[:], f.Bits[:])
	return clone
}

// IsValid checks if the frame contains valid data (not all zeros or all ones)
func (f *AMBEFrame) IsValid() bool {
	allZero := true
	allOne := true

	for i := 0; i < 49; i++ {
		if f.Bits[i] {
			allZero = false
		} else {
			allOne = false
		}
	}

	return !allZero && !allOne
}

// YSFVoicePayload represents the voice payload extracted from a YSFD packet
type YSFVoicePayload struct {
	VCH1 *AMBEFrame // Voice Channel 1
	VCH2 *AMBEFrame // Voice Channel 2
	VCH3 *AMBEFrame // Voice Channel 3
	VCH4 *AMBEFrame // Voice Channel 4
	VCH5 *AMBEFrame // Voice Channel 5
}

// NewYSFVoicePayload creates a new YSF voice payload
func NewYSFVoicePayload() *YSFVoicePayload {
	return &YSFVoicePayload{
		VCH1: NewAMBEFrame(),
		VCH2: NewAMBEFrame(),
		VCH3: NewAMBEFrame(),
		VCH4: NewAMBEFrame(),
		VCH5: NewAMBEFrame(),
	}
}

// DMRVoicePayload represents the voice payload from a DMRD packet
type DMRVoicePayload struct {
	Frame *AMBEFrame // Single AMBE frame per DMR packet
}

// NewDMRVoicePayload creates a new DMR voice payload
func NewDMRVoicePayload() *DMRVoicePayload {
	return &DMRVoicePayload{
		Frame: NewAMBEFrame(),
	}
}

// ExtractYSFVoice extracts AMBE frames from a YSFD packet
// YSFD packet structure (155 bytes total):
// - Bytes 0-3: "YSFD"
// - Bytes 4-13: Callsign
// - Bytes 14-33: Data (includes FICH, etc.)
// - Bytes 34-153: Voice frames (120 bytes)
// - Byte 154: End/Sequence
func ExtractYSFVoice(ysfPacket []byte) (*YSFVoicePayload, error) {
	if len(ysfPacket) < YSFDPacketSize {
		return nil, fmt.Errorf("invalid YSF packet size: %d", len(ysfPacket))
	}

	// Voice data starts at byte 35 (after header, callsign, FICH, etc.)
	// This is a simplified extraction - real implementation needs deinterleaving
	payload := NewYSFVoicePayload()

	// For now, extract placeholder data
	// TODO: Implement proper YSF deinterleaving and extraction
	voiceStart := 35

	if len(ysfPacket) >= voiceStart+35 {
		// Extract 5 voice channels (7 bytes each = 49 bits)
		for i := 0; i < YSFVoiceChannels; i++ {
			offset := voiceStart + (i * 7)
			var frame *AMBEFrame

			switch i {
			case 0:
				frame = payload.VCH1
			case 1:
				frame = payload.VCH2
			case 2:
				frame = payload.VCH3
			case 3:
				frame = payload.VCH4
			case 4:
				frame = payload.VCH5
			}

			if offset+7 <= len(ysfPacket) {
				if err := frame.FromBytes(ysfPacket[offset : offset+7]); err != nil {
					// Skip invalid frames
					continue
				}
			}
		}
	}

	return payload, nil
}

// ExtractDMRVoice extracts AMBE frame from a DMR voice packet
// DMR voice payload is 33 bytes in DMRD packet
func ExtractDMRVoice(dmrVoiceData []byte) (*DMRVoicePayload, error) {
	if len(dmrVoiceData) < DMRDVoicePayloadSize {
		return nil, fmt.Errorf("invalid DMR voice data size: %d", len(dmrVoiceData))
	}

	payload := NewDMRVoicePayload()

	// Extract AMBE bits from DMR payload
	// DMR packs AMBE frames with specific interleaving
	// This is simplified - real implementation needs DMR deinterleaving
	// TODO: Implement proper DMR deinterleaving

	// For now, extract first 7 bytes as AMBE frame
	if err := payload.Frame.FromBytes(dmrVoiceData[0:7]); err != nil {
		return nil, err
	}

	return payload, nil
}

// InjectYSFVoice injects AMBE frames into a YSFD packet template
func InjectYSFVoice(ysfPacket []byte, payload *YSFVoicePayload) error {
	if len(ysfPacket) < YSFDPacketSize {
		return fmt.Errorf("invalid YSF packet size: %d", len(ysfPacket))
	}

	// Inject voice data at the appropriate offset
	// TODO: Implement proper YSF interleaving and injection
	voiceStart := 35

	frames := []*AMBEFrame{
		payload.VCH1,
		payload.VCH2,
		payload.VCH3,
		payload.VCH4,
		payload.VCH5,
	}

	for i, frame := range frames {
		offset := voiceStart + (i * 7)
		if offset+7 <= len(ysfPacket) {
			frameBytes := frame.ToBytes()
			copy(ysfPacket[offset:offset+7], frameBytes)
		}
	}

	return nil
}

// InjectDMRVoice injects AMBE frame into a DMR voice payload
func InjectDMRVoice(dmrVoiceData []byte, payload *DMRVoicePayload) error {
	if len(dmrVoiceData) < DMRDVoicePayloadSize {
		return fmt.Errorf("invalid DMR voice data size: %d", len(dmrVoiceData))
	}

	// Inject AMBE bits into DMR payload with proper interleaving
	// TODO: Implement proper DMR interleaving and injection

	// For now, inject first 7 bytes
	frameBytes := payload.Frame.ToBytes()
	copy(dmrVoiceData[0:7], frameBytes)

	return nil
}

// BitManipulation helpers

// GetBit extracts a bit from a byte array
func GetBit(data []byte, bitIndex int) bool {
	byteIndex := bitIndex / 8
	bitPos := 7 - (bitIndex % 8) // MSB first
	if byteIndex >= len(data) {
		return false
	}
	return (data[byteIndex] & (1 << bitPos)) != 0
}

// SetBit sets a bit in a byte array
func SetBit(data []byte, bitIndex int, value bool) {
	byteIndex := bitIndex / 8
	bitPos := 7 - (bitIndex % 8) // MSB first
	if byteIndex >= len(data) {
		return
	}

	if value {
		data[byteIndex] |= (1 << bitPos)
	} else {
		data[byteIndex] &= ^(1 << bitPos)
	}
}

// ExtractBits extracts a range of bits from a byte array
func ExtractBits(data []byte, startBit, numBits int) []bool {
	bits := make([]bool, numBits)
	for i := 0; i < numBits; i++ {
		bits[i] = GetBit(data, startBit+i)
	}
	return bits
}

// InjectBits injects bits into a byte array at a specific position
func InjectBits(data []byte, startBit int, bits []bool) {
	for i, bit := range bits {
		SetBit(data, startBit+i, bit)
	}
}
