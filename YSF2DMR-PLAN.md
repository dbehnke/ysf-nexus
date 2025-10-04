# YSF2DMR Bridge Implementation Plan

## Current Status

### What Works ✅
- **YSF → DMR**: Working! YSF repeater traffic successfully converts to DMR and transmits to TGIF network
- **DMR Packet Reception**: DMR packets are received from TGIF network (verified in logs)
- **YSFD Packet Header**: Now correctly formatted to match C++ MMDVM_CM implementation
- **Packet Routing**: DMR→YSF converted packets are being sent to YSF repeaters (confirmed via tcpdump)

### What Doesn't Work ❌
- **DMR → YSF Audio**: YSF clients receive packets but can't decode the audio
- **Voice Data Conversion**: Current simplified codec conversion doesn't implement proper AMBE frame processing
- **Dashboard Visibility**: DMR bridge talkers don't appear in the web dashboard (no talk_start events)

## Root Cause Analysis

The current `pkg/codec/conversion.go` DMR→YSF implementation is too simplified. It attempts basic AMBE frame conversion but **doesn't implement the proper MMDVM_CM ModeConv logic**:

### What We're Missing

1. **DMR Deinterleaving**: Extracting 3 AMBE frames from each 55-byte DMR packet using lookup tables
2. **PRNG Whitening**: XORing the B component with PRNG_TABLE values
3. **Bit Triplication**: Each bit of A and B components must be written 3 times
4. **Scrambling**: XORing with WHITENING_DATA after bit arrangement
5. **Interleaving**: Reordering bits using INTERLEAVE_TABLE_26_4
6. **Frame Buffering**: Properly buffering and outputting 13-byte YSF voice frames

## Reference Implementation

**Source**: https://github.com/nostar/MMDVM_CM/tree/master/YSF2DMR

### Key Files
- `ModeConv.cpp` / `ModeConv.h` - Core conversion logic
- `YSF2DMR.cpp` - Main bridge implementation showing packet assembly

### Packet Structure (from C++ code)

#### YSFD Packet Layout (155 bytes)
```
Bytes 0-3:   "YSFD" (magic header)
Bytes 4-13:  Local callsign (reflector/gateway) - 10 bytes, space-padded
Bytes 14-23: Source callsign (who's talking) - 10 bytes, space-padded
Bytes 24-33: Destination callsign ("ALL" for group calls) - 10 bytes, space-padded
Byte 34:     Net frame counter (0-255)
Bytes 35+:   Voice data (from ModeConv.getYSF())
```

**Status**: ✅ Header structure now correctly implemented in `pkg/codec/conversion.go:92-124`

### DMR → YSF Conversion Flow

```
DMR Packet (55 bytes)
    ↓
putDMR() - Extract 3 AMBE triplets using DMR_A/B/C_TABLE
    ↓
For each triplet (a, b, c):
    putAMBE2YSF(a, b, c)
        ↓
    1. dat_a = a >> 12 (top 12 bits)
    2. b ^= (PRNG_TABLE[dat_a] >> 1)  ← PRNG whitening
    3. dat_b = b >> 11 (top 12 bits)
    4. Write dat_a bits: triplicate each bit (12 → 36 bits)
    5. Write dat_b bits: triplicate each bit (12 → 36 bits)
    6. Write dat_c bits: specific positions
    7. XOR with WHITENING_DATA[] (scrambling)
    8. Interleave using INTERLEAVE_TABLE_26_4
        ↓
    13-byte YSF voice frame
        ↓
Buffer until 5 frames available
    ↓
Build complete YSFD packet (header + voice data)
```

## Required Lookup Tables

### 1. DMR_A_TABLE (24 values)
```go
var DMR_A_TABLE = []uint{
    0, 4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44,
    48, 52, 56, 60, 64, 68, 1, 5, 9, 13, 17, 21,
}
```

### 2. DMR_B_TABLE (23 values)
```go
var DMR_B_TABLE = []uint{
    25, 29, 33, 37, 41, 45, 49, 53, 57, 61, 65, 69,
    2, 6, 10, 14, 18, 22, 26, 30, 34, 38, 42,
}
```

### 3. DMR_C_TABLE (25 values)
```go
var DMR_C_TABLE = []uint{
    46, 50, 54, 58, 62, 66, 70, 3, 7, 11, 15, 19, 23,
    27, 31, 35, 39, 43, 47, 51, 55, 59, 63, 67, 71,
}
```

### 4. PRNG_TABLE (720 values)
**See**: ModeConv.cpp lines 40-200 (extracted in session, available in curl output)

