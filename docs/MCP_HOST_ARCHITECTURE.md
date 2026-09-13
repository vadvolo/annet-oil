# MCP + Annet Oil Architecture Documentation

## Architecture Overview

This setup implements a hybrid architecture where:
- **Annet Oil API** runs on the host system (for Docker socket access)
- **MCP Server** runs in a Docker container
- **Communication** happens via HTTP from container to host

```
┌─────────────────────────────────────────────────┐
│                   Host System                    │
│                                                  │
│  ┌──────────────────────────────────────────┐   │
│  │        Annet Oil API Server              │   │
│  │         (Port 8080)                      │   │
│  │                                          │   │
│  │  - Has Docker socket access              │   │
│  │  - Manages Annet containers              │   │
│  │  - Executes network commands             │   │
│  └──────────────────▲───────────────────────┘   │
│                     │                            │
│                     │ HTTP API                   │
│                     │                            │
│  ┌──────────────────┴───────────────────────┐   │
│  │         Docker Container                 │   │
│  │  ┌────────────────────────────────────┐  │   │
│  │  │      MCP Annet Oil Server         │  │   │
│  │  │                                    │  │   │
│  │  │  - Receives commands from Agent    │  │   │
│  │  │  - Proxies to Annet Oil API       │  │   │
│  │  │  - Returns results via stdio      │  │   │
│  │  └────────────────────────────────────┘  │   │
│  └──────────────────────────────────────────┘   │
│                                                  │
│  ┌──────────────────────────────────────────┐   │
│  │    Annet Containers (managed by API)     │   │
│  │    - annet-default                       │   │
│  │    - annet-telnet                        │   │
│  │    - annet-orion                         │   │
│  └──────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites
- Docker and Docker Compose installed
- Go installed (for building annet-oil-server)
- Annet containers available

### Setup

1. **Build Annet Oil Server**
   ```bash
   go build -o annet-oil-server .
   ```

2. **Configure Environment**
   ```bash
   cp .env.example .env
   # Edit .env to set ANNET_OIL_AUTH_TOKEN
   ```

3. **Start Everything**
   ```bash
   ./scripts/start-all.sh
   ```

   This will:
   - Start Annet containers (if configured)
   - Start Annet Oil API on host
   - Build and start MCP container

### Manual Start

#### Start Annet Oil API on Host
```bash
./scripts/start-annet-oil.sh
```

#### Build and Start MCP Container
```bash
# Build MCP container
docker-compose -f docker-compose.mcp.yml build

# Start MCP container
docker-compose -f docker-compose.mcp.yml up -d
```

## Configuration

### Environment Variables

Create `.env` file:
```env
# Authentication token (shared between services)
ANNET_OIL_AUTH_TOKEN=your-secure-token

# API URL for MCP to connect to host
# Use host.docker.internal for Docker Desktop
# On Linux, use host IP or host-gateway
ANNET_OIL_API_URL=http://host.docker.internal:8080

# Annet Oil API host configuration
ANNET_OIL_HOST=0.0.0.0
ANNET_OIL_PORT=8080
```

### Linux Host Networking

On Linux, `host.docker.internal` might not work. The docker-compose.mcp.yml includes:
```yaml
extra_hosts:
  - "host.docker.internal:host-gateway"
```

If this doesn't work, use your host's IP address in `.env`:
```env
ANNET_OIL_API_URL=http://172.17.0.1:8080  # Docker bridge IP
# or
ANNET_OIL_API_URL=http://192.168.1.100:8080  # Your host IP
```

## Usage

### Connect to MCP Server

The MCP server uses stdio transport. Connect via:

```bash
docker exec -it mcp-annet-oil node dist/index.js
```

### Integration with Claude Desktop

Update your Claude configuration:

```json
{
  "mcpServers": {
    "annet-oil": {
      "command": "docker",
      "args": ["exec", "-i", "mcp-annet-oil", "node", "dist/index.js"]
    }
  }
}
```

### Available MCP Tools

- `annet_gen` - Generate network device configuration
- `annet_diff` - Show configuration differences
- `annet_patch` - Apply configuration patches
- `annet_deploy` - Deploy configuration changes
- `annet_containers` - Get container status
- `annet_routing` - Get routing information
- `annet_health` - Check API health
- `annet_execute` - Execute whitelisted commands
- `annet_list_allowed_commands` - List allowed command categories

## Monitoring

### Check Service Status

```bash
# Check if Annet Oil API is running
curl http://localhost:8080/health

# Check MCP container status
docker ps | grep mcp-annet-oil

# View MCP logs
docker-compose -f docker-compose.mcp.yml logs -f
```

### Troubleshooting

#### MCP Can't Connect to API

1. Verify Annet Oil is running:
   ```bash
   curl http://localhost:8080/health
   ```

2. Check auth token matches:
   ```bash
   grep AUTH_TOKEN .env
   docker-compose -f docker-compose.mcp.yml exec mcp-annet-oil env | grep AUTH_TOKEN
   ```

3. Test connection from container:
   ```bash
   docker-compose -f docker-compose.mcp.yml exec mcp-annet-oil \
     sh -c "wget -qO- http://host.docker.internal:8080/health"
   ```

#### Permission Issues

Ensure annet-oil-server has permissions:
```bash
# For Docker socket access
sudo usermod -aG docker $USER
# Logout and login again
```

## Stopping Services

### Stop All Services
```bash
# If started with start-all.sh
Ctrl+C

# Manual cleanup
docker-compose -f docker-compose.mcp.yml down
docker-compose down  # If using Annet containers
```

### Stop Individual Services

```bash
# Stop MCP only
docker-compose -f docker-compose.mcp.yml down

# Stop Annet Oil API
# Find and kill the process
ps aux | grep annet-oil-server
kill <PID>
```

## Development

### Rebuild After Changes

#### MCP Changes
```bash
cd mcp-annet-oil
# Make changes to TypeScript files
docker-compose -f ../docker-compose.mcp.yml build --no-cache
docker-compose -f ../docker-compose.mcp.yml up -d --force-recreate
```

#### Annet Oil API Changes
```bash
# Rebuild Go binary
go build -o annet-oil-server .

# Restart service
./scripts/start-annet-oil.sh
```

## Security Considerations

1. **Authentication Token**: Always use a strong, unique token in production
2. **Network Isolation**: MCP container is isolated, only communicates with host API
3. **Host Access**: Annet Oil API has Docker socket access - ensure proper security
4. **Port Binding**: API binds to 0.0.0.0:8080 - consider firewall rules

## Benefits of This Architecture

1. **Docker Access**: Annet Oil API can manage Docker containers from host
2. **Isolation**: MCP runs isolated in container, reducing attack surface
3. **Portability**: MCP container can be easily distributed and deployed
4. **No Local Dependencies**: No need for Node.js/npm on host system
5. **Easy Updates**: Update MCP by rebuilding container, API independently