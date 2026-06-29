# How to Add mcp-annet-oil to Claude CLI

This guide explains how to configure the `mcp-annet-oil` MCP server for use with the Claude CLI (Claude Code).

## Prerequisites

Build the MCP server first:

```bash
cd mcp-annet-oil
npm install
npm run build
```

## Add MCP Server to Claude CLI

Run the following command to register the MCP server:

```bash
claude mcp add annet-oil node /absolute/path/to/annet-oil/mcp-annet-oil/dist/index.js \
  -e ANNET_OIL_API_URL=http://localhost:8080 \
  -e ANNET_OIL_AUTH_TOKEN=your-auth-token
```

Replace `/absolute/path/to/annet-oil` with the actual path to the project root.

## Verify Configuration

```bash
claude mcp list
```

You should see `annet-oil` in the list of configured MCP servers.

## Configuration File Location

Claude CLI stores MCP configuration in:

- **macOS/Linux:** `~/.claude.json`

The entry looks like:

```json
{
  "mcpServers": {
    "annet-oil": {
      "command": "node",
      "args": ["/absolute/path/to/mcp-annet-oil/dist/index.js"],
      "env": {
        "ANNET_OIL_API_URL": "http://localhost:8080",
        "ANNET_OIL_AUTH_TOKEN": "your-auth-token"
      }
    }
  }
}
```

## Project-scoped Configuration

To share the MCP config with your team, add a `.mcp.json` file to the project root:

```json
{
  "mcpServers": {
    "annet-oil": {
      "command": "node",
      "args": ["./mcp-annet-oil/dist/index.js"],
      "env": {
        "ANNET_OIL_API_URL": "http://localhost:8080",
        "ANNET_OIL_AUTH_TOKEN": "your-auth-token"
      }
    }
  }
}
```

Claude CLI automatically picks up `.mcp.json` from the project root when you run `claude` inside the project directory.

## Available Tools

Once configured, the following tools are available in Claude CLI sessions:

| Tool | Description |
|------|-------------|
| `annet_gen` | Generate device configurations |
| `annet_diff` | Show configuration differences |
| `annet_patch` | Apply configuration patches |
| `annet_deploy` | Deploy configuration changes |
| `annet_containers` | Get Annet container status |
| `annet_routing` | Get device routing information |
| `annet_health` | Check API health status |

## Remove the Server

```bash
claude mcp remove annet-oil
```
