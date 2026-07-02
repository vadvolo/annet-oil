#!/bin/bash

# Annet Oil Service Installation Script
# This script installs and configures annet-oil as a systemd service

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Configuration
INSTALL_DIR="/opt/annet-oil"
SERVICE_USER="annet"
SERVICE_GROUP="annet"
SYSTEMD_DIR="/etc/systemd/system"
ENV_DIR="/etc/annet-oil"

# Source files from project
SOURCE_ENV_FILE="$PROJECT_ROOT/configs/systemd/annet-oil.env"
SOURCE_SERVICE_FILE="$PROJECT_ROOT/configs/systemd/annet-oil.service"
SOURCE_MCP_SERVICE_FILE="$PROJECT_ROOT/configs/systemd/mcp-annet-oil.service"

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
check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "Please run this script as root or with sudo"
        exit 1
    fi
}

# Check if source files exist
check_source_files() {
    local missing_files=0

    if [[ ! -f "$SOURCE_ENV_FILE" ]]; then
        print_error "Source environment file not found: $SOURCE_ENV_FILE"
        missing_files=1
    fi

    if [[ ! -f "$SOURCE_SERVICE_FILE" ]]; then
        print_error "Source service file not found: $SOURCE_SERVICE_FILE"
        missing_files=1
    fi

    # MCP service file is optional
    if [[ ! -f "$SOURCE_MCP_SERVICE_FILE" ]]; then
        print_warn "MCP service file not found: $SOURCE_MCP_SERVICE_FILE (optional)"
    fi

    if [[ $missing_files -eq 1 ]]; then
        exit 1
    fi

    print_info "Found source configuration files"
}

# Create service user
create_user() {
    if ! id "$SERVICE_USER" &>/dev/null; then
        print_info "Creating service user: $SERVICE_USER"
        useradd -r -s /bin/false -m -d /var/lib/annet $SERVICE_USER
    else
        print_info "Service user $SERVICE_USER already exists"
    fi
}

# Create directory structure
create_directories() {
    print_info "Creating installation directory: $INSTALL_DIR"
    mkdir -p $INSTALL_DIR
    mkdir -p $INSTALL_DIR/bin
    mkdir -p $INSTALL_DIR/configs
    mkdir -p $INSTALL_DIR/storage
    mkdir -p $INSTALL_DIR/mcp-annet-oil

    print_info "Creating environment configuration directory: $ENV_DIR"
    mkdir -p $ENV_DIR
}

# Build and install annet-oil binary
install_binary() {
    # Check if Go is installed for building
    if command -v go &> /dev/null; then
        print_info "Go is installed, building annet-oil..."

        # Build annet-oil binary
        cd "$PROJECT_ROOT"
        if [[ -f "Makefile" ]]; then
            make build
        elif [[ -f "go.mod" ]]; then
            go build -o bin/annet-oil ./cmd/annet-oil
        else
            print_warn "Cannot find build configuration. Please build manually."
            return
        fi

        # Copy binary to installation directory
        if [[ -f "bin/annet-oil" ]]; then
            cp bin/annet-oil $INSTALL_DIR/bin/
            chmod 755 $INSTALL_DIR/bin/annet-oil
            print_info "Binary copied to $INSTALL_DIR/bin/"
        elif [[ -f "annet-oil-server" ]]; then
            cp annet-oil-server $INSTALL_DIR/bin/annet-oil
            chmod 755 $INSTALL_DIR/bin/annet-oil
            print_info "Binary copied to $INSTALL_DIR/bin/"
        else
            print_warn "Binary not found. Please build annet-oil manually and copy to $INSTALL_DIR/bin/"
        fi
        cd -
    else
        print_warn "Go is not installed. Please build annet-oil manually and copy to $INSTALL_DIR/bin/"
    fi
}

# Copy project files
copy_project_files() {
    print_info "Copying configuration files..."

    # Copy configs if they exist
    if [[ -d "$PROJECT_ROOT/configs" ]]; then
        cp -r "$PROJECT_ROOT/configs/"* $INSTALL_DIR/configs/ 2>/dev/null || true
    fi

    # Copy storage templates if they exist
    if [[ -d "$PROJECT_ROOT/storage" ]]; then
        cp -r "$PROJECT_ROOT/storage/"* $INSTALL_DIR/storage/ 2>/dev/null || true
    fi

    # Copy MCP server files if they exist
    if [[ -d "$PROJECT_ROOT/mcp-annet-oil" ]]; then
        print_info "Copying MCP server files..."
        cp -r "$PROJECT_ROOT/mcp-annet-oil/"* $INSTALL_DIR/mcp-annet-oil/

        # Build MCP server if Node.js is available
        if command -v node &> /dev/null && command -v npm &> /dev/null; then
            print_info "Building MCP server..."
            cd $INSTALL_DIR/mcp-annet-oil
            npm ci
            npm run build
            cd -
        else
            print_warn "Node.js/npm not installed. Please build MCP server manually."
        fi
    fi
}

