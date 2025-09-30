# Enhanced Bridge System Integration Guide

## Overview

The YSF Nexus enhanced bridge system provides robust inter-reflector connectivity with permanent connections, scheduled bridging, automatic retry logic, and missed schedule recovery. This guide explains how the bridge system integrates with the main reflector components.

## Architecture

### Key Components

1. **Bridge Manager** (`pkg/bridge/manager.go`)
   - Central orchestrator for all bridge connections
   - Handles cron-based scheduling and missed schedule recovery
   - Manages bridge lifecycle and connection state tracking

2. **Individual Bridge** (`pkg/bridge/bridge.go`) 
   - Manages a single connection to a remote reflector
   - Implements exponential backoff retry logic
   - Handles connection health monitoring and keep-alive packets

3. **Network Interface** (`pkg/bridge/network.go`)
   - Defines interface for packet sending and address resolution
   - Integrates with main YSF network server for packet routing

## Integration Points

### 1. Configuration Loading

The bridge system integrates with the existing configuration system:

```go
// In main application initialization
config, err := config.Load("config.yaml")
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

// Create bridge manager with loaded bridge configs
bridgeManager := bridge.NewManager(config.Bridges, networkServer, logger)
```

### 2. Network Server Integration

The bridge system requires a NetworkServer interface implementation:

```go
// Example implementation using existing YSF server
type YSFNetworkBridge struct {
    server *network.Server
}

func (y *YSFNetworkBridge) SendPacket(data []byte, addr *net.UDPAddr) error {
    return y.server.SendToAddress(data, addr)
}

func (y *YSFNetworkBridge) GetListenAddress() *net.UDPAddr {
    return y.server.GetListenAddr()
}
```

### 3. Packet Forwarding Integration

Bridge connections integrate with the main packet routing system:

```go
// In main packet handler
func (s *Server) handlePacket(data []byte, fromAddr *net.UDPAddr) {
    // Check if packet came from a bridge connection
    if s.bridgeManager != nil {
        s.bridgeManager.HandleIncomingPacket(data, fromAddr)
    }
    
    // Continue with normal packet processing
    // Forward to local repeaters, process YSF protocol, etc.
}
```

### 4. Startup Integration

```go
func main() {
    // ... existing initialization ...
    
    // Start bridge manager after network server is ready
    if err := bridgeManager.Start(); err != nil {
        logger.Fatal("Failed to start bridge manager", logger.Error(err))
    }
    defer bridgeManager.Stop()
    
    // ... run main server loop ...
}
```

## Bridge System Features

### Permanent Bridges

- **Purpose**: Maintain constant inter-reflector connectivity
- **Use Cases**: Primary network backbone, redundant links, always-on connections
- **Configuration**: Set `permanent: true`, ignore schedule/duration
- **Behavior**: 
  - Start immediately when bridge manager starts
  - Automatic reconnection with exponential backoff
  - Continuous health monitoring
  - Infinite retries by default (configurable)

### Scheduled Bridges

- **Purpose**: Time-based connectivity for specific events or schedules
- **Use Cases**: Scheduled nets, event bridges, time-limited connections
- **Configuration**: Set `permanent: false`, provide `schedule` and `duration`
- **Behavior**:
  - Start only during scheduled windows
  - Missed schedule recovery on system startup
  - Limited connection duration
  - Configurable retry limits

### Missed Schedule Recovery

The system automatically recovers from missed schedules:

```go
// Recovery logic checks if current time falls within any scheduled window
func (m *Manager) shouldStartNow(config config.BridgeConfig) bool {
    // Parse schedule and check if we're within a missed window
    // If system was down during scheduled start time, recover the connection
    return withinScheduleWindow && shouldBeRunning
}
```

### Health Monitoring

Each bridge performs continuous health monitoring:

- **Keep-alive packets**: Sent every 30 seconds to maintain connection
- **Health checks**: Configurable interval ping packets  
- **Connection timeouts**: Auto-disconnect if no traffic for 2x health interval
- **Statistics tracking**: Packets/bytes TX/RX, connection uptime, error counts

### Exponential Backoff Retry

Implements robust retry logic with exponential backoff:

```go
// Retry delay calculation with jitter to prevent thundering herd
delay := baseDelay * 2^retryCount
maxDelay := 10 * time.Minute  // Cap maximum delay
jitter := ±25% of delay       // Add randomization
```

## Usage Examples

### Basic Permanent Bridge

