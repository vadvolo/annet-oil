# How to Install gnetcli

## gRPC Mode Installation

### Installing gnetcli Server

For production use, gnetcli runs as a gRPC server, providing better performance and connection pooling for multiple concurrent device connections.

### Automated Installation (Recommended)

Use the provided installation script for automated setup:

```bash
# Clone the repository or download the script
git clone https://github.com/yourusername/annet-oil.git
cd annet-oil

# Run the installation script
sudo ./scripts/install-gnetcli-grpc.sh
```

The script provides several commands:

```bash
# Full installation with interactive configuration
sudo ./scripts/install-gnetcli-grpc.sh

# Reconfigure credentials after installation
sudo ./scripts/install-gnetcli-grpc.sh config

# Check service status
sudo ./scripts/install-gnetcli-grpc.sh status

# Test the installation
sudo ./scripts/install-gnetcli-grpc.sh test

# Uninstall completely
sudo ./scripts/install-gnetcli-grpc.sh uninstall
```

The automated installer will:
- Install Go if not present
- Install the gnetcli_server binary
- Copy configuration files from `configs/systemd/`
- Configure credentials interactively
- Set up and start the systemd service
- Test the installation
- Optionally install the Python client

### Manual Installation

If you prefer manual installation, follow these steps:

#### Step 1: Install gnetcli_server binary

The gnetcli_server is a Go-based gRPC server. Install it using Go:

```bash
# Install Go if not already installed
sudo apt update && sudo apt install golang-go  # Debian/Ubuntu
# or
brew install go  # macOS

# Install gnetcli_server
go install github.com/annetutil/gnetcli/cmd/gnetcli_server@latest

# Verify installation
$HOME/go/bin/gnetcli_server --version
```

For root user installation (as shown in the systemd service):

```bash
sudo su -
go install github.com/annetutil/gnetcli/cmd/gnetcli_server@latest
exit
```

#### Step 2: Create configuration directory and environment file

Create the gnetcli configuration directory and environment file:

```bash
# Create configuration directory
sudo mkdir -p /etc/gnetcli

# Create environment file
sudo cat > /etc/gnetcli/gnetcli.env << 'EOF'
# /etc/gnetcli/gnetcli.env
# gnetcli_server settings

GNETCLI_GRPC_PORT=50051
GNETCLI_HTTP_PORT=50052

# Basic auth for gnetcli server clients
GNETCLI_LOGIN=your_gnetcli_user
GNETCLI_PASSWORD=your_secure_password

# Default device credentials (optional, can be passed per-request)
DEVICE_LOGIN=device_admin
DEVICE_PASSWORD=device_password
EOF

# Secure the environment file
sudo chmod 600 /etc/gnetcli/gnetcli.env
```

#### Step 3: Create systemd service

Create the systemd service file `/etc/systemd/system/gnetcli.service`:

```bash
sudo cat > /etc/systemd/system/gnetcli.service << 'EOF'
[Unit]
Description=Gnetcli gRPC Server
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=/root

# Load environment variables from .env file
EnvironmentFile=/etc/gnetcli/gnetcli.env

ExecStart=/root/go/bin/gnetcli_server \
    -port 0.0.0.0:${GNETCLI_GRPC_PORT} \
    -http_port 0.0.0.0:${GNETCLI_HTTP_PORT} \
    -basic-auth ${GNETCLI_LOGIN}:${GNETCLI_PASSWORD} \
    -debug

Restart=on-failure
RestartSec=5s

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gnetcli

[Install]
WantedBy=multi-user.target
EOF
```

#### Step 4: Start and enable the gRPC server

```bash
# Reload systemd to recognize the new service
sudo systemctl daemon-reload

# Enable the service to start on boot
sudo systemctl enable gnetcli.service

# Start the service
sudo systemctl start gnetcli.service

# Check service status
sudo systemctl status gnetcli.service

# View logs
sudo journalctl -u gnetcli -f
```

#### Step 5: Configure client for gRPC mode

Install the gnetcli Python client:

```bash
pip install gnetcli
```

Create client configuration `~/.gnetcli/config.yaml`:

```yaml
# ~/.gnetcli/config.yaml
server:
  address: localhost:50051  # Match GNETCLI_GRPC_PORT from .env
  auth:
    username: your_gnetcli_user  # Match GNETCLI_LOGIN from .env
    password: your_secure_password  # Match GNETCLI_PASSWORD from .env
  tls:
    enabled: false  # Set to true if using TLS
    insecure_skip_verify: false

defaults:
  timeout: 30
  retry_count: 3

# Device-specific credentials (override defaults)
devices:
  router1:
    hostname: 192.168.1.1
    username: admin
    password: router_password

  switch1:
    hostname: 192.168.1.2
    username: admin
    password: switch_password
```

