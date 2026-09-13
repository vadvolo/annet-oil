# MCP Annet Oil Docker Container

This document describes how to build and run the MCP (Model Context Protocol) server as a Docker container that connects to the Annet Oil API.

## Overview

The MCP Annet Oil server is now containerized, eliminating the need for local npm installation. The container communicates with the Annet Oil API service through Docker networking.

## Prerequisites

- Docker and Docker Compose installed
- Annet Oil server running (or use docker-compose to start all services)

## Configuration

### Environment Variables

Create a `.env` file in the root directory (copy from `.env.example`):

```bash
cp .env.example .env
```

Edit the `.env` file to set your authentication token:
```
ANNET_OIL_AUTH_TOKEN=your-secure-token-here
```

The MCP container uses the following environment variables:
- `ANNET_OIL_API_URL`: URL of the Annet Oil API (default: `http://annet-oil:8080`)
- `ANNET_OIL_AUTH_TOKEN`: Authentication token for API access
- `NODE_ENV`: Node environment (default: `production`)

## Building the Container

### Quick Build

Run the provided build script:
```bash
./scripts/build-mcp.sh
```

### Manual Build

Build only the MCP container:
```bash
docker-compose build mcp-annet-oil
```

Build all services:
```bash
docker-compose build
```

## Running the Container

### Start All Services

Start Annet Oil and MCP together:
```bash
docker-compose up -d
```

### Start MCP Only

If Annet Oil is already running:
```bash
docker-compose up -d mcp-annet-oil
```

### View Logs

```bash
docker-compose logs -f mcp-annet-oil
```

## Connecting to MCP Server

The MCP server uses stdio transport. To interact with it:

### Via Docker Exec

```bash
docker exec -it mcp-annet-oil node dist/index.js
```

### Integration with Claude Desktop

To use with Claude Desktop, update your Claude configuration to use Docker:

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

## Container Details

### Dockerfile Structure

The container uses a multi-stage build:
1. **Builder stage**: Compiles TypeScript to JavaScript
2. **Runtime stage**: Runs the compiled application with production dependencies only

### Security

- Runs as non-root user (nodejs:1001)
- Uses Alpine Linux for minimal attack surface
- Production dependencies only in runtime image

### Networking

- Connects to `annet-network` Docker network
- Communicates with `annet-oil` service via internal Docker DNS
- No external ports exposed (uses stdio transport)

## Troubleshooting

### Container Won't Start

Check if Annet Oil service is healthy:
```bash
docker-compose ps
docker-compose logs annet-oil
```

### Authentication Issues

Verify the auth token matches between services:
```bash
docker-compose exec mcp-annet-oil env | grep AUTH_TOKEN
docker-compose exec annet-oil env | grep AUTH_TOKEN
```

### Connection Issues

Test API connectivity from MCP container:
```bash
docker-compose exec mcp-annet-oil sh -c "wget -qO- http://annet-oil:8080/health"
```

## Development

To rebuild after code changes:
```bash
docker-compose build --no-cache mcp-annet-oil
docker-compose restart mcp-annet-oil
```

## Stopping Services

Stop all services:
```bash
docker-compose down
```

Stop and remove volumes:
```bash
docker-compose down -v
```