```yaml
bridges:
  - name: "main-backbone"
    host: "primary.ysfreflector.org"  
    port: 4200
    enabled: true
    permanent: true
    max_retries: 0        # Infinite retries
    retry_delay: "30s"    # Start with 30-second delays
    health_check: "60s"   # Health check every minute
```

### Evening Net Schedule

```yaml
bridges:
  - name: "evening-net"
    host: "net.example.com"
    port: 4200
    enabled: true
    permanent: false
    schedule: "0 0 19 * * *"    # Daily at 7 PM
    duration: "2h"              # 2-hour net
    max_retries: 5
    retry_delay: "10s" 
    health_check: "30s"
```

### Weekend Event Bridge

```yaml
bridges:
  - name: "weekend-event" 
    host: "event.ysf.org"
    port: 4200
    enabled: true
    permanent: false
    schedule: "0 0 10 * * 6,0"  # Weekends at 10 AM
    duration: "6h"              # 6-hour event
    max_retries: 3
    retry_delay: "15s"
    health_check: "45s"
```

## Monitoring and Status

### Bridge Status Information

Each bridge provides comprehensive status:

```go
type BridgeStatus struct {
    Name           string      `json:"name"`
    State          BridgeState `json:"state"`          // connected, connecting, failed, etc.
    ConnectedAt    *time.Time  `json:"connected_at"`
    DisconnectedAt *time.Time  `json:"disconnected_at"`
    NextSchedule   *time.Time  `json:"next_schedule"`
    RetryCount     int         `json:"retry_count"`
    LastError      string      `json:"last_error"`
    PacketsRx      uint64      `json:"packets_rx"`
    PacketsTx      uint64      `json:"packets_tx"`
    BytesRx        uint64      `json:"bytes_rx"`
    BytesTx        uint64      `json:"bytes_tx"`
}
```

### Web Dashboard Integration

Bridge status can be integrated into the existing web dashboard:

```javascript
// Fetch bridge status
fetch('/api/bridges/status')
  .then(response => response.json())
  .then(bridges => {
    // Display bridge connection status, statistics, schedules
    updateBridgeDisplay(bridges);
  });
```

### MQTT Integration

Bridge events can be published to MQTT for external monitoring:

```go
// Example MQTT bridge status publishing
func (m *Manager) publishBridgeStatus(bridgeName string, status BridgeStatus) {
    topic := fmt.Sprintf("ysf/bridges/%s/status", bridgeName)
    payload, _ := json.Marshal(status)
    m.mqttClient.Publish(topic, payload)
}
```

## Best Practices

### Bridge Configuration

1. **Permanent Bridges**: Use for critical backbone connections
   - Set `max_retries: 0` for infinite retry attempts
   - Use reasonable `retry_delay` (30-60 seconds)
   - Enable health checking every 60-120 seconds

2. **Scheduled Bridges**: Use for time-limited events  
   - Set appropriate `max_retries` for window duration
   - Use shorter `retry_delay` (5-15 seconds) for quick recovery
   - More frequent health checks (15-30 seconds)

3. **Network Considerations**:
   - Ensure firewall rules allow bidirectional UDP traffic
   - Consider network latency when setting health check intervals
   - Monitor bridge statistics to optimize retry settings

### Error Handling

1. **Connection Failures**: Logged with context and retry information
2. **Configuration Errors**: Validated on startup with clear error messages  
3. **Schedule Conflicts**: Detected and reported during configuration loading
4. **Resource Limits**: Bridge count and connection limits enforced

### Performance Considerations  

1. **CPU Usage**: Cron scheduler and health checks are lightweight
2. **Memory Usage**: Each bridge maintains minimal state and statistics
3. **Network Usage**: Keep-alive and health check packets are small (< 100 bytes)
4. **Scaling**: System supports dozens of concurrent bridge connections

## Troubleshooting

### Common Issues

1. **Connection Failures**
   - Check DNS resolution for bridge hostnames
   - Verify firewall rules and port accessibility
   - Monitor retry counts and error messages

2. **Schedule Issues**  
   - Validate cron expressions using online tools
   - Check system timezone configuration
   - Review missed schedule recovery logs

3. **Performance Problems**
   - Monitor bridge statistics for packet loss
   - Adjust health check intervals based on network conditions
   - Review retry delay settings for network latency

### Debugging Tools

```bash
# Check bridge status via logs
grep "bridge" /var/log/ysf-nexus/server.log | tail -20

# Test network connectivity 
nc -u bridge-host.example.com 4200

# Validate cron expressions
# Use online cron expression validators
```

This enhanced bridge system provides a robust, scalable solution for YSF reflector interconnectivity with enterprise-grade reliability and monitoring capabilities.