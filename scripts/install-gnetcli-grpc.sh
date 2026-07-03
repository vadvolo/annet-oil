#!/bin/bash

# install-gnetcli-grpc.sh
# Script to install and configure gnetcli gRPC server using project configuration files

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Configuration variables
GNETCLI_CONFIG_DIR="/etc/gnetcli"
SYSTEMD_SERVICE_FILE="/etc/systemd/system/gnetcli.service"
GO_BIN_PATH="/root/go/bin"
SOURCE_ENV_FILE="$PROJECT_ROOT/configs/systemd/gnetcli.env"
SOURCE_SERVICE_FILE="$PROJECT_ROOT/configs/systemd/gnetcli.service"

# Function to print colored output
print_msg() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root"
        exit 1
    fi
}

# Check if source files exist
check_source_files() {
    if [[ ! -f "$SOURCE_ENV_FILE" ]]; then
        print_error "Source environment file not found: $SOURCE_ENV_FILE"
        exit 1
    fi

    if [[ ! -f "$SOURCE_SERVICE_FILE" ]]; then
        print_error "Source service file not found: $SOURCE_SERVICE_FILE"
        exit 1
    fi

    print_msg "Found source configuration files"
}

# Check and install Go if needed
install_go() {
    if command -v go &> /dev/null; then
        print_msg "Go is already installed: $(go version)"
    else
        print_msg "Installing Go..."

        # Detect OS
        if [[ "$OSTYPE" == "linux-gnu"* ]]; then
            # Check if apt is available (Debian/Ubuntu)
            if command -v apt &> /dev/null; then
                apt update
                apt install -y golang-go
            # Check if yum is available (RHEL/CentOS)
            elif command -v yum &> /dev/null; then
                yum install -y golang
            else
                print_error "Unsupported Linux distribution. Please install Go manually."
                exit 1
            fi
        elif [[ "$OSTYPE" == "darwin"* ]]; then
            if command -v brew &> /dev/null; then
                brew install go
            else
                print_error "Homebrew not found. Please install Go manually."
                exit 1
            fi
        else
            print_error "Unsupported OS. Please install Go manually."
            exit 1
        fi
    fi
}

# Install gnetcli_server binary
install_gnetcli_server() {
    print_msg "Installing gnetcli_server..."

    # Set GOPATH for root user
    export GOPATH=/root/go
    export PATH=$PATH:$GOPATH/bin

    # Install the server
    go install github.com/annetutil/gnetcli/cmd/gnetcli_server@latest

    if [[ -f "$GO_BIN_PATH/gnetcli_server" ]]; then
        print_msg "gnetcli_server installed successfully at $GO_BIN_PATH/gnetcli_server"
        chmod +x "$GO_BIN_PATH/gnetcli_server"
    else
        print_error "Failed to install gnetcli_server"
        exit 1
    fi
}

# Create configuration directory and copy environment file
setup_config() {
    print_msg "Creating configuration directory..."
    mkdir -p "$GNETCLI_CONFIG_DIR"

    # Check if environment file already exists
    if [[ -f "$GNETCLI_CONFIG_DIR/gnetcli.env" ]]; then
        print_warning "Environment file already exists at $GNETCLI_CONFIG_DIR/gnetcli.env"
        read -p "Do you want to overwrite it? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_msg "Keeping existing configuration file"
            return
        fi
    fi

    print_msg "Copying environment configuration file..."
    cp "$SOURCE_ENV_FILE" "$GNETCLI_CONFIG_DIR/gnetcli.env"

    # Secure the file
    chmod 600 "$GNETCLI_CONFIG_DIR/gnetcli.env"

    print_msg "Environment file copied to $GNETCLI_CONFIG_DIR/gnetcli.env"
    print_warning "Please edit $GNETCLI_CONFIG_DIR/gnetcli.env and set your credentials:"
    print_warning "  - GNETCLI_LOGIN: Set your gRPC server username"
    print_warning "  - GNETCLI_PASSWORD: Set your gRPC server password"
    print_warning "  - DEVICE_LOGIN: Set default device username"
    print_warning "  - DEVICE_PASSWORD: Set default device password"
}

# Copy systemd service file
setup_systemd_service() {
    print_msg "Setting up systemd service file..."

    if [[ -f "$SYSTEMD_SERVICE_FILE" ]]; then
        print_warning "Service file already exists at $SYSTEMD_SERVICE_FILE"
        read -p "Do you want to overwrite it? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_msg "Keeping existing service file"
            return
        fi
    fi

    print_msg "Copying systemd service file..."
    cp "$SOURCE_SERVICE_FILE" "$SYSTEMD_SERVICE_FILE"

    print_msg "Service file copied to $SYSTEMD_SERVICE_FILE"
}

