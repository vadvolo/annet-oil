# Local Testing Guide for Annet-Oil with AI Agents

This guide helps you run annet-oil with gnetcli server locally for testing with AI agents (Claude).

## Quick Start

### 1. Start Annet-Oil API Server

```bash
# From the annet-oil root directory
cd /Users/vadvolo/Projects/annet-oil

# Run the API server (assuming you have a local config)
go run cmd/annet-oil/main.go --config configs/local.yaml

# Or if you have it built:
./annet-oil --config configs/local.yaml
```

### 2. Start gnetcli Server (if needed)

If you need the gnetcli server component:

```bash
# Build gnetcli server
go build -o gnetcli_server ./cmd/gnetcli_server

# Run with basic auth (adjust ports as needed)
./gnetcli_server \
    -port 0.0.0.0:50051 \
    -http_port 0.0.0.0:50052 \
    -basic-auth admin:password \
    -debug
```

### 3. Configure and Start MCP Server

```bash
# Navigate to MCP directory
cd mcp-annet-oil

# Install dependencies if not done
npm install

# Create .env file
cat > .env << EOF
ANNET_OIL_API_URL=http://localhost:8080
ANNET_OIL_AUTH_TOKEN=your-test-token
EOF

# Build the MCP server
npm run build

# Start MCP server in development mode
npm run dev
```

### 4. Configure Claude Desktop

Edit your Claude Desktop configuration:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "annet-oil-local": {
      "command": "node",
      "args": [
        "/Users/vadvolo/Projects/annet-oil/mcp-annet-oil/dist/index.js"
      ],
      "env": {
        "ANNET_OIL_API_URL": "http://localhost:8080",
        "ANNET_OIL_AUTH_TOKEN": "your-test-token"
      }
    }
  }
}
```

### 5. Restart Claude Desktop

After updating the configuration, restart Claude Desktop to load the MCP server.

## Testing the Setup

### Test 1: Check API Health

```bash
# Basic health check
curl -H "Authorization: Bearer your-test-token" \
     http://localhost:8080/api/v0/health

# Extended health check (with gnetcli status)
curl -H "Authorization: Bearer your-test-token" \
     http://localhost:8080/api/v0/health/extended
```

### Test 2: Test Container Status

```bash
curl -H "Authorization: Bearer your-test-token" \
     http://localhost:8080/api/v0/containers
```

### Test 3: Test Command Execution

```bash
curl -X POST \
     -H "Authorization: Bearer your-test-token" \
     -H "Content-Type: application/json" \
     -d '{"command": "show version", "filters": ["test-device"]}' \
     http://localhost:8080/api/v0/execute
```

### Test 4: Test with Claude

In Claude, you can now use commands like:

```
Use annet_health to check if the API is working

Use annet_list_allowed_commands to see what commands I can execute

Use annet_containers to show container status

Use annet_execute with command="show interfaces" for device test-router
```

## Docker Setup (Alternative)

If you prefer using Docker:

### 1. Create docker-compose.yml

```yaml
version: '3.8'

services:
  annet-oil:
    build: .
    ports:
      - "8080:8080"
    environment:
      - CONFIG_PATH=/app/configs/local.yaml
      - API_AUTH_TOKEN=your-test-token
    volumes:
      - ./configs:/app/configs
      - ./generators:/app/generators
      - ./tests/fixtures:/app/fixtures

  gnetcli:
    build:
      context: .
      dockerfile: Dockerfile.gnetcli
    ports:
      - "50051:50051"
      - "50052:50052"
    environment:
      - GNETCLI_LOGIN=admin
      - GNETCLI_PASSWORD=password
      - DEVICE_LOGIN=device_user
      - DEVICE_PASSWORD=device_pass
```

### 2. Run with Docker Compose

```bash
docker-compose up
```

## Local Development Setup

### Environment Variables

Create a `.env.local` file:

```bash
# API Configuration
ANNET_OIL_API_PORT=8080
ANNET_OIL_API_TOKEN=your-test-token

# gnetcli Configuration
GNETCLI_GRPC_PORT=50051
GNETCLI_HTTP_PORT=50052
GNETCLI_LOGIN=admin
GNETCLI_PASSWORD=password

# Device Credentials (for testing)
DEVICE_LOGIN=test_user
DEVICE_PASSWORD=test_pass

# Container Configuration
ANNET_CONTAINER_PATH=/path/to/annet/containers
```

### Mock Device Setup

For testing without real devices, create a mock configuration:

```yaml
# configs/mock-devices.yaml
devices:
  - hostname: test-router-1
    ip: 192.168.1.1
    vendor: cisco
    model: ISR4451
  - hostname: test-switch-1
    ip: 192.168.1.2
    vendor: cisco
    model: WS-C3850
```

## Troubleshooting

### Port Already in Use

```bash
# Check what's using port 8080
lsof -i :8080

# Kill the process if needed
kill -9 <PID>
```

### MCP Server Not Connecting

1. Check if API is running:
```bash
curl http://localhost:8080/api/v0/health
```

2. Check MCP server logs:
```bash
cd mcp-annet-oil
npm run dev
# Look for error messages
```

3. Verify environment variables:
```bash
cat mcp-annet-oil/.env
```

### Claude Not Finding MCP Server

1. Check Claude Desktop config:
```bash
cat ~/Library/Application\ Support/Claude/claude_desktop_config.json
```

2. Ensure absolute paths are used in the config

3. Restart Claude Desktop completely (Quit and reopen)

### Permission Issues

```bash
# Make scripts executable
chmod +x configs/systemd/*.sh

# Fix ownership if needed
sudo chown -R $(whoami) .
```

## Testing Workflow

1. **Start Services**:
```bash
# Terminal 1: API Server
go run cmd/annet-oil/main.go --config configs/local.yaml

# Terminal 2: MCP Server
cd mcp-annet-oil && npm run dev
```

2. **Test API Directly**:
```bash
# Use the test scripts
./test_api.sh
```

3. **Test with Claude**:
- Open Claude Desktop
- Ask: "Can you check the health of my annet-oil API?"
- Ask: "Show me the allowed commands you can execute"
- Ask: "Execute 'show version' on test-router-1"

## Sample Test Commands

```bash
# Test health endpoint
curl -s -H "Authorization: Bearer your-test-token" \
     http://localhost:8080/api/v0/health | jq .

# Test container status
curl -s -H "Authorization: Bearer your-test-token" \
     http://localhost:8080/api/v0/containers | jq .

# Test command execution
curl -s -X POST \
     -H "Authorization: Bearer your-test-token" \
     -H "Content-Type: application/json" \
     -d '{"command": "show version"}' \
     http://localhost:8080/api/v0/execute | jq .

# Test MCP server directly
cd mcp-annet-oil
node -e "console.log(require('./dist/index.js'))"
```

## Debug Mode

Enable debug logging:

```bash
# For API server
export DEBUG=true
go run cmd/annet-oil/main.go --config configs/local.yaml

# For MCP server
export DEBUG=mcp:*
cd mcp-annet-oil && npm run dev

# For gnetcli
./gnetcli_server -debug
```

## Next Steps

1. Create mock data for testing without real devices
2. Set up integration tests
3. Configure monitoring and logging
4. Add custom generators for your network setup