# Install environment file
install_env_file() {
    print_info "Installing environment configuration..."

    if [[ -f "$ENV_DIR/annet-oil.env" ]]; then
        print_warn "Environment file already exists at $ENV_DIR/annet-oil.env"
        read -p "Do you want to overwrite it? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Keeping existing environment file"
            return
        fi
    fi

    cp "$SOURCE_ENV_FILE" "$ENV_DIR/annet-oil.env"
    chmod 600 "$ENV_DIR/annet-oil.env"
    chown root:root "$ENV_DIR/annet-oil.env"

    print_warn "Please edit $ENV_DIR/annet-oil.env and update:"
    print_warn "  - API_TOKEN: Set your API authentication token"
    print_warn "  - SSH_ENABLED: Enable/disable SSH server"
    print_warn "  - Other settings as needed"
}

# Install systemd service files
install_systemd_services() {
    print_info "Installing systemd service files..."

    # Install annet-oil service
    if [[ -f "$SYSTEMD_DIR/annet-oil.service" ]]; then
        print_warn "Service file already exists at $SYSTEMD_DIR/annet-oil.service"
        read -p "Do you want to overwrite it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            cp "$SOURCE_SERVICE_FILE" "$SYSTEMD_DIR/annet-oil.service"
            print_info "Annet-oil service file installed"
        else
            print_info "Keeping existing annet-oil service file"
        fi
    else
        cp "$SOURCE_SERVICE_FILE" "$SYSTEMD_DIR/annet-oil.service"
        print_info "Annet-oil service file installed"
    fi

    # Install MCP service if source file exists
    if [[ -f "$SOURCE_MCP_SERVICE_FILE" ]]; then
        if [[ -f "$SYSTEMD_DIR/mcp-annet-oil.service" ]]; then
            print_warn "MCP service file already exists at $SYSTEMD_DIR/mcp-annet-oil.service"
            read -p "Do you want to overwrite it? (y/N): " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                cp "$SOURCE_MCP_SERVICE_FILE" "$SYSTEMD_DIR/mcp-annet-oil.service"
                print_info "MCP service file installed"
            else
                print_info "Keeping existing MCP service file"
            fi
        else
            cp "$SOURCE_MCP_SERVICE_FILE" "$SYSTEMD_DIR/mcp-annet-oil.service"
            print_info "MCP service file installed"
        fi
    fi
}

# Set permissions
set_permissions() {
    print_info "Setting permissions..."
    chown -R $SERVICE_USER:$SERVICE_GROUP $INSTALL_DIR

    # Ensure binary is executable
    if [[ -f "$INSTALL_DIR/bin/annet-oil" ]]; then
        chmod 755 $INSTALL_DIR/bin/annet-oil
    fi
}

# Configure systemd
configure_systemd() {
    print_info "Reloading systemd daemon..."
    systemctl daemon-reload

    print_info "Enabling services..."
    systemctl enable annet-oil.service

    if [[ -f "$SYSTEMD_DIR/mcp-annet-oil.service" ]]; then
        systemctl enable mcp-annet-oil.service
    fi
}

# Start services
start_services() {
    read -p "Do you want to start the services now? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Check if environment file is configured
        if grep -q "YOUR_API_TOKEN_HERE" "$ENV_DIR/annet-oil.env" 2>/dev/null; then
            print_warn "Default API token detected. Services will not be started."
            print_warn "Please configure $ENV_DIR/annet-oil.env first."
            return
        fi

        print_info "Starting annet-oil service..."
        systemctl start annet-oil.service

        if [[ -f "$SYSTEMD_DIR/mcp-annet-oil.service" ]]; then
            print_info "Starting MCP service..."
            systemctl start mcp-annet-oil.service
        fi

        sleep 2

        # Check status
        if systemctl is-active --quiet annet-oil.service; then
            print_info "annet-oil service is running successfully"
        else
            print_error "Failed to start annet-oil service"
            print_info "Check logs with: journalctl -u annet-oil -n 50"
        fi
    fi
}