# Configure credentials
configure_credentials() {
    print_msg ""
    print_msg "="
    print_warning "IMPORTANT: Configure your credentials"
    print_msg "="

    read -p "Do you want to configure credentials now? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_warning "Skipping credential configuration. Remember to edit $GNETCLI_CONFIG_DIR/gnetcli.env before starting the service!"
        return
    fi

    # Read current values
    source "$GNETCLI_CONFIG_DIR/gnetcli.env"

    # Get gRPC server credentials
    read -p "Enter gRPC server username [$GNETCLI_LOGIN]: " new_login
    new_login=${new_login:-$GNETCLI_LOGIN}

    read -s -p "Enter gRPC server password: " new_password
    echo
    if [[ -z "$new_password" ]]; then
        new_password=$(openssl rand -base64 16)
        print_warning "Generated random password: $new_password"
    fi

    # Get device credentials
    read -p "Enter default device username [$DEVICE_LOGIN]: " new_device_login
    new_device_login=${new_device_login:-$DEVICE_LOGIN}

    read -s -p "Enter default device password: " new_device_password
    echo
    new_device_password=${new_device_password:-$DEVICE_PASSWORD}

    # Update the configuration file
    sed -i "s/^GNETCLI_LOGIN=.*/GNETCLI_LOGIN=$new_login/" "$GNETCLI_CONFIG_DIR/gnetcli.env"
    sed -i "s/^GNETCLI_PASSWORD=.*/GNETCLI_PASSWORD=$new_password/" "$GNETCLI_CONFIG_DIR/gnetcli.env"
    sed -i "s/^DEVICE_LOGIN=.*/DEVICE_LOGIN=$new_device_login/" "$GNETCLI_CONFIG_DIR/gnetcli.env"
    sed -i "s/^DEVICE_PASSWORD=.*/DEVICE_PASSWORD=$new_device_password/" "$GNETCLI_CONFIG_DIR/gnetcli.env"

    print_msg "Credentials updated successfully"
}

# Start and enable the service
start_service() {
    print_msg "Configuring systemd service..."

    # Reload systemd
    systemctl daemon-reload

    # Enable the service
    systemctl enable gnetcli.service

    # Check if service is already running
    if systemctl is-active --quiet gnetcli.service; then
        print_warning "gnetcli service is already running"
        read -p "Do you want to restart it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            systemctl restart gnetcli.service
            print_msg "Service restarted"
        fi
    else
        # Check if credentials are configured
        source "$GNETCLI_CONFIG_DIR/gnetcli.env"
        if [[ "$GNETCLI_LOGIN" == "GNETCLI_LOGIN" ]] || [[ "$GNETCLI_PASSWORD" == "GNETCLI_PASS" ]]; then
            print_warning "Default credentials detected. Service will not be started."
            print_warning "Please configure credentials in $GNETCLI_CONFIG_DIR/gnetcli.env"
            print_warning "Then start the service with: systemctl start gnetcli"
            return
        fi

        systemctl start gnetcli.service
        print_msg "Service started"
    fi

    # Wait a moment for service to start
    sleep 2

    # Check service status
    if systemctl is-active --quiet gnetcli.service; then
        print_msg "gnetcli service is running successfully"
    else
        print_error "Failed to start gnetcli service"
        print_msg "Check logs with: journalctl -u gnetcli -n 50"
    fi
}

# Test the installation
test_installation() {
    print_msg "Testing gnetcli server installation..."

    # Load environment variables
    source "$GNETCLI_CONFIG_DIR/gnetcli.env"

    # Check if service is running
    if ! systemctl is-active --quiet gnetcli.service; then
        print_warning "gnetcli service is not running. Skipping tests."
        return
    fi

    # Test HTTP health endpoint
    if command -v curl &> /dev/null; then
        print_msg "Testing HTTP health endpoint..."
        if curl -s -f "http://localhost:${GNETCLI_HTTP_PORT}/health" > /dev/null 2>&1; then
            print_msg "HTTP health check passed"
        else
            print_warning "HTTP health check failed (this might be normal if the endpoint is not implemented)"
        fi
    fi

    # Test gRPC connectivity
    if command -v grpcurl &> /dev/null; then
        print_msg "Testing gRPC connectivity..."
        if grpcurl -plaintext "localhost:${GNETCLI_GRPC_PORT}" list > /dev/null 2>&1; then
            print_msg "gRPC connectivity test passed"
        else
            print_warning "gRPC connectivity test failed (this might be normal if reflection is not enabled)"
        fi
    else
        print_msg "grpcurl not found, skipping gRPC connectivity test"
        print_msg "Install grpcurl for testing: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest"
    fi
}

