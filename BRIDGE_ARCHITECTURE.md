# Bridge Architecture - Unified YSF and DMR Bridges

## Overview

YSF Nexus supports two types of bridges, managed uniformly through a single configuration section:

1. **YSF Bridges**: Connect to external YSF reflectors
2. **DMR Bridges**: Bridge YSF traffic to DMR networks (YSF2DMR functionality)

Both bridge types support:
- Scheduled operation (cron-based timing)
- Permanent operation (always connected)
- Duration limits for scheduled bridges
- Health checking and automatic retry
- Unified status monitoring in web dashboard

## Configuration Structure

Bridges are configured in the unified `bridges:` section with a `type` discriminator:

```yaml
bridges:
  # YSF Bridge Example
  - name: "YSF001"
    type: ysf                    # Bridge type: "ysf" or "dmr"
    host: "ysf001.example.com"
    port: 42000
    schedule: "0 */6 * * *"     # Every 6 hours
    duration: "1h"
    enabled: true

  # DMR Bridge Example
  - name: "BrandMeister TG91"
    type: dmr
    schedule: "0 20 * * 0"       # Sundays at 8 PM
    duration: "2h"
    enabled: true
    dmr:                         # DMR-specific configuration
      id: 1234567
      network: "BrandMeister"
      address: "3100.r2.bm.dmrlog.net"
      port: 62031
      password: "secret"
      talk_group: 91
      slot: 2
      color_code: 1
```

## Code Architecture

### Configuration Layer (`pkg/config/`)

**BridgeConfig struct** - Holds configuration for all bridge types:
- Common fields: `name`, `type`, `schedule`, `duration`, `enabled`, `permanent`
- YSF-specific: `host`, `port`
- DMR-specific: `dmr` (nested `DMRBridgeConfig`)

**DMRBridgeConfig struct** - DMR network parameters:
- Network connection: `address`, `port`, `password`
- DMR identity: `id`, `talk_group`, `slot`, `color_code`
- Optional metadata: location, description, frequencies

### Bridge Manager (`pkg/bridge/`)

**Current Implementation** (YSF-only):
- `Manager` - Manages YSF bridge lifecycle
- `Bridge` - Implements YSF bridge protocol
- Handles scheduling, health checking, retry logic

**Required Changes**:
1. Update `Manager.setupBridge()` to check `config.Type`
2. For `type: ysf` → create existing `Bridge` instance
3. For `type: dmr` → create `ysf2dmr.Bridge` adapter instance
4. Both types implement common status/lifecycle interface

### YSF2DMR Bridge (`pkg/ysf2dmr/`)

**Existing Implementation**:
- Complete DMR network client with authentication
- YSF↔DMR audio conversion engine
- DMR ID lookup system
- Call state management

**Integration Approach**:
Create an **adapter** that wraps `ysf2dmr.Bridge` to match the bridge manager's interface:

```go
// pkg/bridge/dmr_adapter.go
type DMRBridgeAdapter struct {
    config config.BridgeConfig
    bridge *ysf2dmr.Bridge
    // ... lifecycle management
}

func (a *DMRBridgeAdapter) RunPermanent(ctx context.Context) {
    // Delegate to ysf2dmr.Bridge
}

func (a *DMRBridgeAdapter) RunScheduled(ctx context.Context, duration time.Duration) {
    // Run with timeout
}

func (a *DMRBridgeAdapter) GetStatus() BridgeStatus {
    // Convert ysf2dmr status to bridge.BridgeStatus format
}
```

### Status Interface

Both YSF and DMR bridges return unified `BridgeStatus`:

```go
type BridgeStatus struct {
    Name       string         `json:"name"`
    Type       BridgeType     `json:"type"`        // "ysf" or "dmr"
    State      BridgeState    `json:"state"`
    // ... common fields ...
    Metadata   map[string]interface{} `json:"metadata,omitempty"`  // DMR-specific info
}
```

DMR bridges populate `Metadata` with:
- `dmr_network`: Network name (e.g., "BrandMeister")
- `talk_group`: Active talk group
- `dmr_id`: DMR ID in use
- `active_call`: Current call information

