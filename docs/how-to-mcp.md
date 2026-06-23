# How to Use MCP with Annet Oil

This guide explains how to set up and use the Model Context Protocol (MCP) server for Annet Oil, enabling AI agents to interact with your network automation infrastructure.

## Overview

The MCP server provides a structured interface for AI agents (like Claude) to execute Annet commands through the Annet Oil API. This allows natural language control of network configuration management tasks.

## Prerequisites

- Node.js 18 or higher
- Annet Oil API server running
- Valid API authentication token

## Installation

### 1. Clone and Build

```bash
cd annet-oil/mcp-annet-oil
npm install
npm run build
```

### 2. Configure Environment

Create a `.env` file with your API credentials:

```bash
ANNET_OIL_API_URL=http://localhost:8080
ANNET_OIL_AUTH_TOKEN=your-auth-token-here
```

## MCP Tools Available

The MCP server exposes the following tools to AI agents:

| Tool | Description | Parameters |
|------|-------------|------------|
| `annet_gen` | Generate device configuration | filters, generators, container, parallel, timeout, quiet |
| `annet_diff` | Show configuration differences | filters, generators, container, parallel, timeout, quiet |
| `annet_patch` | Apply configuration patches | filters, generators, container, dry_run, parallel, timeout |
| `annet_deploy` | Deploy configuration changes | filters, generators, container, dry_run, parallel, timeout |
| `annet_containers` | Get container status | none |
| `annet_routing` | Get routing information | hostname (optional) |
| `annet_health` | Check API health | none |

## Integration with Claude Desktop

### macOS Configuration

1. Edit Claude Desktop configuration:
```bash
nano ~/Library/Application\ Support/Claude/claude_desktop_config.json
```

2. Add the MCP server configuration:
```json
{
  "mcpServers": {
    "annet-oil": {
      "command": "node",
      "args": [
        "/absolute/path/to/annet-oil/mcp-annet-oil/dist/index.js"
      ],
      "env": {
        "ANNET_OIL_API_URL": "http://192.168.52.235:8181",
        "ANNET_OIL_AUTH_TOKEN": "your-token-here"
      }
    }
  }
}
```

3. Restart Claude Desktop

### Windows Configuration

1. Edit configuration at:
```
%APPDATA%\Claude\claude_desktop_config.json
```

2. Use the same JSON configuration with Windows paths:
```json
{
  "mcpServers": {
    "annet-oil": {
      "command": "node",
      "args": [
        "C:\\path\\to\\annet-oil\\mcp-annet-oil\\dist\\index.js"
      ],
      "env": {
        "ANNET_OIL_API_URL": "http://192.168.52.235:8181",
        "ANNET_OIL_AUTH_TOKEN": "your-token-here"
      }
    }
  }
}
```

## Usage Examples

Once configured, you can interact with the network infrastructure using natural language:

### Generate Configuration

```
"Generate configuration for device Kragujevac-4948-10G.otk.rs"
```

Claude will use the `annet_gen` tool to generate the configuration.

### Show Configuration Differences

```
"Show me the configuration differences for routers router1 and router2"
```

### Apply Changes with Dry Run

```
"Apply configuration patches to device switch01 but do a dry run first"
```

### Check System Status

```
"Check the status of all Annet containers"
```

### Deploy Configuration

```
"Deploy the new routing configuration to all edge routers"
```

## Testing the MCP Server

### Manual Test

Create a test script to verify the MCP server is working:

```javascript
// test-mcp.js
import { AnnetOilClient } from './dist/client.js';

const client = new AnnetOilClient({
  apiUrl: 'http://localhost:8080',
  authToken: 'your-token',
});

// Test health
client.health()
  .then(result => console.log('Health:', result))
  .catch(err => console.error('Error:', err));

// Test gen command
client.gen({ filters: ['device-name'] })
  .then(result => console.log('Gen result:', result))
  .catch(err => console.error('Error:', err));
```

