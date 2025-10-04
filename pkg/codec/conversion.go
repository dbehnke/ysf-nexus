package codec

import (
	"fmt"
)

// Converter handles YSF ↔ DMR audio conversion
type Converter struct {
	// Conversion state
	ysfFrameCount uint8
	dmrFrameCount uint8

	// Buffers for multi-frame alignment
	// YSF sends 5 VCH per frame, DMR uses 1 frame at a time
	ysfBuffer []*AMBEFrame
	dmrBuffer []*AMBEFrame

	// Metadata
	srcCallsign string
	srcDMRID    uint32
}

// NewConverter creates a new YSF ↔ DMR converter
func NewConverter() *Converter {
	return &Converter{
		ysfBuffer: make([]*AMBEFrame, 0, 5),
		dmrBuffer: make([]*AMBEFrame, 0, 3),
	}
}

// YSFToDMR converts a YSF YSFD packet to DMR DMRD voice data
// YSF sends 5 voice channels per packet, DMR sends 1 at a time
// Returns DMR voice payload (33 bytes) or nil if buffering
func (c *Converter) YSFToDMR(ysfPacket []byte) ([]byte, error) {
	if len(ysfPacket) < YSFDPacketSize {
		return nil, fmt.Errorf("invalid YSF packet size: %d", len(ysfPacket))
	}

	// Extract YSF voice payload (5 AMBE frames)
	payload, err := ExtractYSFVoice(ysfPacket)
	if err != nil {
		return nil, fmt.Errorf("failed to extract YSF voice: %w", err)
	}

	// Convert each VCH to DMR format
	// For simplicity, we'll output DMR frames in sequence
	// In a real implementation, you'd need to align frame rates

	// Process VCH1 as example
	dmrVoice, err := c.convertYSFFrameToDMR(payload.VCH1)
	if err != nil {
		return nil, err
	}

	c.ysfFrameCount++

	return dmrVoice, nil
}

// DMRToYSF converts DMR DMRD voice data to YSF YSFD packet
// DMR sends 1 frame at a time, YSF needs 5 frames
// Returns YSF packet (155 bytes) or nil if buffering
func (c *Converter) DMRToYSF(dmrVoiceData []byte) ([]byte, error) {
	if len(dmrVoiceData) < DMRDVoicePayloadSize {
		return nil, fmt.Errorf("invalid DMR voice data size: %d", len(dmrVoiceData))
	}

	// Extract DMR voice payload (1 AMBE frame)
	payload, err := ExtractDMRVoice(dmrVoiceData)
	if err != nil {
		return nil, fmt.Errorf("failed to extract DMR voice: %w", err)
	}

	// Convert DMR frame to YSF format
	ysfFrame, err := c.convertDMRFrameToYSF(payload.Frame)
	if err != nil {
		return nil, err
	}

	// Buffer DMR frames until we have enough for a YSF packet
	c.dmrBuffer = append(c.dmrBuffer, ysfFrame)

	// YSF needs 5 frames
	if len(c.dmrBuffer) < 5 {
		// Not enough frames yet, return nil to indicate buffering
		return nil, nil
	}

	// Build YSF packet from buffered frames
	ysfPacket := make([]byte, YSFDPacketSize)

	// Build YSF header matching C++ YSF2DMR implementation
	// Structure from MMDVM_CM/YSF2DMR:
	// Bytes 0-3: "YSFD"
	// Bytes 4-13: Local callsign (reflector/gateway)
	// Bytes 14-23: Source callsign (who's talking)
	// Bytes 24-33: Destination callsign ("ALL" for group calls)
	// Byte 34: Net frame counter
	// Bytes 35+: Voice data

	copy(ysfPacket[0:4], "YSFD")

	// Bytes 4-13: Local callsign (reflector - use a placeholder)
	copy(ysfPacket[4:14], "DMR       ") // 10 bytes, space-padded

	// Bytes 14-23: Source callsign (the DMR talker)
	srcCallsign := c.srcCallsign
	if srcCallsign == "" {
		srcCallsign = fmt.Sprintf("DMR%d", c.srcDMRID)
	}
	// Pad callsign to 10 bytes
	if len(srcCallsign) > 10 {
		srcCallsign = srcCallsign[:10]
	}
	copy(ysfPacket[14:24], srcCallsign)
	for i := 14 + len(srcCallsign); i < 24; i++ {
		ysfPacket[i] = ' ' // Space padding
	}

	// Bytes 24-33: Destination callsign (ALL for group calls)
	copy(ysfPacket[24:34], "ALL       ") // 10 bytes, space-padded

	// Byte 34: Net frame counter
	ysfPacket[34] = c.dmrFrameCount

	// Inject voice frames
	ysfPayload := &YSFVoicePayload{
		VCH1: c.dmrBuffer[0],
		VCH2: c.dmrBuffer[1],
		VCH3: c.dmrBuffer[2],
		VCH4: c.dmrBuffer[3],
		VCH5: c.dmrBuffer[4],
	}

	if err := InjectYSFVoice(ysfPacket, ysfPayload); err != nil {
		return nil, err
	}

	// Clear buffer
	c.dmrBuffer = c.dmrBuffer[:0]
	c.dmrFrameCount++

	return ysfPacket, nil
}

