# Annet-Oil Deployment Guide

This guide covers the deployment of both the gnetcli server component and the MCP (Model Context Protocol) server for AI agent integration.

## Table of Contents

1. [gnetcli Server Deployment](#gnetcli-server-deployment)
2. [MCP Server Setup](#mcp-server-setup)
3. [API Endpoints](#api-endpoints)
4. [Security Considerations](#security-considerations)

## gnetcli Server Deployment

### Prerequisites

- Linux system with systemd
- Go 1.21+ installed
- Root access for service installation
- Network access to target devices

### Installation Steps

1. **Build the gnetcli server**:
```bash
cd /path/to/annet-oil
go build -o /root/go/bin/gnetcli_server ./cmd/gnetcli_server
```

2. **Configure the service**:
```bash
cd configs/systemd
# Edit gnetcli.env with your settings
vim gnetcli.env
```

3. **Install as systemd service**:
```bash
sudo ./install.sh
```

4. **Start the service**:
```bash
sudo systemctl start gnetcli
sudo systemctl status gnetcli
```

### Configuration

Edit `/etc/gnetcli/gnetcli.env`:

```bash
# gnetcli_server settings
GNETCLI_GRPC_PORT=443              # gRPC port
GNETCLI_HTTP_PORT=50052            # HTTP port for health checks

# Basic auth for gnetcli server clients
GNETCLI_LOGIN=your_username
GNETCLI_PASSWORD=your_password

# Default device credentials (optional)
DEVICE_LOGIN=device_username
DEVICE_PASSWORD=device_password
```

### Service Management

```bash
# Start service
sudo systemctl start gnetcli

# Stop service
sudo systemctl stop gnetcli

# Restart service
sudo systemctl restart gnetcli

# View logs
sudo journalctl -u gnetcli -f

# Check status
sudo systemctl status gnetcli

# Enable auto-start on boot
sudo systemctl enable gnetcli

# Disable auto-start
sudo systemctl disable gnetcli
```

### Uninstallation

```bash
cd configs/systemd
sudo ./uninstall.sh
```

## MCP Server Setup

The MCP server allows AI agents (like Claude) to interact with network devices through the Annet-Oil API.

### Prerequisites

- Node.js 18+ installed
- npm or yarn package manager
- Access to Annet-Oil API server

### Installation

1. **Navigate to MCP directory**:
```bash
cd mcp-annet-oil
```

2. **Install dependencies**:
```bash
npm install
```

3. **Configure environment**:
```bash
cp .env.example .env
# Edit .env with your settings
vim .env
```

4. **Build the MCP server**:
```bash
npm run build
```

### Configuration

Edit `.env` file:

```bash
# Annet-Oil API Configuration
ANNET_OIL_API_URL=http://localhost:8080
ANNET_OIL_AUTH_TOKEN=your_api_token_here
```

### Running the MCP Server

**Development mode**:
```bash
npm run dev
```

**Production mode**:
```bash
npm start
```

### Claude Desktop Integration

Add to your Claude Desktop configuration (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "annet-oil": {
      "command": "node",
      "args": ["/path/to/mcp-annet-oil/dist/index.js"],
      "env": {
        "ANNET_OIL_API_URL": "http://your-server:8080",
        "ANNET_OIL_AUTH_TOKEN": "your_token"
      }
    }
  }
}
```

### Available MCP Tools

1. **annet_gen** - Generate network device configuration
2. **annet_diff** - Show configuration differences
3. **annet_patch** - Apply configuration patches
4. **annet_deploy** - Deploy configuration changes
5. **annet_containers** - Get container status
6. **annet_routing** - Get routing information
7. **annet_health** - Check API health
8. **annet_execute** - Execute whitelisted commands on devices
9. **annet_list_allowed_commands** - List allowed command categories

## API Endpoints

### Base URL
```
http://your-server:8080/api/v0
```

### Authentication
All requests require Bearer token authentication:
```
Authorization: Bearer your_token_here
```

### Endpoints

#### Generate Configuration
```
POST /gen
Content-Type: application/json

{
  "filters": ["hostname1", "hostname2"],
  "generators": ["interfaces", "routing"],
  "container": "container_name",
  "dry_run": false,
  "parallel": true,
  "timeout": 60
}
```

#### Show Differences
```
POST /diff
Content-Type: application/json

{
  "filters": ["hostname1"],
  "generators": ["interfaces"]
}
```

#### Apply Patches
```
POST /patch
Content-Type: application/json

{
  "filters": ["hostname1"],
  "generators": ["interfaces"],
  "dry_run": true
}
```

#### Deploy Configuration
```
POST /deploy
Content-Type: application/json

{
  "filters": ["hostname1"],
  "parallel": false
}
```

#### Execute Commands (Whitelisted)
```
POST /execute
Content-Type: application/json

{
  "command": "show interfaces",
  "filters": ["hostname1", "hostname2"],
  "container": "container_name",
  "timeout": 30
}
```

#### Get Container Status
```
GET /containers
```

#### Get Routing Information
```
GET /routing
GET /routing?hostname=device1
```

#### Health Check
```
GET /health              # Basic health check
GET /health/extended     # Extended health check with gnetcli status
```

Extended health check response example:
```json
{
  "status": "ok",
  "timestamp": "2024-06-29T10:30:00Z",
  "version": "1.0.0",
  "service": "annet-oil",
  "gnetcli": {
    "enabled": true,
    "running": true,
    "grpc_port": "443",
    "http_port": "50052",
    "status": "healthy",
    "service_info": "Active: active (running) since..."
  },
  "checks": {
    "gnetcli": {
      "status": "healthy",
      "message": "gnetcli service is running"
    },
    "api": {
      "status": "healthy",
      "message": "API is responsive"
    }
  }
}
```

## Security Considerations

### Command Whitelist

The `/execute` endpoint only allows whitelisted commands to prevent malicious operations. Allowed command categories include:

- **Read-only show commands**: `show version`, `show interfaces`, `show running-config`
- **Diagnostic commands**: `ping`, `traceroute`
- **Monitoring commands**: `show logging`, `show processes`
- **Layer 2/3 information**: `show vlan`, `show ip route`, `show arp`

### Best Practices

1. **Use strong authentication tokens**: Generate secure random tokens for API access
2. **Enable HTTPS**: Use TLS certificates for production deployments
3. **Limit network access**: Use firewall rules to restrict API access
4. **Regular updates**: Keep all dependencies up to date
5. **Audit logs**: Monitor system logs for suspicious activity
6. **Credential rotation**: Regularly rotate API tokens and passwords
7. **Principle of least privilege**: Grant minimum necessary permissions

### Environment Variables Security

- Never commit `.env` files to version control
- Use secure secret management systems in production
- Set appropriate file permissions (600) for configuration files
- Use environment-specific configurations

### Network Security

1. **Firewall Configuration**:
```bash
# Allow only specific IPs to access API
sudo ufw allow from 192.168.1.0/24 to any port 8080

# Enable firewall
sudo ufw enable
```

2. **TLS/SSL Setup**:
- Use reverse proxy (nginx/apache) with SSL certificates
- Redirect HTTP to HTTPS
- Use strong cipher suites

3. **Rate Limiting**:
- Implement rate limiting on API endpoints
- Use fail2ban for brute force protection

## Troubleshooting

### Common Issues

1. **Service fails to start**:
   - Check logs: `sudo journalctl -u gnetcli -n 50`
   - Verify configuration file syntax
   - Ensure ports are not in use

2. **MCP connection errors**:
   - Verify API server is accessible
   - Check authentication token
   - Review MCP server logs

3. **Command execution fails**:
   - Ensure command is whitelisted
   - Check device connectivity
   - Verify device credentials

### Debug Mode

Enable debug logging in gnetcli service:
```bash
# Edit service file to add -debug flag
ExecStart=/root/go/bin/gnetcli_server ... -debug
```

### Support

For issues and questions:
- Check application logs
- Review configuration files
- Test connectivity to devices
- Verify API authentication