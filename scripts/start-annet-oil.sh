#!/bin/bash

# Start Annet Oil API server on host
# This script starts the Annet Oil API that will be accessed by the MCP container

set -e

# Load environment variables
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Set default values if not provided
ANNET_OIL_HOST=${ANNET_OIL_HOST:-0.0.0.0}
ANNET_OIL_PORT=${ANNET_OIL_PORT:-8080}
ANNET_OIL_AUTH_TOKEN=${ANNET_OIL_AUTH_TOKEN:-change-me-in-production}

echo "Starting Annet Oil API server on host..."
echo "Host: $ANNET_OIL_HOST"
echo "Port: $ANNET_OIL_PORT"
echo ""

# Check if annet-oil-server binary exists
if [ ! -f "./annet-oil-server" ]; then
    echo "Error: annet-oil-server binary not found!"
    echo "Please build it first with: go build -o annet-oil-server ."
    exit 1
fi

# Check if config exists
if [ ! -f "./configs/config.yaml" ]; then
    echo "Warning: configs/config.yaml not found!"
    echo "Using configs/config.example.yaml as template..."
    if [ -f "./configs/config.example.yaml" ]; then
        cp ./configs/config.example.yaml ./configs/config.yaml
        echo "Please edit configs/config.yaml with your settings"
    fi
fi

# Export environment variables for the server
export ANNET_OIL_HOST
export ANNET_OIL_PORT
export ANNET_OIL_AUTH_TOKEN
export ANNET_OIL_CONFIG_PATH=./configs/config.yaml

# Start the server
echo "Starting server..."
./annet-oil-server