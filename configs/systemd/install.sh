#!/bin/bash

# Gnetcli Server Deployment Script
# This script installs gnetcli_server as a systemd service

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_msg() {
    echo -e "${2}${1}${NC}"
}

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    print_msg "This script must be run as root" $RED
    exit 1
fi

print_msg "Starting gnetcli server deployment..." $GREEN

# Create configuration directory
print_msg "Creating configuration directory..." $YELLOW
mkdir -p /etc/gnetcli

# Copy environment file
print_msg "Copying environment configuration..." $YELLOW
cp gnetcli.env /etc/gnetcli/gnetcli.env
chmod 600 /etc/gnetcli/gnetcli.env

# Build gnetcli_server if not exists
if [ ! -f "/root/go/bin/gnetcli_server" ]; then
    print_msg "Building gnetcli_server..." $YELLOW
    cd ../..
    go build -o /root/go/bin/gnetcli_server ./cmd/gnetcli_server
    cd configs/systemd
fi

# Copy systemd service file
print_msg "Installing systemd service..." $YELLOW
cp gnetcli.service /etc/systemd/system/gnetcli.service

# Reload systemd daemon
print_msg "Reloading systemd daemon..." $YELLOW
systemctl daemon-reload

# Enable the service
print_msg "Enabling gnetcli service..." $YELLOW
systemctl enable gnetcli.service

print_msg "Installation complete!" $GREEN
print_msg "You can now configure the service by editing /etc/gnetcli/gnetcli.env" $YELLOW
print_msg "Start the service with: systemctl start gnetcli" $YELLOW
print_msg "Check service status with: systemctl status gnetcli" $YELLOW
print_msg "View logs with: journalctl -u gnetcli -f" $YELLOW