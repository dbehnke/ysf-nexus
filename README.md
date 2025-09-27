# YSF Nexus

A modern, high-performance YSF (Yaesu System Fusion) reflector written in Go with web dashboard, MQTT integration, and bridge capabilities.

## 🚀 Features

- **High Concurrency**: Handle 200+ simultaneous connections using Go's goroutines
- **Web Dashboard**: Real-time monitoring with connection status, talk logs, and system metrics
- **Bridge System**: Automated connections to other YSF reflectors with cron-like scheduling
- **MQTT Integration**: Real-time events for connect/disconnect/talk actions
- **Single Binary**: All features packaged in one executable
- **Docker Ready**: Easy deployment with containerization support
- **Comprehensive Testing**: Unit and integration tests with high coverage

## 🏗️ Architecture

YSF Nexus leverages Go's excellent concurrency model to efficiently handle I/O-heavy amateur radio digital voice communication:

- **UDP Network Layer**: Efficient packet handling with worker pools
- **Thread-Safe Repeater Management**: Concurrent connection handling
- **Real-Time Web Interface**: WebSocket-based live updates
- **Event-Driven MQTT**: Non-blocking external system integration
- **Scheduled Bridging**: Background goroutines for inter-reflector connections

## 📋 YSF Protocol Support

- **YSFP**: Poll/Registration packets
- **YSFU**: Unlink packets
- **YSFD**: Data transmission (155 bytes)
- **YSFS**: Status packets with system information
- **IPv4/IPv6**: Dual-stack networking support

## 🔧 Quick Start

### Prerequisites
- Go 1.21 or later
- Docker (optional)

### Installation

```bash
# Clone the repository
git clone https://github.com/dbehnke/ysf-nexus.git
cd ysf-nexus

# Build the binary
make build

# Run with default configuration
./bin/ysf-nexus
```

### Docker Deployment

```bash
# Build Docker image
docker build -t ysf-nexus .

# Run container
docker run -p 42000:42000/udp -p 8080:8080 ysf-nexus
```

## ⚙️ Configuration

Create a `config.yaml` file:

```yaml
server:
  host: "0.0.0.0"
  port: 42000
  timeout: "5m"
  max_connections: 200

web:
  enabled: true
  port: 8080
  auth_required: false

bridges:
  - name: "YSF001"
    host: "ysf001.example.com"
    port: 42000
    schedule: "0 */6 * * *"  # Every 6 hours
    duration: "1h"

mqtt:
  enabled: true
  broker: "tcp://localhost:1883"
  topic_prefix: "ysf/reflector"
  client_id: "ysf-nexus"

logging:
  level: "info"
  format: "json"
```

## 📊 Web Dashboard

Access the web dashboard at `http://localhost:8080` to view:

- **Live Connections**: Real-time repeater status and activity
- **Talk Log**: History of transmissions with callsigns and duration
- **System Metrics**: Connection counts, packet rates, uptime statistics
- **Bridge Status**: Active bridge connections and schedules
- **Configuration**: Web-based settings management

## 🌉 Bridge System

YSF Nexus can automatically connect to other YSF reflectors on a schedule:

```yaml
bridges:
  - name: "Primary Hub"
    host: "ysf-main.example.com"
    port: 42000
    schedule: "0 8 * * *"     # Daily at 8 AM
    duration: "2h"            # Stay connected for 2 hours

  - name: "Regional Net"
    host: "regional.ysf.net"
    port: 42000
    schedule: "0 20 * * 6"    # Saturdays at 8 PM
    duration: "1h30m"         # 1.5 hour duration
```

## 📡 MQTT Integration

Real-time events are published to MQTT topics:

```json
// Connection events
{
  "type": "connect",
  "callsign": "W1ABC",
  "timestamp": "2024-01-15T10:30:00Z"
}

// Talk events
{
  "type": "talk_start",
  "callsign": "W1ABC",
  "timestamp": "2024-01-15T10:31:00Z"
}

{
  "type": "talk_end",
  "callsign": "W1ABC",
  "timestamp": "2024-01-15T10:31:30Z",
  "duration": "30s"
}
```

## 🧪 Development

### Prerequisites
- Go 1.21+
- Make

### Building
```bash
make build          # Build binary
make test           # Run tests
make test-coverage  # Generate coverage report
make lint           # Run linter
make docker         # Build Docker image
```

### Testing
```bash
make test                    # Unit tests
make test-integration       # Integration tests
make test-load              # Load testing
```

## 📈 Performance

YSF Nexus is designed for high performance:

- **Connections**: 200+ simultaneous repeaters
- **Latency**: <10ms packet routing
- **Memory Usage**: <100MB under full load
- **CPU Usage**: <5% on modern hardware

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔗 Related Projects

- [Original C++ YSF Reflector](https://github.com/nostar/DVReflectors/tree/main/YSFReflector) - The original implementation
- [YSF Protocol Documentation](https://github.com/g4klx/YSFClients) - Protocol specifications

## 📞 Support

- 📧 Issues: [GitHub Issues](https://github.com/dbehnke/ysf-nexus/issues)
- 💬 Discussions: [GitHub Discussions](https://github.com/dbehnke/ysf-nexus/discussions)

## 🙏 Acknowledgments

- Thanks to the original [DVReflectors](https://github.com/nostar/DVReflectors) project
- Amateur radio community for YSF protocol development
- Go community for excellent networking libraries

---

**73!** 📻