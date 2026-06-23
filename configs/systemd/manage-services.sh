#!/bin/bash

# Service Management Script for Annet Oil and MCP Server
# Provides easy commands to manage the services

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Service names
ANNET_SERVICE="annet-oil.service"
MCP_SERVICE="mcp-annet-oil.service"

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

print_status() {
    echo -e "${BLUE}[STATUS]${NC} $1"
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "Please run this script as root or with sudo"
        exit 1
    fi
}

# Function to show usage
show_usage() {
    echo "Usage: $0 {start|stop|restart|status|logs|enable|disable|install|uninstall}"
    echo ""
    echo "Commands:"
    echo "  start      - Start both services"
    echo "  stop       - Stop both services"
    echo "  restart    - Restart both services"
    echo "  status     - Show status of both services"
    echo "  logs       - Show logs for both services (tail -f)"
    echo "  enable     - Enable services to start at boot"
    echo "  disable    - Disable services from starting at boot"
    echo "  install    - Run the installation script"
    echo "  uninstall  - Remove services and optionally remove files"
    echo ""
    echo "Service-specific commands:"
    echo "  start-annet    - Start only annet-oil service"
    echo "  start-mcp      - Start only mcp-annet-oil service"
    echo "  stop-annet     - Stop only annet-oil service"
    echo "  stop-mcp       - Stop only mcp-annet-oil service"
    echo "  logs-annet     - Show logs for annet-oil only"
    echo "  logs-mcp       - Show logs for mcp-annet-oil only"
}

# Start services
start_services() {
    check_root
    print_info "Starting Annet Oil services..."

    systemctl start $ANNET_SERVICE
    print_status "Annet Oil API server started"

    sleep 2  # Wait for API to be ready

    systemctl start $MCP_SERVICE
    print_status "MCP Annet Oil server started"

    print_info "All services started successfully"
}

# Stop services
stop_services() {
    check_root
    print_info "Stopping Annet Oil services..."

    systemctl stop $MCP_SERVICE
    print_status "MCP Annet Oil server stopped"

    systemctl stop $ANNET_SERVICE
    print_status "Annet Oil API server stopped"

    print_info "All services stopped successfully"
}

# Restart services
restart_services() {
    check_root
    print_info "Restarting Annet Oil services..."
    stop_services
    sleep 2
    start_services
}

# Show status
show_status() {
    print_info "Service Status:"
    echo ""
    echo "=== Annet Oil API Server ==="
    systemctl status $ANNET_SERVICE --no-pager || true
    echo ""
    echo "=== MCP Annet Oil Server ==="
    systemctl status $MCP_SERVICE --no-pager || true
}

# Show logs
show_logs() {
    print_info "Showing logs (press Ctrl+C to exit)..."
    journalctl -u $ANNET_SERVICE -u $MCP_SERVICE -f
}

# Show logs for specific service
show_logs_annet() {
    print_info "Showing Annet Oil logs (press Ctrl+C to exit)..."
    journalctl -u $ANNET_SERVICE -f
}

show_logs_mcp() {
    print_info "Showing MCP Annet Oil logs (press Ctrl+C to exit)..."
    journalctl -u $MCP_SERVICE -f
}

# Enable services
enable_services() {
    check_root
    print_info "Enabling services to start at boot..."
    systemctl enable $ANNET_SERVICE
    systemctl enable $MCP_SERVICE
    print_info "Services enabled successfully"
}

# Disable services
disable_services() {
    check_root
    print_info "Disabling services from starting at boot..."
    systemctl disable $MCP_SERVICE
    systemctl disable $ANNET_SERVICE
    print_info "Services disabled successfully"
}

# Install services
install_services() {
    check_root
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    if [ -f "$SCRIPT_DIR/install-annet-oil.sh" ]; then
        print_info "Running installation script..."
        bash "$SCRIPT_DIR/install-annet-oil.sh"
    else
        print_error "Installation script not found: $SCRIPT_DIR/install-annet-oil.sh"
        exit 1
    fi
}

# Uninstall services
uninstall_services() {
    check_root

    print_warn "This will stop and remove the Annet Oil services."
    read -p "Are you sure you want to continue? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Uninstall cancelled"
        exit 0
    fi

    # Stop services if running
    systemctl stop $MCP_SERVICE 2>/dev/null || true
    systemctl stop $ANNET_SERVICE 2>/dev/null || true

    # Disable services
    systemctl disable $MCP_SERVICE 2>/dev/null || true
    systemctl disable $ANNET_SERVICE 2>/dev/null || true

    # Remove service files
    rm -f /etc/systemd/system/$ANNET_SERVICE
    rm -f /etc/systemd/system/$MCP_SERVICE

    # Reload systemd
    systemctl daemon-reload

    print_info "Services removed successfully"

    read -p "Do you want to remove the installation directory /opt/annet-oil? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf /opt/annet-oil
        print_info "Installation directory removed"
    fi

    read -p "Do you want to remove the configuration directory /etc/annet-oil? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf /etc/annet-oil
        print_info "Configuration directory removed"
    fi

    read -p "Do you want to remove the service user 'annet'? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        userdel -r annet 2>/dev/null || true
        print_info "Service user removed"
    fi

    print_info "Uninstall completed"
}

# Start specific service
start_annet() {
    check_root
    print_info "Starting Annet Oil API server..."
    systemctl start $ANNET_SERVICE
    print_status "Annet Oil API server started"
}

start_mcp() {
    check_root
    print_info "Starting MCP Annet Oil server..."
    systemctl start $MCP_SERVICE
    print_status "MCP Annet Oil server started"
}

# Stop specific service
stop_annet() {
    check_root
    print_info "Stopping Annet Oil API server..."
    systemctl stop $ANNET_SERVICE
    print_status "Annet Oil API server stopped"
}

stop_mcp() {
    check_root
    print_info "Stopping MCP Annet Oil server..."
    systemctl stop $MCP_SERVICE
    print_status "MCP Annet Oil server stopped"
}

# Main script logic
case "$1" in
    start)
        start_services
        ;;
    stop)
        stop_services
        ;;
    restart)
        restart_services
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs
        ;;
    logs-annet)
        show_logs_annet
        ;;
    logs-mcp)
        show_logs_mcp
        ;;
    enable)
        enable_services
        ;;
    disable)
        disable_services
        ;;
    install)
        install_services
        ;;
    uninstall)
        uninstall_services
        ;;
    start-annet)
        start_annet
        ;;
    start-mcp)
        start_mcp
        ;;
    stop-annet)
        stop_annet
        ;;
    stop-mcp)
        stop_mcp
        ;;
    *)
        show_usage
        exit 1
        ;;
esac

exit 0