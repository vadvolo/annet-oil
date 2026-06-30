#!/bin/bash

# Gnetcli Server Uninstall Script
# This script removes gnetcli_server systemd service

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

print_msg "Starting gnetcli server removal..." $YELLOW

# Stop the service if running
if systemctl is-active --quiet gnetcli; then
    print_msg "Stopping gnetcli service..." $YELLOW
    systemctl stop gnetcli
fi

# Disable the service
if systemctl is-enabled --quiet gnetcli; then
    print_msg "Disabling gnetcli service..." $YELLOW
    systemctl disable gnetcli
fi

# Remove systemd service file
if [ -f /etc/systemd/system/gnetcli.service ]; then
    print_msg "Removing systemd service..." $YELLOW
    rm -f /etc/systemd/system/gnetcli.service
fi

# Reload systemd daemon
print_msg "Reloading systemd daemon..." $YELLOW
systemctl daemon-reload
systemctl reset-failed

# Remove configuration files
if [ -d /etc/gnetcli ]; then
    print_msg "Removing configuration files..." $YELLOW
    rm -rf /etc/gnetcli
fi

print_msg "Uninstallation complete!" $GREEN
print_msg "Note: The gnetcli_server binary was not removed from /root/go/bin/" $YELLOW