### 5. INTERLEAVE_TABLE_26_4 (104 values)
```go
var INTERLEAVE_TABLE_26_4 = []uint{
    0, 4, 8, 12, 16, 20, 24, 28, 32, 36, 40, 44, 48, 52, 56, 60, 64, 68, 72, 76, 80, 84, 88, 92, 96, 100,
    1, 5, 9, 13, 17, 21, 25, 29, 33, 37, 41, 45, 49, 53, 57, 61, 65, 69, 73, 77, 81, 85, 89, 93, 97, 101,
    2, 6, 10, 14, 18, 22, 26, 30, 34, 38, 42, 46, 50, 54, 58, 62, 66, 70, 74, 78, 82, 86, 90, 94, 98, 102,
    3, 7, 11, 15, 19, 23, 27, 31, 35, 39, 43, 47, 51, 55, 59, 63, 67, 71, 75, 79, 83, 87, 91, 95, 99, 103,
}
```

### 6. WHITENING_DATA (20 bytes)
```go
var WHITENING_DATA = []byte{
    0x93, 0xD7, 0x51, 0x21, 0x9C, 0x2F, 0x6C, 0xD0, 0xEF, 0x0F,
    0xF8, 0x3D, 0xF1, 0x73, 0x20, 0x94, 0xED, 0x1E, 0x7C, 0xD8,
}
```

## Implementation Tasks

### Phase 1: Create ModeConv Package
**File**: `pkg/codec/modeconv.go`

```go
package codec

// ModeConv handles DMR ↔ YSF voice frame conversion
type ModeConv struct {
    ysfBuffer []byte  // Ring buffer for YSF frames
    dmrBuffer []byte  // Ring buffer for DMR frames
    ysfN      int     // YSF frame counter
    dmrN      int     // DMR frame counter
}

func NewModeConv() *ModeConv {
    return &ModeConv{
        ysfBuffer: make([]byte, 5000*13), // 13 bytes per YSF frame
        dmrBuffer: make([]byte, 5000*9),  // 9 bytes per DMR frame
    }
}

// PutDMR processes a DMR voice frame and converts to YSF
func (c *ModeConv) PutDMR(dmrData []byte) error {
    // Extract 3 AMBE triplets from DMR packet
    a1, b1, c1 := c.extractAMBE(dmrData, 0)
    a2, b2, c2 := c.extractAMBE(dmrData, 72)
    a3, b3, c3 := c.extractAMBE(dmrData, 144)

    // Convert each triplet to YSF
    c.putAMBE2YSF(a1, b1, c1)
    c.putAMBE2YSF(a2, b2, c2)
    c.putAMBE2YSF(a3, b3, c3)

    return nil
}

// GetYSF retrieves a converted YSF voice frame (13 bytes)
func (c *ModeConv) GetYSF() ([]byte, error) {
    // Return next 13-byte YSF frame from buffer
    // Return nil if buffer doesn't have enough frames yet
}

func (c *ModeConv) extractAMBE(data []byte, offset uint) (a, b, c uint32) {
    // Use DMR_A_TABLE, DMR_B_TABLE, DMR_C_TABLE to extract bits
}

func (c *ModeConv) putAMBE2YSF(a, b, c uint32) {
    // 1. dat_a = a >> 12
    // 2. b ^= (PRNG_TABLE[dat_a] >> 1)
    // 3. dat_b = b >> 11
    // 4. Create 13-byte VCH with tripled bits
    // 5. XOR with WHITENING_DATA
    // 6. Interleave with INTERLEAVE_TABLE_26_4
    // 7. Append to ysfBuffer
}
```

### Phase 2: Update Converter to Use ModeConv
**File**: `pkg/codec/conversion.go`

Replace current simplified conversion with:

```go
type Converter struct {
    modeConv *ModeConv  // Use proper ModeConv instead of simple conversion

    // Keep existing metadata
    srcCallsign string
    srcDMRID    uint32
}

func (c *Converter) DMRToYSF(dmrVoiceData []byte) ([]byte, error) {
    // Feed DMR data to ModeConv
    if err := c.modeConv.PutDMR(dmrVoiceData); err != nil {
        return nil, err
    }

    // Try to get 5 YSF voice frames (65 bytes = 5 * 13)
    ysfVoice, err := c.getYSFFrames(5)
    if err != nil {
        return nil, nil  // Buffering, not enough frames yet
    }

    // Build complete YSFD packet with proper header
    ysfPacket := c.buildYSFDPacket(ysfVoice)
    return ysfPacket, nil
}

func (c *Converter) buildYSFDPacket(voiceData []byte) []byte {
    packet := make([]byte, 155)

    // Bytes 0-3: "YSFD"
    copy(packet[0:4], "YSFD")

    // Bytes 4-13: Local callsign
    copy(packet[4:14], "DMR       ")

    // Bytes 14-23: Source callsign (DMR talker)
    srcCall := c.srcCallsign
    if srcCall == "" {
        srcCall = fmt.Sprintf("DMR%d", c.srcDMRID)
    }
    if len(srcCall) > 10 {
        srcCall = srcCall[:10]
    }
    copy(packet[14:24], srcCall)
    for i := 14 + len(srcCall); i < 24; i++ {
        packet[i] = ' '
    }

    // Bytes 24-33: Destination "ALL"
    copy(packet[24:34], "ALL       ")

    // Byte 34: Frame counter
    packet[34] = c.frameCount

    // Bytes 35+: Voice data
    copy(packet[35:], voiceData)

    return packet
}
```

