# MCP Annet Oil Server

Model Context Protocol (MCP) server for integrating AI agents with Annet Oil API. This server provides structured tools for network configuration management through the Annet framework.

## Features

The MCP server exposes the following tools to AI agents:

- **annet_gen** - Generate network device configurations
- **annet_diff** - Show configuration differences
- **annet_patch** - Apply configuration patches
- **annet_deploy** - Deploy configuration changes
- **annet_containers** - Get status of Annet containers
- **annet_routing** - Get device routing information
- **annet_health** - Check API health status

## Installation

```bash
cd mcp-annet-oil
npm install
npm run build
```

## Configuration

### Environment Variables

Create a `.env` file based on `.env.example`:

```env
ANNET_OIL_API_URL=http://localhost:8080
ANNET_OIL_AUTH_TOKEN=your-auth-token
```

### Claude Desktop Integration

To use with Claude Desktop, add this configuration to your Claude Desktop config:

**macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "annet-oil": {
      "command": "node",
      "args": [
        "/absolute/path/to/mcp-annet-oil/dist/index.js"
      ],
      "env": {
        "ANNET_OIL_API_URL": "http://localhost:8080",
        "ANNET_OIL_AUTH_TOKEN": "your-auth-token"
      }
    }
  }
}
```

## Usage Examples

Once configured, Claude can use the Annet Oil tools directly:

### Generate Configuration

```
Use annet_gen to generate configuration for device Kragujevac-4948-10G.otk.rs
```

### Show Configuration Diff

```
Use annet_diff to show configuration differences for Kragujevac-4948-10G.otk.rs
```

### Deploy with Dry Run

```
Use annet_patch with dry_run=true for device Kragujevac-4948-10G.otk.rs
```

### Check Container Status

```
Use annet_containers to show the status of all Annet containers
```

## Tool Parameters

### Common Parameters (gen, diff, patch, deploy)

- `filters` - Array of device hostnames or patterns
- `generators` - Array of generator filters (e.g., "interfaces", "routing")
- `container` - Specific container to use
- `dry_run` - Perform dry run without changes (patch/deploy only)
- `parallel` - Execute in parallel mode
- `timeout` - Command timeout in seconds
- `quiet` - Suppress stderr warnings

### Routing Tool

- `hostname` - Optional specific hostname to check routing for

## Development

```bash
# Run in development mode
npm run dev

# Build for production
npm run build

# Start production server
npm start
```

## Testing

Test the MCP server locally:

```bash
# Start the server
npm run dev

# In another terminal, test with MCP client
npx @modelcontextprotocol/cli query --server "node dist/index.js"
```

## API Requirements

The MCP server requires an Annet Oil API server running with:

- API accessible at the configured URL (default: http://localhost:8080)
- Valid authentication token
- Configured Annet containers

## Error Handling

The server handles various error conditions:

- API connection failures
- Authentication errors
- Invalid parameters
- Container execution errors
- Timeout errors

All errors are returned with descriptive messages to help diagnose issues.

## Security

- Store authentication tokens securely
- Use HTTPS for production API connections
- Restrict API access to authorized clients only
- Never commit `.env` files with real tokens

## License

MIT