# Print summary
print_summary() {
    print_info ""
    print_info "========================================="
    print_info "Installation complete!"
    print_info "========================================="
    print_info ""
    print_info "Installation paths:"
    print_info "  Installation directory: $INSTALL_DIR"
    print_info "  Environment file: $ENV_DIR/annet-oil.env"
    print_info "  Service files: $SYSTEMD_DIR/annet-oil.service"

    if [[ -f "$SYSTEMD_DIR/mcp-annet-oil.service" ]]; then
        print_info "                 $SYSTEMD_DIR/mcp-annet-oil.service"
    fi

    print_info ""
    print_info "Next steps:"
    print_info "1. Edit the configuration file: $ENV_DIR/annet-oil.env"
    print_info "2. Update the API token and other settings as needed"
    print_info "3. Start the services:"
    print_info "   sudo systemctl start annet-oil"

    if [[ -f "$SYSTEMD_DIR/mcp-annet-oil.service" ]]; then
        print_info "   sudo systemctl start mcp-annet-oil"
    fi

    print_info ""
    print_info "Service management commands:"
    print_info "  Status:  systemctl status annet-oil"
    print_info "  Start:   systemctl start annet-oil"
    print_info "  Stop:    systemctl stop annet-oil"
    print_info "  Restart: systemctl restart annet-oil"
    print_info "  Logs:    journalctl -u annet-oil -f"

    if [[ -f "$SYSTEMD_DIR/mcp-annet-oil.service" ]]; then
        print_info ""
        print_info "MCP service commands:"
        print_info "  Status:  systemctl status mcp-annet-oil"
        print_info "  Start:   systemctl start mcp-annet-oil"
        print_info "  Stop:    systemctl stop mcp-annet-oil"
        print_info "  Restart: systemctl restart mcp-annet-oil"
        print_info "  Logs:    journalctl -u mcp-annet-oil -f"
    fi
}

# Uninstall function
uninstall() {
    print_warn "This will uninstall annet-oil and remove all configuration"
    read -p "Are you sure you want to continue? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Uninstall cancelled"
        exit 0
    fi

    print_info "Stopping and disabling services..."
    systemctl stop annet-oil.service 2>/dev/null || true
    systemctl stop mcp-annet-oil.service 2>/dev/null || true
    systemctl disable annet-oil.service 2>/dev/null || true
    systemctl disable mcp-annet-oil.service 2>/dev/null || true

    print_info "Removing service files..."
    rm -f "$SYSTEMD_DIR/annet-oil.service"
    rm -f "$SYSTEMD_DIR/mcp-annet-oil.service"

    print_info "Removing installation directory..."
    rm -rf "$INSTALL_DIR"

    print_info "Removing environment configuration..."
    rm -rf "$ENV_DIR"

    print_info "Reloading systemd daemon..."
    systemctl daemon-reload

    print_info "Uninstallation complete"

    print_warn "Note: Service user '$SERVICE_USER' was not removed"
    print_warn "To remove it, run: userdel -r $SERVICE_USER"
}

# Main installation flow
main() {
    print_info "Starting Annet Oil service installation..."
    print_info "Using configuration from: $PROJECT_ROOT/configs/systemd/"

    check_root
    check_source_files
    create_user
    create_directories
    install_binary
    copy_project_files
    install_env_file
    install_systemd_services
    set_permissions
    configure_systemd
    start_services
    print_summary
}

# Handle command line arguments
case "${1:-}" in
    uninstall)
        check_root
        uninstall
        ;;
    status)
        systemctl status annet-oil.service
        echo
        if [[ -f "$SYSTEMD_DIR/mcp-annet-oil.service" ]]; then
            systemctl status mcp-annet-oil.service
        fi
        ;;
    restart)
        check_root
        systemctl restart annet-oil.service
        if [[ -f "$SYSTEMD_DIR/mcp-annet-oil.service" ]]; then
            systemctl restart mcp-annet-oil.service
        fi
        print_info "Services restarted"
        ;;
    stop)
        check_root
        systemctl stop annet-oil.service
        if [[ -f "$SYSTEMD_DIR/mcp-annet-oil.service" ]]; then
            systemctl stop mcp-annet-oil.service
        fi
        print_info "Services stopped"
        ;;
    start)
        check_root
        systemctl start annet-oil.service
        if [[ -f "$SYSTEMD_DIR/mcp-annet-oil.service" ]]; then
            systemctl start mcp-annet-oil.service
        fi
        print_info "Services started"
        ;;
    *)
        main
        ;;
esac