// convertYSFFrameToDMR converts a single YSF AMBE frame to DMR format
func (c *Converter) convertYSFFrameToDMR(ysfFrame *AMBEFrame) ([]byte, error) {
	// 1. Extract AMBE bits from YSF frame
	ysfBits := ysfFrame.Bits[:]

	// 2. Deinterleave YSF data (if frame was 104 bits)
	// For 49-bit AMBE, we work directly with the bits
	deinterleaved := make([]bool, 49)
	copy(deinterleaved, ysfBits)

	// 3. Descramble YSF whitening
	descrambled := DescrambleYSF(deinterleaved)

	// 4. Apply error correction if needed (Golay for some bits)
	// AMBE frames have specific FEC patterns
	corrected := c.applyErrorCorrection(descrambled)

	// 5. Re-scramble for DMR
	scrambled := ScrambleDMR(corrected)

	// 6. Interleave for DMR (frame type 0 = A frame)
	interleaved := InterleaveDMR(scrambled, 0)

	// 7. Pack into DMR voice payload (33 bytes)
	dmrPayload := make([]byte, DMRDVoicePayloadSize)

	// Convert bits to bytes
	for i := 0; i < 72 && i < len(interleaved); i++ {
		if interleaved[i] {
			byteIndex := i / 8
			bitIndex := 7 - (i % 8)
			if byteIndex < len(dmrPayload) {
				dmrPayload[byteIndex] |= (1 << bitIndex)
			}
		}
	}

	return dmrPayload, nil
}

// convertDMRFrameToYSF converts a single DMR AMBE frame to YSF format
func (c *Converter) convertDMRFrameToYSF(dmrFrame *AMBEFrame) (*AMBEFrame, error) {
	// 1. Extract AMBE bits from DMR frame
	dmrBits := dmrFrame.Bits[:]

	// 2. Deinterleave DMR data (frame type 0 = A frame)
	deinterleaved := DeinterleaveDMR(dmrBits, 0)

	// 3. Descramble DMR whitening
	descrambled := DescrambletDMR(deinterleaved)

	// 4. Apply error correction if needed
	corrected := c.applyErrorCorrection(descrambled)

	// 5. Re-scramble for YSF
	scrambled := ScrambleYSF(corrected)

	// 6. Interleave for YSF
	interleaved := InterleaveYSF(scrambled)

	// 7. Pack into YSF AMBE frame
	ysfFrame := NewAMBEFrame()
	for i := 0; i < 49 && i < len(interleaved); i++ {
		ysfFrame.Bits[i] = interleaved[i]
	}

	return ysfFrame, nil
}

// applyErrorCorrection applies FEC to AMBE bits
func (c *Converter) applyErrorCorrection(bits []bool) []bool {
	// AMBE frames use Golay(24,12) for some bit groups
	// This is a simplified version - full implementation would
	// apply Golay to specific bit ranges

	corrected := make([]bool, len(bits))
	copy(corrected, bits)

	// For now, just pass through
	// TODO: Apply Golay to appropriate bit ranges
	// Example: bits 0-23 might be one Golay codeword

	return corrected
}

// Reset resets the converter state
func (c *Converter) Reset() {
	c.ysfFrameCount = 0
	c.dmrFrameCount = 0
	c.ysfBuffer = c.ysfBuffer[:0]
	c.dmrBuffer = c.dmrBuffer[:0]
}

// GetFrameCounts returns the conversion statistics
func (c *Converter) GetFrameCounts() (ysfFrames, dmrFrames uint8) {
	return c.ysfFrameCount, c.dmrFrameCount
}

// SetMetadata sets metadata for the conversion
func (c *Converter) SetMetadata(callsign string, dmrID uint32) {
	c.srcCallsign = callsign
	c.srcDMRID = dmrID
}

// ConvertYSFToDMRSimple is a simplified conversion for testing
// Converts YSF AMBE bits directly to DMR format without full protocol overhead
func ConvertYSFToDMRSimple(ysfBits []bool) []bool {
	if len(ysfBits) != 49 {
		return ysfBits
	}

	// Deinterleave -> Descramble -> Scramble -> Interleave
	descrambled := DescrambleYSF(ysfBits)
	scrambled := ScrambleDMR(descrambled)
	interleaved := InterleaveDMR(scrambled, 0)

	return interleaved
}

// ConvertDMRToYSFSimple is a simplified conversion for testing
// Converts DMR AMBE bits directly to YSF format without full protocol overhead
func ConvertDMRToYSFSimple(dmrBits []bool) []bool {
	if len(dmrBits) != 72 {
		return dmrBits
	}

	// Deinterleave -> Descramble -> Scramble -> Interleave
	deinterleaved := DeinterleaveDMR(dmrBits, 0)
	descrambled := DescrambletDMR(deinterleaved)
	scrambled := ScrambleYSF(descrambled)
	interleaved := InterleaveYSF(scrambled)

	return interleaved
}

// Helper function to build a DMR voice packet header
func BuildDMRVoiceHeader() []byte {
	// Returns minimal DMR voice header for testing
	header := make([]byte, DMRDVoicePayloadSize)
	// TODO: Add proper DMR voice header structure
	return header
}

// Helper function to build a YSF packet header
func BuildYSFPacketHeader(callsign string) []byte {
	// Returns minimal YSF packet header for testing
	header := make([]byte, 35) // Up to voice data start
	copy(header[0:4], "YSFD")

	// Callsign at bytes 4-13 (10 bytes)
	if len(callsign) > 10 {
		callsign = callsign[:10]
	}
	copy(header[4:14], callsign)

	// TODO: Add proper FICH, FT, etc.
	return header
}
