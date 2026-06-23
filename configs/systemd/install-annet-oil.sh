#!/bin/bash

# Annet Oil Service Installation Script
# This script installs and configures annet-oil as a systemd service

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/opt/annet-oil"
SERVICE_USER="annet"
SERVICE_GROUP="annet"
SYSTEMD_DIR="/etc/systemd/system"
ENV_DIR="/etc/annet-oil"

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    print_error "Please run this script as root or with sudo"
    exit 1
fi

print_info "Starting Annet Oil service installation..."

# Create service user if it doesn't exist
if ! id "$SERVICE_USER" &>/dev/null; then
    print_info "Creating service user: $SERVICE_USER"
    useradd -r -s /bin/false -m -d /var/lib/annet $SERVICE_USER
else
    print_info "Service user $SERVICE_USER already exists"
fi

# Create installation directory
print_info "Creating installation directory: $INSTALL_DIR"
mkdir -p $INSTALL_DIR
mkdir -p $INSTALL_DIR/bin
mkdir -p $INSTALL_DIR/configs
mkdir -p $INSTALL_DIR/storage
mkdir -p $INSTALL_DIR/mcp-annet-oil

# Create environment configuration directory
print_info "Creating environment configuration directory: $ENV_DIR"
mkdir -p $ENV_DIR

# Check if Go is installed for building
if command -v go &> /dev/null; then
    print_info "Go is installed, building annet-oil..."

    # Build annet-oil binary
    cd "$(dirname "$0")/../.."
    make build

    # Copy binary to installation directory
    cp bin/annet-oil $INSTALL_DIR/bin/
    print_info "Binary copied to $INSTALL_DIR/bin/"
else
    print_warn "Go is not installed. Please build annet-oil manually and copy to $INSTALL_DIR/bin/"
fi

# Copy configuration files
print_info "Copying configuration files..."
cp -r configs/* $INSTALL_DIR/configs/
cp -r storage/* $INSTALL_DIR/storage/ 2>/dev/null || true

# Copy MCP server files
print_info "Copying MCP server files..."
cp -r mcp-annet-oil/* $INSTALL_DIR/mcp-annet-oil/

# Check if Node.js is installed
if command -v node &> /dev/null && command -v npm &> /dev/null; then
    print_info "Building MCP server..."
    cd $INSTALL_DIR/mcp-annet-oil
    npm ci
    npm run build
    cd -
else
    print_warn "Node.js/npm not installed. Please build MCP server manually."
fi

# Set proper permissions
print_info "Setting permissions..."
chown -R $SERVICE_USER:$SERVICE_GROUP $INSTALL_DIR
chmod 755 $INSTALL_DIR/bin/annet-oil

# Copy environment file
print_info "Installing environment configuration..."
cp configs/systemd/annet-oil.env $ENV_DIR/annet-oil.env
chmod 600 $ENV_DIR/annet-oil.env
chown root:root $ENV_DIR/annet-oil.env

# Update systemd service files with environment file
print_info "Installing systemd service files..."

# Create annet-oil service with EnvironmentFile
cat > $SYSTEMD_DIR/annet-oil.service << 'EOF'
[Unit]
Description=Annet Oil Server - Network Configuration Management API
Documentation=https://github.com/yourusername/annet-oil
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
User=annet
Group=annet
WorkingDirectory=/opt/annet-oil
EnvironmentFile=/etc/annet-oil/annet-oil.env
Environment="PATH=/usr/local/bin:/usr/bin:/bin"

# Start command for both API and SSH servers
ExecStart=/opt/annet-oil/bin/annet-oil server start

# Restart policy
Restart=always
RestartSec=5
StartLimitInterval=60s
StartLimitBurst=3

# Process management
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/annet-oil/storage
ReadOnlyPaths=/opt/annet-oil/configs

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=annet-oil

[Install]
WantedBy=multi-user.target
EOF

# Create mcp-annet-oil service with EnvironmentFile
cat > $SYSTEMD_DIR/mcp-annet-oil.service << 'EOF'
[Unit]
Description=MCP Annet Oil - Model Context Protocol Server for Annet Oil
Documentation=https://github.com/yourusername/annet-oil
After=network-online.target annet-oil.service
Wants=network-online.target
Requires=annet-oil.service

[Service]
Type=simple
User=annet
Group=annet
WorkingDirectory=/opt/annet-oil/mcp-annet-oil
EnvironmentFile=/etc/annet-oil/annet-oil.env
Environment="PATH=/usr/local/bin:/usr/bin:/bin"

# Start command
ExecStartPre=/usr/bin/npm ci --production
ExecStart=/usr/bin/node dist/index.js

# Restart policy
Restart=always
RestartSec=5
StartLimitInterval=60s
StartLimitBurst=3

# Process management
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/annet-oil/mcp-annet-oil

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096
MemoryLimit=2G
CPUQuota=200%

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=mcp-annet-oil

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd daemon
print_info "Reloading systemd daemon..."
systemctl daemon-reload

# Enable services
print_info "Enabling services..."
systemctl enable annet-oil.service
systemctl enable mcp-annet-oil.service

print_info "Installation complete!"
print_info ""
print_info "Next steps:"
print_info "1. Edit the configuration file: $ENV_DIR/annet-oil.env"
print_info "2. Update the API token and other settings as needed"
print_info "3. Start the services:"
print_info "   sudo systemctl start annet-oil"
print_info "   sudo systemctl start mcp-annet-oil"
print_info ""
print_info "To check service status:"
print_info "   sudo systemctl status annet-oil"
print_info "   sudo systemctl status mcp-annet-oil"
print_info ""
print_info "To view logs:"
print_info "   sudo journalctl -u annet-oil -f"
print_info "   sudo journalctl -u mcp-annet-oil -f"