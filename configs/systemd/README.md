# Systemd Service Configuration for Annet Oil

This directory contains systemd service configuration files and management scripts for running Annet Oil and MCP Annet Oil as Linux services.

## Files

- **annet-oil.service** - Systemd service file for the Annet Oil API/SSH server
- **mcp-annet-oil.service** - Systemd service file for the MCP Annet Oil server
- **annet-oil.env** - Environment configuration file for both services
- **install-annet-oil.sh** - Installation script to set up the services
- **manage-services.sh** - Service management utility script

## Installation

### Quick Install

Run the installation script as root:

```bash
sudo ./install-annet-oil.sh
```

This will:
1. Create the service user (`annet`)
2. Create installation directories (`/opt/annet-oil`)
3. Build and copy the binaries
4. Install configuration files
5. Set up systemd services

### Manual Installation

1. Copy files to appropriate locations:
```bash
sudo cp annet-oil.service /etc/systemd/system/
sudo cp mcp-annet-oil.service /etc/systemd/system/
sudo cp annet-oil.env /etc/annet-oil/annet-oil.env
```

2. Create service user:
```bash
sudo useradd -r -s /bin/false -m -d /var/lib/annet annet
```

3. Copy application files:
```bash
sudo mkdir -p /opt/annet-oil
sudo cp -r /path/to/annet-oil/* /opt/annet-oil/
sudo chown -R annet:annet /opt/annet-oil
```

4. Reload systemd and enable services:
```bash
sudo systemctl daemon-reload
sudo systemctl enable annet-oil.service
sudo systemctl enable mcp-annet-oil.service
```

## Configuration

Edit the environment file to configure the services:

```bash
sudo nano /etc/annet-oil/annet-oil.env
```

Important settings to configure:
- `ANNET_OIL_API_TOKEN` - Change to a secure token
- `ANNET_OIL_API_PORT` - API server port (default: 8080)
- `ANNET_OIL_SSH_PORT` - SSH server port (default: 22)
- `DOCKER_HOST` - Docker daemon endpoint

## Service Management

### Using the Management Script

```bash
# Start services
sudo ./manage-services.sh start

# Stop services
sudo ./manage-services.sh stop

# Check status
sudo ./manage-services.sh status

# View logs
sudo ./manage-services.sh logs

# Enable auto-start at boot
sudo ./manage-services.sh enable
```

### Using Systemctl Directly

```bash
# Start services
sudo systemctl start annet-oil
sudo systemctl start mcp-annet-oil

# Stop services
sudo systemctl stop mcp-annet-oil
sudo systemctl stop annet-oil

# Check status
sudo systemctl status annet-oil
sudo systemctl status mcp-annet-oil

# View logs
sudo journalctl -u annet-oil -f
sudo journalctl -u mcp-annet-oil -f

# Enable services
sudo systemctl enable annet-oil
sudo systemctl enable mcp-annet-oil
```

## Directory Structure

After installation:

```
/opt/annet-oil/
├── bin/
│   └── annet-oil           # Main binary
├── configs/
│   └── config.yaml          # Application config
├── storage/
│   └── routing.json         # Routing configuration
└── mcp-annet-oil/
    ├── dist/               # Built Node.js application
    ├── node_modules/       # Dependencies
    └── package.json        # Node.js config

/etc/annet-oil/
└── annet-oil.env           # Environment variables

/etc/systemd/system/
├── annet-oil.service       # Main service
└── mcp-annet-oil.service   # MCP service
```

## Security Notes

1. The services run as the `annet` user (non-root)
2. Services have restricted filesystem access using systemd security features
3. Environment file has restricted permissions (600)
4. Remember to change the default API token in the configuration

## Troubleshooting

### Service won't start

Check logs for errors:
```bash
sudo journalctl -u annet-oil -n 50
sudo journalctl -u mcp-annet-oil -n 50
```

### Permission issues

Ensure proper ownership:
```bash
sudo chown -R annet:annet /opt/annet-oil
sudo chmod 755 /opt/annet-oil/bin/annet-oil
```

### Docker connection issues

Verify Docker socket permissions:
```bash
sudo usermod -aG docker annet
sudo systemctl restart annet-oil
```

### Port conflicts

Check if ports are in use:
```bash
sudo ss -tulpn | grep -E ':(8080|22)'
```

## Uninstallation

To remove the services:

```bash
sudo ./manage-services.sh uninstall
```

Or manually:
```bash
sudo systemctl stop annet-oil mcp-annet-oil
sudo systemctl disable annet-oil mcp-annet-oil
sudo rm /etc/systemd/system/annet-oil.service
sudo rm /etc/systemd/system/mcp-annet-oil.service
sudo systemctl daemon-reload
sudo rm -rf /opt/annet-oil
sudo rm -rf /etc/annet-oil
sudo userdel -r annet
```