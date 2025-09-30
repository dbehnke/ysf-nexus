# YSF2DMR Bridge Implementation Plan

## Overview
Add DMR (Digital Mobile Radio) bridging capability to YSF Nexus, enabling cross-mode communication between YSF reflector and DMR networks. This implementation is based on the proven architecture from [nostar/MMDVM_CM/YSF2DMR](https://github.com/nostar/MMDVM_CM/tree/master/YSF2DMR).

## Goals
- **Cross-Mode Communication**: Seamlessly bridge YSF voice traffic to DMR networks and vice versa
- **Audio Transcoding**: Convert between YSF and DMR AMBE voice formats
- **Flexible Routing**: Support multiple DMR destinations (talkgroups, private calls)
- **Robust Networking**: Handle DMR network authentication and state management
- **Monitoring**: Real-time visibility into bridge status and activity
- **Go-Native**: Implement in pure Go without external C/C++ dependencies where possible

## YSF2DMR Protocol Analysis

### DMR Network Protocol
Based on the C++ implementation analysis:

1. **Connection Authentication**:
   - Multi-step handshake: `RPTL` (login) → server salt → `RPTK` (SHA256 password) → `RPTC` (configuration)
   - Requires DMR ID, password, and network-specific configuration
   - Keep-alive packets required to maintain connection

2. **Packet Structure** (55 bytes for voice):
   ```
   Byte 0-3:   Signature (varies by packet type)
   Byte 4-7:   Sequence number
   Byte 8-11:  Source DMR ID (3 bytes BCD-encoded)
   Byte 12-15: Destination DMR ID (3 bytes BCD-encoded)
   Byte 16:    Slot number (1 or 2)
   Byte 17:    Call type (group/private)
   Byte 18:    Frame type (voice header, voice sync, voice data, terminator)
   Byte 19-54: Voice payload (AMBE frames) or data
   ```

3. **Voice Transmission Flow**:
   - Voice Header → Voice Sync → Voice Data (repeating) → Voice Terminator
   - Each voice frame contains AMBE audio data
   - Sequence numbers track frame order
   - Color codes and timeslots organize traffic

### Audio Conversion Pipeline

The YSF ↔ DMR conversion involves complex bit-level operations:

1. **AMBE Codec Differences**:
   - **YSF**: AMBE+2 vocoder (49-bit frames), DN mode or VW mode
   - **DMR**: AMBE+2 vocoder (49-bit frames), but different framing and interleaving
   - Both use same core codec but different bit arrangements

2. **Conversion Steps** (from ModeConv.cpp):
   ```
   YSF → DMR:
   1. Extract YSF AMBE frame from 120-byte YSF packet
   2. Deinterleave bits using YSF-specific pattern
   3. Descramble with YSF whitening sequence
   4. Re-interleave for DMR format
   5. Apply DMR scrambling pattern
   6. Inject into 55-byte DMR packet with headers

   DMR → YSF:
   1. Extract DMR AMBE frame from 55-byte packet
   2. Deinterleave bits using DMR-specific pattern
   3. Descramble with DMR whitening sequence
   4. Re-interleave for YSF format
   5. Apply YSF scrambling pattern
   6. Inject into 120-byte YSF YSFD packet
   ```

3. **Error Correction**:
   - Golay(24,12) codes for protection
   - CRC checks for data integrity
   - FEC (Forward Error Correction) for voice quality

4. **Metadata Translation**:
   - Callsign mapping (YSF callsign ↔ DMR ID via database)
   - GPS coordinates conversion
   - Talker alias handling

### Configuration Requirements

From Conf.cpp analysis, YSF2DMR needs:

```yaml
ysf2dmr:
  enabled: true

  # YSF side configuration
  ysf:
    callsign: "W1ABC"          # Station callsign (converted to DMR ID)
    suffix: ""                  # Optional suffix
    local_address: "0.0.0.0"
    local_port: 42001           # Different from main reflector to avoid conflicts
    enable_wiresx: false        # WiresX control commands
    hang_time: 5s               # Time before considering transmission ended

  # DMR network configuration
  dmr:
    enabled: true
    id: 1234567                 # DMR ID of this bridge
    network: "BrandMeister"     # Network name for logging
    address: "bm.example.com"   # DMR network server
    port: 62030                 # DMR network port
    password: "passw0rd"        # Network password (hashed with SHA256)

    # Routing configuration
    startup_tg: 91              # Default talkgroup on startup
    slot: 2                     # DMR timeslot (1 or 2)
    color_code: 1               # Color code for network

    # Optional features
    enable_private_call: false  # Allow PC in addition to group calls
    xlx_reflector: 0            # XLX reflector number (0 = disabled)
    xlx_module: ""              # XLX module letter

  # ID Lookup database
  lookup:
    enabled: true
    dmr_id_file: "DMRIds.dat"   # DMR ID → Callsign mapping
    refresh_interval: 24h        # How often to reload database

  # Audio conversion settings
  audio:
    gain: 1.0                   # Audio gain adjustment (0.5 - 2.0)
    vox_enabled: true           # Voice-activated transmission
    vox_threshold: 0.1          # VOX sensitivity

  # Monitoring
  logging:
    log_dmr_packets: false      # Detailed packet logging (very verbose)
    log_conversions: true       # Log audio conversion events
```

## Architecture Design

### New Package Structure

```
pkg/
├── dmr/
│   ├── network.go           # DMR network client (authentication, packet I/O)
│   ├── packets.go           # DMR packet structures and parsing
│   ├── codec.go             # AMBE frame handling
│   ├── lookup.go            # DMR ID database management
│   └── network_test.go
├── codec/
│   ├── ambe.go              # AMBE codec utilities
│   ├── conversion.go        # YSF ↔ DMR audio conversion
│   ├── interleave.go        # Bit interleaving/deinterleaving
│   ├── golay.go             # Golay error correction
│   └── conversion_test.go
└── ysf2dmr/
    ├── bridge.go            # Main YSF2DMR bridge coordinator
    ├── ysf_handler.go       # YSF packet reception and forwarding
    ├── dmr_handler.go       # DMR packet reception and forwarding
    ├── state.go             # Bridge state machine
    └── bridge_test.go

configs/
└── ysf2dmr-example.yaml     # Example configuration

docs/
└── YSF2DMR.md              # User documentation (this file)
```

### Component Breakdown

#### 1. DMR Network Client (`pkg/dmr/network.go`)

```go
type Network struct {
    address      string
    port         int
    dmrID        uint32
    password     string
    conn         *net.UDPConn

    // Authentication state
    authenticated bool
    salt          []byte
    streamID      uint32

    // Packet handling
    rxChan       chan Packet
    txQueue      chan Packet

    // Statistics
    packetsRx    uint64
    packetsTx    uint64
}

func (n *Network) Connect(ctx context.Context) error
func (n *Network) Authenticate() error
func (n *Network) SendVoiceHeader(srcID, dstID uint32, slot byte) error
func (n *Network) SendVoiceData(ambe []byte, seq uint8) error
func (n *Network) SendVoiceTerminator() error
func (n *Network) ReceivePacket() (*Packet, error)
```

**Key Features**:
- SHA256 password hashing with server-provided salt
- Automatic keep-alive (ping every 5-10 seconds)
- Reconnection logic on connection loss
- Sequence number management
- Stream ID generation

#### 2. Audio Conversion Engine (`pkg/codec/conversion.go`)

```go
type Converter struct {
    // Conversion state
    ysfToNativeBuffer []byte  // Intermediate PCM or AMBE buffer
    nativeToDMRBuffer []byte

    // Frame alignment
    ysfFrameCount  uint8
    dmrFrameCount  uint8

    // Metadata
    srcCallsign    string
    srcDMRID       uint32
}

func NewConverter() *Converter
func (c *Converter) YSFToDMR(ysfFrame []byte) ([]byte, error)
func (c *Converter) DMRToYSF(dmrFrame []byte) ([]byte, error)

// Internal conversion utilities
func deinterleaveYSF(data []byte) []byte
func descrambleYSF(data []byte) []byte
func interleaveDMR(data []byte) []byte
func scrambleDMR(data []byte) []byte
func encodeGolay2412(data []byte) []byte
func decodeGolay2412(data []byte) ([]byte, error)
```

**Conversion Approach**:
- **Option A**: Bit-level transcoding (complex but no external dependencies)
  - Implement YSF/DMR interleaving tables in Go
  - Pure Go Golay code implementation
  - Direct AMBE frame manipulation

- **Option B**: PCM intermediate (simpler, may require codec library)
  - AMBE decode to PCM (requires codec library or hardware)
  - PCM encode to target format
  - Higher latency but cleaner implementation

**Recommendation**: Start with Option A (bit-level) since both YSF and DMR use AMBE+2 with 49-bit frames. The conversion is primarily re-framing and interleaving, not full decode/encode.

#### 3. DMR ID Lookup (`pkg/dmr/lookup.go`)

```go
type Lookup struct {
    mu          sync.RWMutex
    dmrToCall   map[uint32]string   // DMR ID → Callsign
    callToDMR   map[string]uint32   // Callsign → DMR ID
    lastRefresh time.Time
}

func NewLookup(filepath string) (*Lookup, error)
func (l *Lookup) LoadFromFile(filepath string) error
func (l *Lookup) GetCallsign(dmrID uint32) string
func (l *Lookup) GetDMRID(callsign string) (uint32, bool)
func (l *Lookup) Refresh() error
```

**Database Format** (DMRIds.dat):
```
1234567 W1ABC    John Doe      City, State
2345678 K2XYZ    Jane Smith    Town, Country
...
```

Parse to extract DMR ID and callsign columns.

#### 4. YSF2DMR Bridge (`pkg/ysf2dmr/bridge.go`)

```go
type Bridge struct {
    config      *config.YSF2DMRConfig
    logger      *logger.Logger

    // Network connections
    ysfServer   *network.Server      // Listen for YSF packets from reflector
    dmrNetwork  *dmr.Network         // Connect to DMR network

    // Audio conversion
    converter   *codec.Converter
    lookup      *dmr.Lookup

    // State tracking
    activeCall  *CallState
    mu          sync.RWMutex
}

type CallState struct {
    direction   Direction  // YSF→DMR or DMR→YSF
    srcCallsign string
    srcDMRID    uint32
    dstTG       uint32
    startTime   time.Time
    lastSeq     uint32
}

func NewBridge(cfg *config.YSF2DMRConfig, log *logger.Logger) (*Bridge, error)
func (b *Bridge) Start(ctx context.Context) error
func (b *Bridge) Stop() error

// Packet handlers
func (b *Bridge) handleYSFPacket(pkt *network.Packet)
func (b *Bridge) handleDMRPacket(pkt *dmr.Packet)

// Audio routing
func (b *Bridge) routeYSFToDMR(ysfData []byte) error
func (b *Bridge) routeDMRToYSF(dmrData []byte) error
```

**Bridge Operation Flow**:

1. **Initialization**:
   - Start YSF UDP server on configured port (different from main reflector)
   - Connect to DMR network and authenticate
   - Load DMR ID lookup database
   - Initialize audio converter

2. **YSF → DMR** (YSF repeater talks, send to DMR):
   - Receive YSFD packet from YSF side
   - Extract source callsign from YSF header
   - Lookup DMR ID for callsign (or use default/bridge ID)
   - Convert YSF AMBE frames to DMR format
   - Send DMR voice header/data/terminator to network
   - Track call state (start time, sequence, destination TG)

3. **DMR → YSF** (DMR network has traffic, send to YSF):
   - Receive DMR voice packet from network
   - Lookup callsign for source DMR ID
   - Convert DMR AMBE frames to YSF format
   - Construct YSFD packet with proper headers
   - Send to YSF reflector (which then distributes to repeaters)
   - Track call state and sequence numbers

4. **Call Termination**:
   - Detect end-of-transmission (YSF terminator or DMR terminator)
   - Send corresponding terminator to other side
   - Clear call state
   - Log call statistics (duration, packet count)

### Integration with YSF Nexus

#### Reflector Integration

Add YSF2DMR as an optional component:

```go
// pkg/reflector/reflector.go

type Reflector struct {
    // ... existing fields ...
    ysf2dmrBridge *ysf2dmr.Bridge  // Optional DMR bridge
}

func (r *Reflector) Start(ctx context.Context) error {
    // ... existing startup code ...

    // Start YSF2DMR bridge if configured
    if r.config.YSF2DMR.Enabled {
        r.logger.Info("Starting YSF2DMR bridge")
        if err := r.ysf2dmrBridge.Start(ctx); err != nil {
            return fmt.Errorf("failed to start YSF2DMR bridge: %w", err)
        }
    }

    // ... rest of startup ...
}

// Add method to forward packets from reflector to DMR bridge
func (r *Reflector) handleYSFDataPacket(pkt *network.Packet) {
    // ... existing packet handling ...

    // Forward to YSF2DMR bridge if enabled
    if r.ysf2dmrBridge != nil && r.config.YSF2DMR.Enabled {
        r.ysf2dmrBridge.OnYSFPacket(pkt)
    }
}
```

#### Web Dashboard Integration

Add YSF2DMR status to web dashboard:

```html
<!-- web/templates/dashboard.html -->
<div class="ysf2dmr-status">
  <h3>YSF2DMR Bridge</h3>
  <div class="bridge-info">
    <span class="status {{.YSF2DMR.State}}">{{.YSF2DMR.State}}</span>
    <span>DMR Network: {{.YSF2DMR.NetworkName}}</span>
    <span>Talkgroup: {{.YSF2DMR.CurrentTG}}</span>
    <span>Slot: {{.YSF2DMR.Slot}}</span>
  </div>

  {{if .YSF2DMR.ActiveCall}}
  <div class="active-call">
    <span>Active: {{.YSF2DMR.ActiveCall.Direction}}</span>
    <span>Source: {{.YSF2DMR.ActiveCall.Source}}</span>
    <span>Duration: {{.YSF2DMR.ActiveCall.Duration}}</span>
  </div>
  {{end}}

  <div class="statistics">
    <span>Calls Bridged: {{.YSF2DMR.Stats.TotalCalls}}</span>
    <span>YSF→DMR: {{.YSF2DMR.Stats.YSFToDMR}}</span>
    <span>DMR→YSF: {{.YSF2DMR.Stats.DMRToYSF}}</span>
  </div>
</div>
```

#### MQTT Event Publishing

Extend MQTT events to include YSF2DMR activity:

```json
{
  "type": "ysf2dmr_call_start",
  "direction": "ysf_to_dmr",
  "ysf_callsign": "W1ABC",
  "dmr_id": 1234567,
  "talkgroup": 91,
  "timestamp": "2024-01-15T10:30:00Z"
}

{
  "type": "ysf2dmr_call_end",
  "direction": "dmr_to_ysf",
  "dmr_id": 2345678,
  "dmr_callsign": "K2XYZ",
  "duration": "45s",
  "timestamp": "2024-01-15T10:31:00Z"
}
```

## Implementation Phases

### Phase 1: Core DMR Network Protocol (Week 1-2)
**Goal**: Establish basic DMR network connectivity and authentication

- [ ] `pkg/dmr/packets.go`: Define DMR packet structures
  - Packet types (RPTL, RPTK, RPTC, DMRD for voice)
  - Serialization/deserialization
  - Validation and checksum

- [ ] `pkg/dmr/network.go`: DMR network client
  - UDP connection management
  - Multi-step authentication (login → salt → password hash → config)
  - Keep-alive packet sender
  - Packet receive/transmit queues

- [ ] Unit tests for DMR network protocol
  - Mock DMR server for testing authentication
  - Packet parsing/generation tests

**Deliverable**: Standalone DMR network client that can connect, authenticate, and maintain connection to a DMR network server

### Phase 2: Audio Conversion Engine (Week 3-4)
**Goal**: Convert AMBE frames between YSF and DMR formats

- [ ] `pkg/codec/ambe.go`: AMBE frame utilities
  - Frame validation
  - Bit manipulation helpers
  - AMBE metadata extraction

- [ ] `pkg/codec/interleave.go`: Interleaving/deinterleaving
  - YSF interleave tables (from original C++ code)
  - DMR interleave tables
  - Scrambling/descrambling sequences

- [ ] `pkg/codec/golay.go`: Golay error correction
  - Golay(24,12) encoder
  - Golay(24,12) decoder with error correction

- [ ] `pkg/codec/conversion.go`: Main conversion logic
  - YSF → DMR frame conversion
  - DMR → YSF frame conversion
  - Frame alignment and buffering

- [ ] Comprehensive conversion tests
  - Test vectors from known good conversions
  - Round-trip conversion tests
  - Error handling tests

**Deliverable**: Audio conversion library that can translate YSF YSFD packets to DMR DMRD packets and vice versa

### Phase 3: DMR ID Lookup (Week 5)
**Goal**: Map between YSF callsigns and DMR IDs

- [ ] `pkg/dmr/lookup.go`: ID/Callsign database
  - Load DMRIds.dat file (standard format)
  - Bidirectional lookup (ID↔Callsign)
  - Periodic refresh capability
  - Thread-safe access

- [ ] Optional: Download DMR ID database automatically
  - Fetch from radioid.net or other sources
  - Update on startup or scheduled intervals

**Deliverable**: DMR ID lookup service with test database

### Phase 4: YSF2DMR Bridge (Week 6-7)
**Goal**: Integrate all components into functional bridge

- [ ] `pkg/ysf2dmr/bridge.go`: Main bridge coordinator
  - Initialize YSF and DMR connections
  - Manage bridge state (idle, YSF→DMR call, DMR→YSF call)
  - Call state tracking

- [ ] `pkg/ysf2dmr/ysf_handler.go`: YSF packet handling
  - Receive YSF packets from reflector
  - Identify voice traffic vs. control packets
  - Trigger YSF→DMR conversion and forwarding

- [ ] `pkg/ysf2dmr/dmr_handler.go`: DMR packet handling
  - Receive DMR packets from network
  - Filter for relevant talkgroup/slot
  - Trigger DMR→YSF conversion and forwarding

- [ ] Integration tests
  - Mock YSF reflector + mock DMR network
  - Simulate full call flows
  - Test edge cases (dropped packets, overlapping calls)

**Deliverable**: Functional YSF2DMR bridge that can route audio in both directions

### Phase 5: Reflector Integration (Week 8)
**Goal**: Integrate bridge into YSF Nexus main application

- [ ] Configuration support
  - Add YSF2DMR section to config.yaml
  - Validation of YSF2DMR settings

- [ ] Reflector initialization
  - Start YSF2DMR bridge alongside main reflector
  - Share event channel for MQTT/logging

- [ ] Packet routing
  - Forward relevant YSF packets from reflector to bridge
  - Inject DMR→YSF packets into reflector for distribution

- [ ] Graceful shutdown
  - Clean disconnect from DMR network
  - Send terminator packets for active calls

**Deliverable**: YSF Nexus with integrated YSF2DMR bridge, controlled by configuration

### Phase 6: Monitoring & Dashboard (Week 9)
**Goal**: Add visibility into YSF2DMR bridge operation

- [ ] Web dashboard updates
  - YSF2DMR status card (connected, talkgroup, slot)
  - Active call display (direction, source, duration)
  - Bridge statistics (total calls, packets, errors)

- [ ] MQTT event publishing
  - Call start/end events with direction and metadata
  - Bridge state changes (connected, disconnected, error)

- [ ] Prometheus metrics
  - YSF→DMR calls counter
  - DMR→YSF calls counter
  - Conversion errors gauge
  - DMR network latency histogram

**Deliverable**: Full monitoring and observability for YSF2DMR bridge

### Phase 7: Advanced Features (Week 10+)
**Goal**: Optional enhancements and optimization

- [ ] **Multiple DMR Networks**: Support connections to multiple DMR networks simultaneously
- [ ] **Dynamic TG Switching**: Allow users to change talkgroup via DTMF or WiresX commands
- [ ] **Private Call Support**: Enable DMR private calls in addition to group calls
- [ ] **XLX Reflector Support**: Connect to XLX reflectors with DMR modules
- [ ] **Audio Quality Tuning**: Gain adjustment, noise gating, voice activation
- [ ] **Call Recording**: Optional recording of bridged calls for logging/debugging
- [ ] **Web UI Controls**: Start/stop bridge, change talkgroup from dashboard

## Testing Strategy

### Unit Tests
- **DMR Packet Parsing**: Verify all packet types serialize/deserialize correctly
- **Audio Conversion**: Test YSF↔DMR conversion with known test vectors
- **Golay Codes**: Validate error correction with intentional bit errors
- **ID Lookup**: Test database loading and bidirectional lookup

### Integration Tests
- **Mock DMR Network**: Create test server that simulates DMR network authentication
- **Mock YSF Reflector**: Inject YSF packets and verify DMR output
- **Round-Trip Audio**: Send YSF packet, convert to DMR, convert back, verify integrity

### End-to-End Tests
- **Live YSF→DMR**: Connect real YSF repeater, bridge to test DMR network, verify audio
- **Live DMR→YSF**: Transmit on DMR network, verify YSF repeater receives audio
- **Load Testing**: Simulate multiple simultaneous calls, verify no packet loss

### Compliance Testing
- **DMR Network Compatibility**: Test with BrandMeister, DMRPlus, TGIF, others
- **YSF Protocol Compliance**: Verify proper YSF packet structure per protocol spec
- **Audio Quality**: Subjective listening tests to assess conversion quality

## Configuration Example

```yaml
# configs/config.yaml

# ... existing YSF Nexus configuration ...

# YSF2DMR Bridge Configuration
ysf2dmr:
  enabled: true

  ysf:
    callsign: "YSF2DMR"
    local_address: "0.0.0.0"
    local_port: 42001        # Separate from main reflector port
    hang_time: 5s

  dmr:
    enabled: true
    id: 3106969              # Bridge DMR ID (must be registered)
    network: "BrandMeister"
    address: "bm.example.com"
    port: 62030
    password: "yourpassword"

    # Routing
    startup_tg: 91           # North America talkgroup
    slot: 2
    color_code: 1

    # Keep-alive and timeouts
    ping_interval: 10s
    auth_timeout: 30s

  lookup:
    enabled: true
    dmr_id_file: "/etc/ysf-nexus/DMRIds.dat"
    auto_download: true
    download_url: "https://radioid.net/static/users.csv"
    refresh_interval: 24h

  audio:
    gain: 1.0
    vox_enabled: false

  logging:
    log_dmr_packets: false
    log_conversions: true
    log_level: "info"
```

## Documentation Requirements

### User Documentation
- **Installation Guide**: How to configure YSF2DMR bridge
- **DMR ID Registration**: How to obtain a DMR ID for the bridge
- **Network Setup**: Connecting to BrandMeister, DMRPlus, TGIF
- **Troubleshooting**: Common issues and solutions

### Developer Documentation
- **Architecture Overview**: Component diagram and data flow
- **Protocol Reference**: DMR network protocol details
- **Audio Conversion**: Explanation of AMBE transcoding process
- **API Documentation**: GoDoc for all public interfaces

## Success Criteria

1. ✅ **DMR Network Connectivity**: Bridge can authenticate and maintain connection to major DMR networks (BrandMeister, DMRPlus)
2. ✅ **Bidirectional Audio**: Voice traffic flows both YSF→DMR and DMR→YSF with acceptable quality
3. ✅ **Call Handling**: Properly handle call start, in-progress, and termination states
4. ✅ **Callsign Mapping**: Correct translation between YSF callsigns and DMR IDs
5. ✅ **Stability**: Bridge runs continuously for 24+ hours without crashes or memory leaks
6. ✅ **Monitoring**: Real-time status visible in web dashboard and MQTT events
7. ✅ **Audio Quality**: Subjective audio quality rated "good" or better by test users
8. ✅ **Performance**: Bridge adds <50ms latency to audio path

## Potential Challenges & Mitigations

### Challenge 1: AMBE Codec Licensing
**Issue**: AMBE codec is proprietary and may have licensing restrictions

**Mitigation**:
- We're not implementing AMBE decode/encode, only re-framing existing AMBE bits
- Both YSF and DMR use AMBE+2, so conversion is bit manipulation, not codec implementation
- Similar to how the original YSF2DMR works - no codec library needed
- If needed, can interface with hardware AMBE chips (USB dongles)

### Challenge 2: DMR Network Protocol Variations
**Issue**: Different DMR networks may have slightly different protocols

**Mitigation**:
- Start with BrandMeister (most popular, well-documented)
- Make protocol handlers pluggable for network-specific quirks
- Test with multiple networks (BrandMeister, DMRPlus, TGIF)
- Community feedback to add support for additional networks

### Challenge 3: Audio Quality Degradation
**Issue**: Multiple conversions may degrade audio quality

**Mitigation**:
- Minimize conversions - only reframe, don't decode/encode
- Implement proper error correction (Golay codes)
- Add optional audio processing (gain, filtering)
- Extensive listening tests to tune conversion parameters

### Challenge 4: Call Collision Handling
**Issue**: Simultaneous YSF and DMR calls may conflict

**Mitigation**:
- Implement call priority (first-in wins)
- Add configurable policy (YSF priority, DMR priority, or FIFO)
- Queue conflicting calls or reject with proper signaling
- Monitor for conflicts and log/alert

### Challenge 5: Packet Loss and Network Issues
**Issue**: UDP nature of both protocols means packet loss is possible

**Mitigation**:
- Implement FEC and error correction where possible
- Add packet buffering and jitter handling
- Monitor packet loss rates and alert on high loss
- Automatic reconnection for DMR network failures

## Timeline Summary

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| 1. Core DMR Protocol | 2 weeks | DMR network client |
| 2. Audio Conversion | 2 weeks | YSF↔DMR transcoding |
| 3. DMR ID Lookup | 1 week | ID database service |
| 4. YSF2DMR Bridge | 2 weeks | Functional bridge |
| 5. Reflector Integration | 1 week | Integrated YSF Nexus |
| 6. Monitoring & Dashboard | 1 week | Full observability |
| 7. Advanced Features | Ongoing | Optional enhancements |
| **Total** | **9-10 weeks** | **Production-ready YSF2DMR** |

## Dependencies & Prerequisites

### Go Libraries
- Existing YSF Nexus dependencies (viper, zap, gorilla)
- No new external dependencies for core functionality
- Optional: DMR ID database parser (can use stdlib `encoding/csv`)

### External Resources
- DMR ID database file (DMRIds.dat or radioid.net CSV)
- DMR network credentials (DMR ID and password)
- Access to test DMR network (BrandMeister allows free registration)

### Knowledge Requirements
- Understanding of YSF protocol (already have)
- DMR network protocol (study from C++ code and docs)
- AMBE codec framing (reference material available)
- Digital signal processing basics (for audio conversion)

## References

### Primary References
- [nostar/MMDVM_CM/YSF2DMR](https://github.com/nostar/MMDVM_CM/tree/master/YSF2DMR) - Original C++ implementation
- [G4KLX MMDVM Project](https://github.com/g4klx/MMDVM) - Base MMDVM framework
- [BrandMeister Network](https://brandmeister.network/) - Major DMR network with documentation
- [RadioID.net](https://radioid.net/) - DMR ID database

### Technical Specifications
- YSF Protocol: Available in existing YSF reflector documentation
- DMR ETSI Standard (ETSI TS 102 361) - Official DMR specification
- AMBE+2 Vocoder: DVSI technical documentation
- Golay Code: Standard error correction algorithm documentation

### Community Resources
- Amateur Radio Digital Communications (ARDC) forums
- Reddit r/amateurradio DMR discussions
- BrandMeister Discord for network-specific questions

---

## Next Steps

1. **Stakeholder Review**: Get feedback on this plan from project maintainers and users
2. **Prioritization**: Confirm if YSF2DMR is high priority vs. other features
3. **Resource Allocation**: Determine if this is solo effort or team collaboration
4. **Prototype**: Start with Phase 1 (DMR network protocol) as proof-of-concept
5. **Community Engagement**: Reach out to DMR community for testing volunteers

**Ready to proceed when approved!** 🚀
