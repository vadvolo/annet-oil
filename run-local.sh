#!/bin/bash

# Quick run script for local testing

# Build the binary
echo "Building annet-oil..."
go build -o annet-oil cmd/annet-oil/main.go

# Export config path
export ANNET_OIL_CONFIG="./configs/local.yaml"

# Run the API server
echo "Starting API server..."
echo "Config: $ANNET_OIL_CONFIG"
echo "API will be available at: http://localhost:8080"
echo "Auth token: test-token-12345"
echo ""
echo "Press Ctrl+C to stop"
echo ""

./annet-oil server api