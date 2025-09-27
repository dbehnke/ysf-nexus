# YSF Nexus Docker Setup

This directory contains Docker configurations for running YSF Nexus in containerized environments.

## Quick Start (Simple Setup)

For a basic YSF reflector with web dashboard:

```bash
# Copy and customize the configuration
cp config.yaml config.yaml.local
# Edit config.yaml.local with your settings

# Start the reflector
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the reflector
docker-compose down
```

Access the web dashboard at: http://localhost:8080

## Configuration Files

### Simple Setup
- `docker-compose.yml` - Basic setup with just YSF Nexus
- `config.yaml` - Simple configuration template

### Full Monitoring Setup
- `docker-compose.full.yml` - Complete setup with monitoring stack
- Includes: MQTT broker, Prometheus metrics, Grafana dashboards

## Port Mappings

### Simple Setup
- `42000/udp` - YSF protocol (repeater connections)
- `8080/tcp` - Web dashboard

### Full Setup (Additional)
- `1883/tcp` - MQTT broker
- `9001/tcp` - MQTT WebSocket
- `9090/tcp` - Prometheus metrics
- `3000/tcp` - Grafana dashboards

## Configuration Options

### Basic Settings
```yaml
server:
  name: "Your Reflector Name"     # Max 16 characters
  description: "Your Description" # Max 14 characters
  max_connections: 200
  timeout: "5m"

web:
  enabled: true
  auth_required: false  # Set to true for authentication
  # username: "admin"   # Required if auth_required: true
  # password: "secure"  # Required if auth_required: true
```

### Authentication
To enable web dashboard authentication:

1. Edit your config file:
```yaml
web:
  auth_required: true
  username: "admin"
  password: "your-secure-password"  # Change this!
```

2. Restart the container:
```bash
docker-compose restart
```

### Logging
```yaml
logging:
  level: "info"                    # debug, info, warn, error
  format: "text"                   # text or json
  file: "/app/logs/ysf-nexus.log" # Optional file logging
```

### Blocklist
```yaml
blocklist:
  enabled: true
  callsigns:
    - "BLOCKED1"
    - "SPAM123"
```

## Full Monitoring Stack

For advanced monitoring with MQTT, Prometheus, and Grafana:

```bash
# Use the full compose file
docker-compose -f docker-compose.full.yml up -d
```

This provides:
- **MQTT Broker**: Real-time event streaming
- **Prometheus**: Metrics collection and storage
- **Grafana**: Visual dashboards and alerts

### Grafana Access
- URL: http://localhost:3000
- Default credentials: admin/admin (change on first login)

### MQTT Topics
When MQTT is enabled, events are published to:
- `ysf/reflector/connect` - Repeater connections
- `ysf/reflector/disconnect` - Repeater disconnections
- `ysf/reflector/talk_start` - Talk events begin
- `ysf/reflector/talk_end` - Talk events end

## Docker Commands

### Basic Operations
```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs -f ysf-nexus

# Restart services
docker-compose restart

# Stop services
docker-compose down

# Update and rebuild
docker-compose build --no-cache
docker-compose up -d
```

### Maintenance
```bash
# View container status
docker-compose ps

# Execute commands in container
docker-compose exec ysf-nexus sh

# View container resource usage
docker stats ysf-nexus

# Clean up old images
docker image prune
```

## Persistent Data

Data is stored in Docker volumes:
- `ysf_logs` - Application logs
- `mosquitto_data` - MQTT broker data (full setup)
- `prometheus_data` - Metrics data (full setup)
- `grafana_data` - Dashboard configs (full setup)

## Firewall Configuration

Ensure these ports are open on your host:
- `42000/udp` - **Required** for YSF repeater connections
- `8080/tcp` - Optional, for web dashboard access

## Health Checks

The container includes automatic health monitoring:
- Checks web API every 30 seconds
- Marks container unhealthy after 3 failed checks
- Useful for orchestration platforms (Docker Swarm, Kubernetes)

## Environment Variables

You can override configuration with environment variables:

```bash
# In docker-compose.yml
environment:
  - YSF_SERVER_HOST=0.0.0.0
  - YSF_SERVER_PORT=42000
  - YSF_WEB_PORT=8080
  - TZ=America/New_York
```

## Production Deployment

For production use:

1. **Security**: Enable authentication and use strong passwords
2. **Reverse Proxy**: Use nginx/traefik for HTTPS termination
3. **Monitoring**: Enable the full monitoring stack
4. **Backups**: Backup Docker volumes regularly
5. **Updates**: Keep the container image updated

Example nginx configuration:
```nginx
server {
    listen 443 ssl;
    server_name your-reflector.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Troubleshooting

### Container Won't Start
```bash
# Check logs for errors
docker-compose logs ysf-nexus

# Verify configuration syntax
docker-compose config
```

### No Repeater Connections
- Verify UDP port 42000 is accessible
- Check firewall/NAT configuration
- Review reflector logs for connection attempts

### Web Dashboard Not Accessible
- Confirm port 8080 is not blocked
- Check if authentication is required
- Verify container is healthy: `docker-compose ps`

### Performance Issues
```bash
# Monitor resource usage
docker stats ysf-nexus

# Check container health
docker-compose exec ysf-nexus ./ysf-nexus --version
```

## Support

For issues and questions:
- Check the logs: `docker-compose logs -f`
- Review configuration: `docker-compose config`
- GitHub Issues: [YSF Nexus Repository](https://github.com/dbehnke/ysf-nexus)