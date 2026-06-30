#!/bin/bash

# Start script for local testing of annet-oil with AI agents

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

print_msg() {
    echo -e "${2}${1}${NC}"
}

print_msg "Starting Annet-Oil Local Testing Environment..." $GREEN

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_msg "Go is not installed. Please install Go first." $RED
    exit 1
fi

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    print_msg "Node.js is not installed. Please install Node.js first." $RED
    exit 1
fi

# Function to cleanup on exit
cleanup() {
    print_msg "\nStopping services..." $YELLOW

    # Kill API server
    if [ ! -z "$API_PID" ]; then
        kill $API_PID 2>/dev/null || true
    fi

    # Kill MCP server
    if [ ! -z "$MCP_PID" ]; then
        kill $MCP_PID 2>/dev/null || true
    fi

    # Kill gnetcli server
    if [ ! -z "$GNETCLI_PID" ]; then
        kill $GNETCLI_PID 2>/dev/null || true
    fi

    print_msg "Services stopped." $GREEN
    exit 0
}

# Set trap for cleanup
trap cleanup EXIT INT TERM

# Step 1: Build the API server
print_msg "\n1. Building Annet-Oil API server..." $YELLOW
go build -o annet-oil-server cmd/annet-oil/main.go || {
    print_msg "Failed to build API server" $RED
    exit 1
}

# Step 2: Start API server
print_msg "2. Starting API server on port 8080..." $YELLOW
./annet-oil-server --config configs/local.yaml server api &
API_PID=$!
sleep 2

# Check if API server started
if ! curl -s -H "Authorization: Bearer test-token-12345" http://localhost:8080/api/v0/health > /dev/null 2>&1; then
    print_msg "API server failed to start. Check logs above." $RED
    exit 1
fi
print_msg "   API server started successfully!" $GREEN

# Step 3: Build and start gnetcli server (optional)
if [ "$1" == "--with-gnetcli" ]; then
    print_msg "\n3. Building gnetcli server..." $YELLOW
    if [ -f "cmd/gnetcli_server/main.go" ]; then
        go build -o gnetcli_server cmd/gnetcli_server/main.go
        print_msg "   Starting gnetcli server..." $YELLOW
        ./gnetcli_server \
            -port 0.0.0.0:50051 \
            -http_port 0.0.0.0:50052 \
            -basic-auth admin:password \
            -debug &
        GNETCLI_PID=$!
        sleep 2
        print_msg "   gnetcli server started!" $GREEN
    else
        print_msg "   gnetcli server source not found, skipping..." $YELLOW
    fi
else
    print_msg "\n3. Skipping gnetcli server (use --with-gnetcli to enable)" $YELLOW
fi

# Step 4: Setup MCP server
print_msg "\n4. Setting up MCP server..." $YELLOW
cd mcp-annet-oil

# Install dependencies if needed
if [ ! -d "node_modules" ]; then
    print_msg "   Installing npm dependencies..." $YELLOW
    npm install
fi

# Create .env file
print_msg "   Creating .env file..." $YELLOW
cat > .env << EOF
ANNET_OIL_API_URL=http://localhost:8080
ANNET_OIL_AUTH_TOKEN=test-token-12345
EOF

# Build MCP server
print_msg "   Building MCP server..." $YELLOW
npm run build

# Start MCP server
print_msg "   Starting MCP server..." $YELLOW
npm run dev &
MCP_PID=$!
cd ..
sleep 3

print_msg "\n✅ All services are running!" $GREEN
print_msg "\nService Status:" $YELLOW
print_msg "  • API Server: http://localhost:8080" $NC
print_msg "  • API Health: http://localhost:8080/api/v0/health" $NC
if [ "$1" == "--with-gnetcli" ]; then
    print_msg "  • gnetcli gRPC: localhost:50051" $NC
    print_msg "  • gnetcli HTTP: localhost:50052" $NC
fi
print_msg "  • MCP Server: Running in development mode" $NC

print_msg "\n📝 Claude Desktop Configuration:" $YELLOW
print_msg "Add this to ~/Library/Application Support/Claude/claude_desktop_config.json:" $NC
echo '
{
  "mcpServers": {
    "annet-oil-local": {
      "command": "node",
      "args": ["'$(pwd)'/mcp-annet-oil/dist/index.js"],
      "env": {
        "ANNET_OIL_API_URL": "http://localhost:8080",
        "ANNET_OIL_AUTH_TOKEN": "test-token-12345"
      }
    }
  }
}'

print_msg "\n🧪 Test Commands:" $YELLOW
print_msg "Test API health:" $NC
echo "  curl -H 'Authorization: Bearer test-token-12345' http://localhost:8080/api/v0/health"

print_msg "\nTest containers:" $NC
echo "  curl -H 'Authorization: Bearer test-token-12345' http://localhost:8080/api/v0/containers"

print_msg "\nTest command execution:" $NC
echo "  curl -X POST -H 'Authorization: Bearer test-token-12345' -H 'Content-Type: application/json' \\"
echo "    -d '{\"command\": \"show version\"}' http://localhost:8080/api/v0/execute"

print_msg "\n⚡ In Claude, try these commands:" $YELLOW
echo "  - Use annet_health to check the API"
echo "  - Use annet_list_allowed_commands to see allowed commands"
echo "  - Use annet_execute with command='show interfaces' for device test-router-1"

print_msg "\n🛑 Press Ctrl+C to stop all services" $RED

# Wait indefinitely
while true; do
    sleep 1
done