Run the test:
```bash
node test-mcp.js
```

### Using MCP CLI

Test with the MCP CLI tool:

```bash
npx @modelcontextprotocol/cli query \
  --server "node /path/to/mcp-annet-oil/dist/index.js" \
  --tool annet_health
```

## Advanced Configuration

### Custom Timeout

Set custom timeout for long-running operations:

```json
"env": {
  "ANNET_OIL_API_URL": "http://localhost:8080",
  "ANNET_OIL_AUTH_TOKEN": "your-token",
  "ANNET_OIL_TIMEOUT": "120000"
}
```

### Multiple Environments

Configure multiple MCP servers for different environments:

```json
{
  "mcpServers": {
    "annet-oil-prod": {
      "command": "node",
      "args": ["/path/to/mcp-annet-oil/dist/index.js"],
      "env": {
        "ANNET_OIL_API_URL": "https://prod-api.example.com",
        "ANNET_OIL_AUTH_TOKEN": "prod-token"
      }
    },
    "annet-oil-staging": {
      "command": "node",
      "args": ["/path/to/mcp-annet-oil/dist/index.js"],
      "env": {
        "ANNET_OIL_API_URL": "https://staging-api.example.com",
        "ANNET_OIL_AUTH_TOKEN": "staging-token"
      }
    }
  }
}
```

## Docker Deployment

### Using Docker

Create a Dockerfile for the MCP server:

```dockerfile
FROM node:20-alpine
WORKDIR /app
COPY mcp-annet-oil/package*.json ./
RUN npm ci --only=production
COPY mcp-annet-oil/dist ./dist
CMD ["node", "dist/index.js"]
```

Build and run:
```bash
docker build -t mcp-annet-oil .
docker run -e ANNET_OIL_API_URL=http://api:8080 \
           -e ANNET_OIL_AUTH_TOKEN=token \
           mcp-annet-oil
```

## Troubleshooting

### Common Issues

1. **Connection Refused**
   - Verify the API URL is correct
   - Check if the Annet Oil API server is running
   - Ensure network connectivity between MCP server and API

2. **Authentication Failed**
   - Verify the auth token is correct
   - Check token format (Bearer token expected)

3. **Node.js Version Error**
   - Ensure Node.js 18 or higher is installed
   - Use `node --version` to check

4. **MCP Not Available in Claude**
   - Restart Claude Desktop after configuration changes
   - Check the configuration file path is correct
   - Verify the MCP server path is absolute, not relative

### Debug Logging

Enable debug logging by setting environment variable:

```bash
DEBUG=mcp:* node dist/index.js
```

## Security Considerations

1. **Token Security**
   - Never commit tokens to version control
   - Use environment variables or secure secret management
   - Rotate tokens regularly

2. **Network Security**
   - Use HTTPS for production API connections
   - Implement proper firewall rules
   - Restrict API access to authorized clients

3. **Audit Logging**
   - Monitor MCP server usage
   - Log all configuration changes
   - Track command execution history

## Best Practices

1. **Use Dry Run**
   - Always test with `dry_run: true` before applying changes
   - Review diff output before deploying

2. **Batch Operations**
   - Use parallel mode for multiple devices
   - Set appropriate timeouts for large operations

3. **Error Handling**
   - Check command responses for errors
   - Implement retry logic for transient failures

4. **Testing**
   - Test in staging environment first
   - Validate configurations before deployment
   - Maintain rollback procedures

## API Reference

For detailed API documentation, see:
- [Annet Oil REST API Documentation](./how-to-rest-api.md)
- [MCP Protocol Specification](https://modelcontextprotocol.io/docs)

## Support

For issues or questions:
- GitHub Issues: [annet-oil/issues](https://github.com/your-org/annet-oil/issues)
- MCP Documentation: [modelcontextprotocol.io](https://modelcontextprotocol.io)