### Phase 3: Integration
**File**: `pkg/ysf2dmr/bridge.go`

Update to use new converter:

```go
// In handleDMRPacket(), line 504:
ysfPacket, err := b.converter.DMRToYSF(dmrdPacket.Data)
// The rest stays the same - broadcasts to repeaters
```

### Phase 4: Dashboard Integration

Add event emission when DMR calls start/end so they appear in dashboard:

```go
// In handleDMRPacket(), after starting new call:
if b.eventChan != nil {
    b.eventChan <- Event{
        Type:      "talk_start",
        Callsign:  dmrCallsign,
        Timestamp: time.Now(),
    }
}

// On call end (EOT or timeout):
if b.eventChan != nil {
    b.eventChan <- Event{
        Type:      "talk_end",
        Callsign:  dmrCallsign,
        Duration:  &duration,
        Timestamp: time.Now(),
    }
}
```

## Testing Plan

### Test 1: Verify Packet Structure
```bash
# Capture YSFD packets with tcpdump
sudo tcpdump -i any -X udp port 42000 -w dmr2ysf.pcap

# Verify in Wireshark:
# - Bytes 0-3 = "YSFD"
# - Bytes 4-13 = Local callsign
# - Bytes 14-23 = Source DMR callsign
# - Bytes 24-33 = "ALL"
# - Byte 34 = Frame counter (incrementing)
# - Bytes 35+ = Voice data (should not be all zeros)
```

### Test 2: Audio Quality
- Key up on DMR → should hear audio on YSF client
- Verify no static/distortion
- Check callsign displays correctly in YSF client

### Test 3: Dashboard
- DMR talker should appear in "Current Talker" section
- Bridge name and callsign should be visible
- Talk duration should be tracked

## Files to Modify

1. ✅ **`pkg/codec/conversion.go`** - Header structure fixed, needs voice conversion update
2. 🆕 **`pkg/codec/modeconv.go`** - New file with ModeConv implementation
3. 🔄 **`pkg/ysf2dmr/bridge.go`** - Add event emission for dashboard
4. 📝 **`pkg/codec/tables.go`** - New file to hold lookup tables (optional, keep them in modeconv.go)

## Estimated Effort

- **Phase 1 (ModeConv package)**: 4-6 hours
  - Implement lookup tables ✓
  - Port C++ bit manipulation logic
  - Test frame extraction and conversion

- **Phase 2 (Converter update)**: 2-3 hours
  - Update DMRToYSF to use ModeConv
  - Keep packet header structure (already done)
  - Test with real DMR traffic

- **Phase 3 (Integration)**: 1-2 hours
  - Ensure bridge uses updated converter
  - Verify end-to-end packet flow

- **Phase 4 (Dashboard)**: 1-2 hours
  - Add event emission
  - Test UI updates

**Total**: ~8-13 hours of development + testing

## Success Criteria

- [x] YSFD packet header matches C++ implementation
- [ ] Voice data properly converted using ModeConv logic
- [ ] YSF clients can decode and play DMR audio
- [ ] DMR talkers appear in web dashboard
- [ ] No audio artifacts or distortion
- [ ] Frame synchronization maintained during long transmissions

## Notes from Debugging Session

### Findings
1. **tcpdump confirms packets arrive** at YSF client (127.0.0.1:51708)
2. **YSF client receives but doesn't decode** - indicates malformed voice data
3. **Server instance is shared correctly** - both reflector and bridge use `0x1400012e540`
4. **Header was wrong** - Fixed to match C++ byte layout
5. **Voice conversion is simplified** - Doesn't implement PRNG/whitening/interleaving

### Log Evidence
```
Generated YSF packet from DMR
  size: 155
  header: 59534644444D5220202020202020444D522D3331383233323820414C4C2020202020202020
          ^YSFD   ^DMR (local) ^DMR-3182328 (src) ^ALL (dest)
  callsign: DMR-318232 (truncated - should be DMR-3182328)
```

### Callsign Truncation Issue
The DMR ID 3182328 creates "DMR3182328" (11 chars) → truncated to "DMR3182328"[:10] = "DMR318232"

**Fix**: Use lookup table or shorten format (e.g., "3182328" without "DMR" prefix)

## References

- **C++ Implementation**: https://github.com/nostar/MMDVM_CM/tree/master/YSF2DMR
- **ModeConv.cpp**: Core conversion logic
- **YSF2DMR.cpp**: Main bridge showing packet assembly
- **MMDVM Project**: https://github.com/g4klx/MMDVM (original by G4KLX)

## License Considerations

The MMDVM_CM code is GPL v2. Our Go port must:
- Maintain GPL v2 license compatibility
- Credit original authors (Jonathan Naylor G4KLX, Andy Uribe CA6JAU)
- Include GPL license headers in new files

---

**Created**: 2025-10-03
**Last Updated**: 2025-10-03
**Status**: Planning complete, ready for implementation
**Next Step**: Implement `pkg/codec/modeconv.go` with full ModeConv logic
