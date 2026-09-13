#!/bin/bash

# Start complete Annet Oil + MCP stack
# This script starts Annet Oil API on host and MCP in container

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$ROOT_DIR"

# Load environment variables
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
else
    echo "Creating .env file from .env.example..."
    cp .env.example .env
    echo "Please edit .env file to set your ANNET_OIL_AUTH_TOKEN"
    echo ""
fi

echo "==================================="
echo "Starting Annet Oil Stack"
echo "==================================="
echo ""

# Step 1: Start Annet containers (if using docker-compose.yml)
if [ -f "docker-compose.yml" ]; then
    echo "Starting Annet containers..."
    docker-compose up -d annet-default annet-telnet annet-orion
    echo "Annet containers started"
    echo ""
fi

# Step 2: Start Annet Oil API on host
echo "Starting Annet Oil API on host..."
echo "(Press Ctrl+C to stop)"
echo ""

# Start in background
./scripts/start-annet-oil.sh &
ANNET_OIL_PID=$!

# Wait for API to be ready
echo "Waiting for Annet Oil API to be ready..."
for i in {1..30}; do
    if curl -s -f http://localhost:8080/health > /dev/null 2>&1; then
        echo "Annet Oil API is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "Error: Annet Oil API failed to start"
        kill $ANNET_OIL_PID 2>/dev/null || true
        exit 1
    fi
    sleep 1
done

echo ""

# Step 3: Build and start MCP container
echo "Building MCP container..."
docker-compose -f docker-compose.mcp.yml build

echo "Starting MCP container..."
docker-compose -f docker-compose.mcp.yml up -d

echo ""
echo "==================================="
echo "Stack started successfully!"
echo "==================================="
echo ""
echo "Services:"
echo "  - Annet Oil API: http://localhost:8080"
echo "  - MCP Container: mcp-annet-oil (stdio)"
echo ""
echo "To connect to MCP:"
echo "  docker exec -it mcp-annet-oil node dist/index.js"
echo ""
echo "To view logs:"
echo "  - Annet Oil: Running in foreground"
echo "  - MCP: docker-compose -f docker-compose.mcp.yml logs -f"
echo ""
echo "To stop all services:"
echo "  - Press Ctrl+C to stop Annet Oil"
echo "  - Run: docker-compose -f docker-compose.mcp.yml down"
echo ""

# Keep script running and forward signals
trap "echo 'Stopping services...'; kill $ANNET_OIL_PID 2>/dev/null || true; docker-compose -f docker-compose.mcp.yml down; exit" INT TERM
wait $ANNET_OIL_PID