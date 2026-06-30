# 🚀 Quick Start - Local Testing with AI Agent

## Prerequisites
- Go 1.21+
- Node.js 18+
- Claude Desktop app

## 1. Start Everything (One Command)

```bash
./start-local.sh
```

This starts:
- ✅ Annet-Oil API server on port 8080
- ✅ MCP server for Claude integration
- ✅ All necessary configurations

To include gnetcli server:
```bash
./start-local.sh --with-gnetcli
```

## 2. Configure Claude Desktop

Copy this to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "annet-oil-local": {
      "command": "node",
      "args": ["/Users/vadvolo/Projects/annet-oil/mcp-annet-oil/dist/index.js"],
      "env": {
        "ANNET_OIL_API_URL": "http://localhost:8080",
        "ANNET_OIL_AUTH_TOKEN": "test-token-12345"
      }
    }
  }
}
```

**Then restart Claude Desktop completely (Quit and reopen).**

## 3. Test in Claude

Ask Claude:
- "Check the health of annet-oil API"
- "List allowed commands you can execute"
- "Show me container status"
- "Execute 'show interfaces' on test-router-1"

## 4. Test API Directly

```bash
# Run all tests
./test-local-api.sh

# Or test individual endpoints:

# Health check
curl -H "Authorization: Bearer test-token-12345" \
     http://localhost:8080/api/v0/health

# Container status
curl -H "Authorization: Bearer test-token-12345" \
     http://localhost:8080/api/v0/containers

# Execute command
curl -X POST \
     -H "Authorization: Bearer test-token-12345" \
     -H "Content-Type: application/json" \
     -d '{"command": "show version"}' \
     http://localhost:8080/api/v0/execute
```

## Stop Services

Press `Ctrl+C` in the terminal running `start-local.sh`

## Troubleshooting

### Claude doesn't see the MCP server
1. Make sure you restarted Claude Desktop after updating config
2. Check the config path is correct
3. Verify services are running: `./test-local-api.sh`

### Port 8080 already in use
```bash
lsof -i :8080
kill -9 <PID>
```

### API returns 401 Unauthorized
Check the auth token in your requests matches: `test-token-12345`

## Manual Setup (Alternative)

### Terminal 1: API Server
```bash
go run cmd/annet-oil/main.go --config configs/local.yaml
```

### Terminal 2: MCP Server
```bash
cd mcp-annet-oil
npm install
npm run build
npm run dev
```

## What's Running?

| Service | URL/Port | Purpose |
|---------|----------|---------|
| API Server | http://localhost:8080 | Main Annet-Oil API |
| MCP Server | (stdio) | Claude Desktop integration |
| gnetcli (optional) | localhost:50051 | Device communication |

## Available MCP Commands in Claude

- `annet_health` - Check API health
- `annet_containers` - List container status
- `annet_execute` - Run whitelisted commands
- `annet_list_allowed_commands` - Show allowed commands
- `annet_gen` - Generate configuration
- `annet_diff` - Show config differences
- `annet_patch` - Apply patches
- `annet_deploy` - Deploy changes
- `annet_routing` - Get routing info

## Example Claude Conversation

```
You: Check if my network API is working

Claude: I'll check the health of your annet-oil API.
[Uses annet_health tool]
The API is working! Status: ok, Service: annet-oil

You: What commands can you run on network devices?

Claude: Let me show you the allowed command categories.
[Uses annet_list_allowed_commands tool]
I can execute these types of commands:
1. Interface information
2. Configuration display
3. Routing protocols
...

You: Show the interfaces on test-router-1

Claude: I'll execute the show interfaces command on test-router-1.
[Uses annet_execute tool with command="show interfaces"]
```

## 🎉 That's it! You're ready to test with Claude!