## Web Dashboard Integration

### Bridges View

Currently shows YSF bridges only. Needs updates:

1. **Type Badge**: Display "YSF" or "DMR" badge for each bridge
2. **Connection Info**:
   - YSF: Show `host:port`
   - DMR: Show `network TG###`
3. **Active Calls**: For DMR bridges, show YSF callsign ↔ DMR ID

### API Response

`GET /api/bridges` returns unified bridge list:

```json
{
  "bridges": {
    "YSF001": {
      "name": "YSF001",
      "type": "ysf",
      "state": "connected",
      // ... YSF status
    },
    "BrandMeister TG91": {
      "name": "BrandMeister TG91",
      "type": "dmr",
      "state": "scheduled",
      "next_schedule": "2025-09-30T20:00:00Z",
      "metadata": {
        "dmr_network": "BrandMeister",
        "talk_group": 91,
        "dmr_id": 1234567
      }
    }
  }
}
```

## Implementation Checklist

### Backend
- [x] Update `BridgeConfig` to include `type` field
- [x] Add `DMRBridgeConfig` struct for DMR parameters
- [x] Update config.example.yaml with mixed bridge examples
- [ ] Create `DMRBridgeAdapter` in `pkg/bridge/dmr_adapter.go`
- [ ] Update `Manager.setupBridge()` to instantiate correct bridge type
- [ ] Update `BridgeStatus` to include `Type` and `Metadata` fields
- [ ] Update existing YSF bridge's `GetStatus()` to set `Type: "ysf"`
- [ ] Implement DMR adapter's `GetStatus()` with metadata

### Frontend
- [ ] Update Bridges.vue to display bridge type badges
- [ ] Update bridge connection info display (host vs network/TG)
- [ ] Add DMR-specific metadata display (talk group, DMR ID)
- [ ] Update Dashboard.vue bridge cards to show type

### Testing
- [ ] Test YSF bridge configuration (backward compatibility)
- [ ] Test DMR bridge configuration
- [ ] Test mixed YSF + DMR bridge configuration
- [ ] Verify scheduling works for both types
- [ ] Verify permanent bridges work for both types
- [ ] Test web UI with mixed bridge types

### Documentation
- [x] Create BRIDGE_ARCHITECTURE.md (this file)
- [ ] Update README.md with bridge examples
- [ ] Update YSF2DMR.md with new integration approach

## Migration Notes

### Backward Compatibility

Existing YSF bridge configurations without `type` field will default to `type: ysf`:

```yaml
# Old config (still works)
bridges:
  - name: "YSF001"
    host: "ysf001.example.com"
    port: 42000
    enabled: true

# Equivalent to:
bridges:
  - name: "YSF001"
    type: ysf  # Auto-defaulted
    host: "ysf001.example.com"
    port: 42000
    enabled: true
```

### Removing YSF2DMR Top-Level Section

Once DMR bridges are integrated into the `bridges:` section, the top-level `ysf2dmr:` configuration section can be deprecated. Users would migrate from:

```yaml
# Old approach
ysf2dmr:
  enabled: true
  dmr:
    id: 1234567
    # ... config
```

To:

```yaml
# New approach
bridges:
  - name: "My DMR Bridge"
    type: dmr
    enabled: true
    dmr:
      id: 1234567
      # ... config
```

This provides better consistency and allows multiple DMR bridges to different networks/talk groups.

## Benefits

1. **Unified Management**: All bridges (YSF and DMR) managed through one system
2. **Flexible Scheduling**: DMR bridges can be scheduled just like YSF bridges
3. **Multiple DMR Bridges**: Support connections to multiple DMR networks/talk groups
4. **Consistent UX**: Same monitoring, status, and configuration patterns
5. **Easier to Extend**: Future bridge types (P25, NXDN) follow same pattern

## Next Steps

1. Implement `DMRBridgeAdapter` wrapper
2. Update bridge manager's setup logic
3. Update web UI for bridge type display
4. Test end-to-end with mixed configuration
5. Document migration path for existing YSF2DMR users