### Using gnetcli in gRPC mode

Once the gRPC server is running, use the gnetcli client to connect through it:

```bash
# Basic command execution through gRPC server
gnetcli -s localhost:50051 -u your_gnetcli_user -p your_secure_password \
    -H device.example.com -d device_admin -P device_password \
    "show version"

# Using environment variables for authentication
export GNETCLI_SERVER=localhost:50051
export GNETCLI_USER=your_gnetcli_user
export GNETCLI_PASSWORD=your_secure_password
gnetcli -H device.example.com "show interfaces"

# Execute multiple commands
gnetcli -s localhost:50051 -u your_gnetcli_user -p your_secure_password \
    -H router1 -d admin -P router_password \
    "show version" "show interfaces" "show ip route"

# Use with SSH config for device authentication
gnetcli -s localhost:50051 -u your_gnetcli_user -p your_secure_password \
    -H router1 "show running-config"
```

### Verifying gRPC Server Health

The gRPC server exposes an HTTP endpoint for health checks and metrics:

```bash
# Check server health (HTTP port from GNETCLI_HTTP_PORT)
curl http://localhost:50052/health

# View server metrics
curl http://localhost:50052/metrics

# Check gRPC service status
grpcurl -plaintext localhost:50051 list

# Test gRPC connectivity with reflection
grpcurl -plaintext localhost:50051 describe
```

### Docker Installation for gRPC Mode

Run gnetcli server in Docker:

```dockerfile
# Dockerfile
FROM python:3.10-slim

RUN pip install gnetcli[grpc]

COPY server.yaml /etc/gnetcli/server.yaml

EXPOSE 50051

CMD ["gnetcli-server", "--config", "/etc/gnetcli/server.yaml"]
```

Build and run:

```bash
# Build Docker image
docker build -t gnetcli-server .

# Run container
docker run -d \
  --name gnetcli-server \
  -p 50051:50051 \
  -v /etc/gnetcli:/etc/gnetcli \
  -v /var/log/gnetcli:/var/log/gnetcli \
  gnetcli-server
```

### Docker Compose Setup

```yaml
# docker-compose.yml
version: '3.8'

services:
  gnetcli-server:
    image: gnetcli-server
    container_name: gnetcli-server
    ports:
      - "50051:50051"
    volumes:
      - ./configs:/etc/gnetcli
      - ./logs:/var/log/gnetcli
    environment:
      - GNETCLI_LOG_LEVEL=INFO
      - GNETCLI_MAX_WORKERS=10
    restart: always
    networks:
      - gnetcli-net

networks:
  gnetcli-net:
    driver: bridge
```

### High Availability Setup

For production environments, use multiple gRPC servers with load balancing:

```yaml
# haproxy.cfg
global
    daemon

defaults
    mode tcp
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms

frontend gnetcli_frontend
    bind *:50051
    default_backend gnetcli_servers

backend gnetcli_servers
    balance roundrobin
    server gnetcli1 server1:50051 check
    server gnetcli2 server2:50051 check
    server gnetcli3 server3:50051 check
```

### Monitoring gRPC Server

Check server health:

```bash
# Check if server is running
grpcurl -plaintext localhost:50051 list

# Health check endpoint
curl http://localhost:8080/health

# View server metrics
curl http://localhost:8080/metrics
```

Monitor logs:

```bash
# Follow server logs
sudo journalctl -u gnetcli -f

# Check specific log file
tail -f /var/log/gnetcli/server.log
```

### Troubleshooting gRPC Mode

1. **Connection refused errors**:
```bash
# Check if server is listening
sudo netstat -tlnp | grep 50051

# Test connection
telnet localhost 50051
```

2. **Performance issues**:
```bash
# Increase worker threads
gnetcli-server --max-workers 20

# Enable connection pooling in config
```

3. **TLS/SSL errors**:
```bash
# Verify certificates
openssl x509 -in /etc/gnetcli/server.crt -text -noout

# Test with TLS disabled first
gnetcli --grpc-insecure -H device "show version"
```

## Standard Configuration

After installation, create a configuration file for standard mode:

```bash
# Create config directory
mkdir -p ~/.gnetcli

# Create basic configuration
cat > ~/.gnetcli/config.yaml << EOF
default:
  timeout: 30
  retry: 3
EOF
```

## Troubleshooting

### Permission Errors

If you encounter permission errors during installation:

```bash
pip install --user gnetcli
```

Or use sudo (not recommended):

```bash
sudo pip install gnetcli
```

### SSL Certificate Errors

If you encounter SSL certificate verification issues:

```bash
pip install --trusted-host pypi.org --trusted-host files.pythonhosted.org gnetcli
```

### Dependencies Issues

Install required dependencies manually:

```bash
pip install pyyaml paramiko netmiko
```

## Updating gnetcli

To update to the latest version:

```bash
pip install --upgrade gnetcli
```

## Uninstalling

To remove gnetcli:

```bash
pip uninstall gnetcli
```

## Authentication and Device Access

### Method 1: Username and Password Authentication

You can provide credentials directly in the command:

```bash
gnetcli -H device.example.com -u admin -p 'yourpassword' "show version"
```

For better security, use environment variables:

```bash
export GNETCLI_USER=admin
export GNETCLI_PASSWORD='yourpassword'
gnetcli -H device.example.com "show version"
```

Or store credentials in the configuration file:

```yaml
# ~/.gnetcli/config.yaml
default:
  username: admin
  password: yourpassword
  timeout: 30
  retry: 3

devices:
  router1:
    host: 192.168.1.1
    username: admin
    password: devicepassword
```

### Method 2: SSH Key Authentication

Configure SSH keys for passwordless authentication:

```bash
# Generate SSH key pair if you don't have one
ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa_gnetcli

# Copy public key to the network device
ssh-copy-id -i ~/.ssh/id_rsa_gnetcli.pub admin@device.example.com
```

Use the SSH key with gnetcli:

```bash
gnetcli -H device.example.com -u admin --ssh-key ~/.ssh/id_rsa_gnetcli "show version"
```

### Method 3: SSH Config File

Create or edit ~/.ssh/config to define device connections:

```bash
# ~/.ssh/config
Host router1
    HostName 192.168.1.1
    User admin
    Port 22
    IdentityFile ~/.ssh/id_rsa_gnetcli
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null

Host switch1
    HostName 192.168.1.2
    User admin
    Port 22
    IdentityFile ~/.ssh/id_rsa_gnetcli
    PreferredAuthentications publickey,password

Host firewall*
    User admin
    Port 22
    IdentityFile ~/.ssh/id_rsa_firewall
    ConnectTimeout 10
    ServerAliveInterval 60
```

Then use the SSH config alias with gnetcli:

```bash
# Uses settings from SSH config
gnetcli -H router1 "show interfaces"
gnetcli -H switch1 "show vlan"
```

### Method 4: Jump Hosts (Bastion)

For devices behind a jump host, configure SSH ProxyJump:

```bash
# ~/.ssh/config
Host bastion
    HostName bastion.example.com
    User jumpuser
    IdentityFile ~/.ssh/id_rsa_bastion

Host internal-*
    ProxyJump bastion
    User admin
    IdentityFile ~/.ssh/id_rsa_gnetcli

Host internal-router
    HostName 10.0.1.1
```

Access internal devices through bastion:

```bash
gnetcli -H internal-router "show running-config"
```

### Security Best Practices

1. **Never hardcode passwords** in scripts or command history
2. **Use SSH keys** whenever possible
3. **Set appropriate file permissions**:

```bash
chmod 600 ~/.ssh/config
chmod 600 ~/.ssh/id_rsa_gnetcli
chmod 600 ~/.gnetcli/config.yaml
```

4. **Use SSH agent** to manage keys:

```bash
# Start SSH agent
eval $(ssh-agent)

# Add your key
ssh-add ~/.ssh/id_rsa_gnetcli

# Now gnetcli can use the key without prompting
gnetcli -H device.example.com -u admin "show version"
```

5. **Rotate credentials regularly** and use strong passwords

### Testing Connection

Test device connectivity before running commands:

```bash
# Test SSH connection
ssh -T admin@device.example.com

# Test with gnetcli
gnetcli -H device.example.com -u admin --test-connection
```

## Additional Resources

- [gnetcli Documentation](https://github.com/annetutil/gnetcli)
- [Issue Tracker](https://github.com/annetutil/gnetcli/issues)
- [PyPI Package Page](https://pypi.org/project/gnetcli/)