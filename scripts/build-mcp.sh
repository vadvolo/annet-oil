#!/bin/bash

# Build MCP Annet Oil Docker container
# This script builds the MCP container that interfaces with the Annet Oil API

set -e

echo "Building MCP Annet Oil container..."

# Check if we're in the right directory
if [ ! -f "docker-compose.yml" ]; then
    echo "Error: Please run this script from the annet-oil root directory"
    exit 1
fi

# Check if .env file exists, if not copy from example
if [ ! -f ".env" ]; then
    echo "Creating .env file from .env.example..."
    cp .env.example .env
    echo "Please edit .env file to set your ANNET_OIL_AUTH_TOKEN"
fi

# Build the MCP container
echo "Building MCP container..."
docker-compose build mcp-annet-oil

echo "MCP container built successfully!"
echo ""
echo "To start all services, run:"
echo "  docker-compose up -d"
echo ""
echo "To start only the MCP service (requires annet-oil to be running):"
echo "  docker-compose up -d mcp-annet-oil"
echo ""
echo "To connect to the MCP server via stdio:"
echo "  docker exec -it mcp-annet-oil node dist/index.js"