# Install Python client
install_python_client() {
    print_msg "Checking for gnetcli Python client..."

    # Check if gnetcli is already installed
    if command -v gnetcli &> /dev/null; then
        print_msg "gnetcli client is already installed: $(gnetcli --version 2>/dev/null || echo 'version unknown')"
        read -p "Do you want to upgrade it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            if command -v pip3 &> /dev/null; then
                pip3 install --upgrade gnetcli
                print_msg "gnetcli client upgraded successfully"
            elif command -v pip &> /dev/null; then
                pip install --upgrade gnetcli
                print_msg "gnetcli client upgraded successfully"
            fi
        fi
    else
        print_msg "gnetcli client not found. Installing..."

        # Try to install automatically
        if command -v pip3 &> /dev/null; then
            pip3 install gnetcli
            if command -v gnetcli &> /dev/null; then
                print_msg "Python gnetcli client installed successfully"
            else
                # Try installing with --break-system-packages for newer systems
                print_msg "Retrying installation with system packages flag..."
                pip3 install --break-system-packages gnetcli 2>/dev/null || pip3 install --user gnetcli
                print_msg "Python gnetcli client installed (user mode)"
            fi
        elif command -v pip &> /dev/null; then
            pip install gnetcli
            if command -v gnetcli &> /dev/null; then
                print_msg "Python gnetcli client installed successfully"
            else
                pip install --user gnetcli
                print_msg "Python gnetcli client installed (user mode)"
            fi
        else
            print_warning "pip not found. Installing python3-pip..."
            if command -v apt &> /dev/null; then
                apt update && apt install -y python3-pip
                pip3 install gnetcli
                print_msg "Python gnetcli client installed successfully"
            elif command -v yum &> /dev/null; then
                yum install -y python3-pip
                pip3 install gnetcli
                print_msg "Python gnetcli client installed successfully"
            else
                print_error "Cannot install pip automatically. Please install manually:"
                print_error "  sudo apt install python3-pip  # For Debian/Ubuntu"
                print_error "  sudo yum install python3-pip  # For RHEL/CentOS"
                print_error "Then run: pip3 install gnetcli"
            fi
        fi
    fi

    # Check if gnetcli is in PATH
    if ! command -v gnetcli &> /dev/null; then
        print_warning "gnetcli installed but not in PATH. You may need to:"
        print_warning "  - Add ~/.local/bin to your PATH"
        print_warning "  - Or logout and login again"
        print_warning "  - Or run: export PATH=\$PATH:~/.local/bin"
    fi
}

# Print configuration summary
print_summary() {
    print_msg "="
    print_msg "Installation completed!"
    print_msg "="
    print_msg ""
    print_msg "Configuration:"
    print_msg "  Config directory: $GNETCLI_CONFIG_DIR"
    print_msg "  Environment file: $GNETCLI_CONFIG_DIR/gnetcli.env"
    print_msg "  Service file: $SYSTEMD_SERVICE_FILE"
    print_msg "  Server binary: $GO_BIN_PATH/gnetcli_server"
    print_msg ""

    # Load and display connection info
    source "$GNETCLI_CONFIG_DIR/gnetcli.env"
    print_msg "Server endpoints:"
    print_msg "  gRPC: 0.0.0.0:${GNETCLI_GRPC_PORT}"
    print_msg "  HTTP: 0.0.0.0:${GNETCLI_HTTP_PORT}"
    print_msg ""

    if systemctl is-active --quiet gnetcli.service; then
        print_msg "Service status: ${GREEN}Running${NC}"
    else
        print_msg "Service status: ${YELLOW}Stopped${NC}"
        print_msg "Start with: systemctl start gnetcli"
    fi

    print_msg ""
    print_msg "Service management commands:"
    print_msg "  Start:   systemctl start gnetcli"
    print_msg "  Stop:    systemctl stop gnetcli"
    print_msg "  Restart: systemctl restart gnetcli"
    print_msg "  Status:  systemctl status gnetcli"
    print_msg "  Logs:    journalctl -u gnetcli -f"
    print_msg ""

    if [[ "$GNETCLI_LOGIN" != "GNETCLI_LOGIN" ]]; then
        print_msg "Client usage example:"
        print_msg "  gnetcli -s localhost:${GNETCLI_GRPC_PORT} -u ${GNETCLI_LOGIN} -p <password> \\"
        print_msg "          -H device.example.com -d <device_user> -P <device_pass> \"show version\""
    else
        print_warning "Remember to configure credentials in $GNETCLI_CONFIG_DIR/gnetcli.env"
    fi
}

# Main installation flow
main() {
    print_msg "Starting gnetcli gRPC server installation..."
    print_msg "Using configuration from: $PROJECT_ROOT/configs/systemd/"

    check_root
    check_source_files
    install_go
    install_gnetcli_server
    setup_config
    configure_credentials
    setup_systemd_service
    start_service
    test_installation
    install_python_client
    print_summary
}

# Handle command line arguments
case "${1:-}" in
    uninstall)
        print_msg "Uninstalling gnetcli gRPC server..."
        check_root
        systemctl stop gnetcli.service 2>/dev/null || true
        systemctl disable gnetcli.service 2>/dev/null || true
        rm -f "$SYSTEMD_SERVICE_FILE"
        rm -rf "$GNETCLI_CONFIG_DIR"
        rm -f "$GO_BIN_PATH/gnetcli_server"
        systemctl daemon-reload
        print_msg "Uninstallation completed"
        ;;
    status)
        systemctl status gnetcli.service
        ;;
    test)
        check_source_files
        test_installation
        ;;
    config)
        check_root
        check_source_files
        configure_credentials
        ;;
    *)
        main
        ;;
esac