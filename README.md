# Annet Oil

Annet Oil is a Go wrapper for orchestrating multiple annet containers. It provides CLI and REST API interfaces for managing annet operations (gen, diff, patch, deploy) with automatic hostname-based routing.

## Features

- 🐳 **Docker container orchestration** - manage multiple annet containers
- 🌐 **REST API** - HTTP API for integration with external systems
- 💻 **CLI interface** - convenient command line with Cobra
- 🔀 **Automatic routing** - distribute commands to containers based on hostname
- 🔐 **SSH server** - remote access to commands
- ⚙️ **Flexible configuration** - YAML configuration with SSH key support

## Architecture

```
annet-oil (port 22 SSH, 8080 API)
    ↓
JSON routing hostname → container
    ↓
┌─────────────────┬─────────────────┬─────────────────┐
│   annet-default │   annet-telnet  │   annet-ssh-c   │
│   (default)     │   (telnet dev.) │   (custom ssh)  │
└─────────────────┴─────────────────┴─────────────────┘
```

## Architecture Decision: Docker Exec API vs SDK Integration

### Current Approach

Annet Oil uses Docker Exec API for communication with Annet containers:

```
Annet Oil (Go application)
    ↓
Docker SDK for Go (github.com/docker/docker)
    ↓
Docker Exec API
    ↓
Container stdin/stdout (annet CLI commands)
```

### Why Docker Exec API?

**Advantages:**

1. **Simplicity & Reliability**
   - Uses standard Docker SDK for Go, a mature and well-tested solution
   - No modifications required to existing Annet containers
   - Works with any Annet version without compatibility concerns

2. **Container Isolation**
   - Each Annet container remains fully isolated
   - No network dependencies between containers
   - Security boundaries enforced by Docker

3. **Flexibility**
   - Easy to add new containers with different Annet versions or configurations
   - Supports custom Annet builds without code changes
   - Can work with any container that has Annet CLI installed

4. **Sufficient Performance**
   - Docker Exec API latency (typically <10ms) is negligible for CLI operations
   - No additional network overhead from API layers
   - Suitable for typical workloads (up to 100 requests/second)

**Limitations:**

1. **Text-based communication** - requires parsing stdout/stderr
2. **No real-time progress** for long-running operations
3. **Limited error handling** compared to structured API responses

### Alternative Approaches Considered

1. **Python SDK Integration**
   - Would require embedding Python runtime or separate Python service
   - Adds complexity and version dependency on Annet Python SDK
   - Not justified for current use cases

2. **gRPC/REST API in Containers**
   - Requires modifying Annet containers to add API servers
   - Increases container size and complexity
   - More moving parts to maintain

### Decision Rationale

The Docker Exec approach is **optimal for the current stage** because:
- It meets all functional requirements
- Maintains simplicity and reliability
- Requires minimal dependencies
- Provides good performance for expected workloads

### Future Considerations

Migration to SDK-based approach would be considered if:
- Annet provides an official Go SDK
- Requirements emerge for real-time operation progress
- Workload exceeds 100+ requests/second consistently
- Need for complex data structures that are difficult to parse from text

## Prerequisites

- **Go** 1.21 or later
- **Docker** and **Docker Compose**
- **Make** utility
- **Git**
- **Linux/macOS** (Windows users can use WSL2)

## Installation

### Method 1: Quick Start (Development)

1. **Clone the repository:**
```bash
git clone https://github.com/yourusername/annet-oil.git
cd annet-oil
```

2. **Set up the environment:**
```bash
# Create necessary directories and configuration files
make setup

# This will:
# - Create .env file from template
# - Create keys/ directory for SSH keys
# - Create annet-configs/ directories
# - Create annet-data/ directories
```

3. **Configure authentication:**
```bash
# Edit .env file to set your authentication token
vim .env
# Change ANNET_OIL_AUTH_TOKEN to a secure value
```

4. **Build all components:**
```bash
# Build Go binary and Docker containers
make build

# Or build separately:
make build-api  # Build only the Go binary
make build-mcp  # Build only the MCP container
```

5. **Run services:**
```bash
# Run in background
make run-bg

# Or run in foreground (for debugging)
make run

# Or run specific components
make run-api     # Run API server only
make run-mcp     # Run MCP container only
make run-annet   # Run Annet containers only
```

6. **Verify installation:**
```bash
# Check service health
make health

# Check service status
make status

# Test API endpoint (replace TOKEN with your actual token)
curl -H "Authorization: Bearer YOUR_TOKEN" http://localhost:8080/api/v0/health
```

### Method 2: Production Installation (systemd)

1. **Prepare the system:**
```bash
# Create system user
sudo useradd -r -s /bin/false annet
sudo usermod -aG docker annet

# Create installation directories
sudo mkdir -p /opt/annet-oil/{bin,configs,storage,keys}
sudo chown -R annet:annet /opt/annet-oil
```

2. **Build and install:**
```bash
# Clone and build
git clone https://github.com/yourusername/annet-oil.git
cd annet-oil
make build-api

# Copy files to system directories
sudo cp annet-oil-server /opt/annet-oil/bin/annet-oil
sudo cp -r configs/* /opt/annet-oil/configs/
sudo cp .env /opt/annet-oil/.env  # After configuring
sudo chown -R annet:annet /opt/annet-oil
```

3. **Install and start systemd service:**
```bash
# Install service file
make service-install

# Enable auto-start on boot
make service-enable

# Start the service
make service-start

# Check service status
make service-status

# View logs
make service-logs
```

### Method 3: Docker-Only Installation

1. **Using docker-compose:**
```bash
# Clone repository
git clone https://github.com/yourusername/annet-oil.git
cd annet-oil

# Configure environment
cp .env.example .env
vim .env  # Set ANNET_OIL_AUTH_TOKEN

# Start all services
docker-compose up -d

# Check logs
docker-compose logs -f
```

2. **Using pre-built images (if available):**
```bash
# Pull images
docker pull yourusername/annet-oil:latest
docker pull yourusername/mcp-annet-oil:latest

# Run with docker-compose
docker-compose up -d
```

### Post-Installation Configuration

1. **Configure routing:**
```bash
# Add device routing
./annet-oil-server routing add device1.example.com annet-telnet
./annet-oil-server routing add device2.example.com annet-default

# Or edit storage/routing.json directly
```

2. **Set up SSH keys (optional):**
```bash
# Generate SSH key pair
ssh-keygen -t rsa -b 4096 -f keys/id_rsa -N ""

# Add public key to authorized_keys for SSH access
cat keys/id_rsa.pub >> ~/.ssh/authorized_keys
```

3. **Start Annet containers (if not using docker-compose):**
```bash
# Start individual Annet containers
docker run -d --name annet-default annet:latest
docker run -d --name annet-telnet annet:telnet
docker run -d --name annet-orion annet:orion
```

### Verifying Installation

```bash
# Check all services
make health

# API health check
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/api/v0/health

# Extended health check (includes gnetcli status)
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/api/v0/health/extended

# List containers
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/api/v0/containers

# SSH test
ssh -p 2222 admin@localhost "help"
```

### Troubleshooting

**Service won't start:**
```bash
# Check logs
make service-logs
sudo journalctl -u annet-oil.service -n 50

# Check Docker permissions
ls -la /var/run/docker.sock
sudo usermod -aG docker $USER  # or annet user
```

**API returns 401 Unauthorized:**
```bash
# Check token in config
cat /opt/annet-oil/configs/config.yaml | grep auth_token
# Ensure it matches your Authorization header
```

**Container connection issues:**
```bash
# Check if containers are running
docker ps | grep annet

# Test container access
docker exec annet-default annet --help

# Check Docker socket permissions
sudo chmod 666 /var/run/docker.sock  # temporary fix
```

**Port conflicts:**
```bash
# Check if ports are in use
sudo netstat -tlnp | grep -E "8080|2222"

# Change ports in configs/config.yaml if needed
```

## Usage

### CLI

```bash
# Generate configurations
annet-oil gen -g router1.example.com
annet-oil gen -g device1,device2 --container annet-telnet

# Show differences
annet-oil diff -G group1

# Apply changes
annet-oil patch -g router1.example.com --dry-run
annet-oil deploy -g router1.example.com

# Container management
annet-oil containers list
annet-oil routing show
annet-oil routing add device1.example.com annet-telnet

# Device availability check (ports + SSH login)
annet-oil check router1.example.com                 # single host/IP
annet-oil check --vendor cisco --no-login           # all Cisco devices, ports only
annet-oil check --ports 22,23,10022,21022 \
  --concurrency 100 -o availability-report.json      # batch the whole inventory in parallel

# Start servers
annet-oil server start        # API + SSH
annet-oil server api          # API only
annet-oil server ssh          # SSH only
```

### REST API

```bash
# Generate
curl -X GET "http://localhost:8080/api/v0/gen?filters=router1.example.com" \
  -H "Authorization: Bearer your-token"

# Deploy with JSON
curl -X POST "http://localhost:8080/api/v0/deploy" \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "filters": ["router1.example.com"],
    "container": "annet-telnet",
    "dry_run": true
  }'

# Container status
curl "http://localhost:8080/api/v0/containers" \
  -H "Authorization: Bearer your-token"

# Routing
curl "http://localhost:8080/api/v0/routing" \
  -H "Authorization: Bearer your-token"

# Device availability check
curl "http://localhost:8080/api/v0/check?host=router1.example.com&ports=22,23" \
  -H "Authorization: Bearer your-token"

curl -X POST "http://localhost:8080/api/v0/check" \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{"host": "10.0.0.1", "ports": [22, 23, 10022], "login": true}'
```

### SSH

```bash
# Connect via SSH
ssh -p 2222 admin@localhost

# Execute commands
ssh -p 2222 admin@localhost "annet-oil gen -g router1.example.com"
```

## Configuration

### configs/config.yaml

```yaml
annet_containers:
  - name: "annet"
    container_name: "annet-default"
    default: true
    description: "Default annet container"
  - name: "annet-telnet"
    container_name: "annet-telnet"
    description: "Telnet devices container"

ssh_keys:
  - name: "default"
    path: "/keys/id_rsa"
    user: "admin"

server:
  ssh:
    port: 22
    bind: "0.0.0.0"
  api:
    port: 8080
    bind: "0.0.0.0"
    auth_token: "your-secret-token"

storage:
  routing_file: "./storage/routing.json"

docker:
  # For Docker Desktop: leave empty (auto-detect)
  host: ""
  # For Colima: unix:///Users/<user>/.colima/default/docker.sock
  # For remote Docker: tcp://hostname:2376
  # api_version: "1.41"  # optional
```

### storage/routing.json

```json
{
  "routes": {
    "router1.example.com": "annet",
    "old-router.example.com": "annet-telnet",
    "orion-device1.example.com": "annet-orion"
  }
}
```

## API Endpoints

| Endpoint | Methods | Description |
|----------|---------|-------------|
| `/api/v0/gen` | GET, POST | Generate configurations |
| `/api/v0/diff` | GET, POST | Show differences |
| `/api/v0/patch` | POST | Apply changes |
| `/api/v0/deploy` | POST | Deploy configurations |
| `/api/v0/containers` | GET | Container status |
| `/api/v0/routing` | GET, POST, DELETE | Manage routing |
| `/api/v0/inventory` | GET | List inventory devices |
| `/api/v0/check` | GET, POST | Device availability (ports + SSH login) |
| `/api/v0/health` | GET | Health check |

## Makefile commands

```bash
make help           # Show help
make build          # Build the project
make run            # Run
make dev            # Development mode
make test           # Run tests
make lint           # Lint code
make docker-run     # Run in Docker
make clean          # Clean artifacts
```

## Workflow

1. **Command arrives** via CLI, API, or SSH
2. **Parameter parsing** - extract filters (-g, -G) and options
3. **Routing** - determine target container by hostname from routing.json
4. **Execution** - proxy command to the appropriate annet container
5. **Return result** - formatted output to the user

## Docker Configuration

### Docker Desktop
```yaml
docker:
  host: ""  # Auto-detect
```

### Colima
```yaml
docker:
  host: "unix:///Users/<username>/.colima/default/docker.sock"
  api_version: "1.41"
  tls_verify: false
```

### Remote Docker
```yaml
docker:
  host: "tcp://docker-host:2376"
  api_version: "1.41"
  tls_verify: true
  cert_path: "/path/to/certs"
```

### Quick switching
```bash
# For Colima
cp configs/config.colima.yaml configs/config.yaml

# For Docker Desktop
cp configs/config.docker.yaml configs/config.yaml
```

## Environment Variables

- `ANNET_OIL_CONFIG` - path to configuration file
- `DOCKER_HOST` - Docker daemon endpoint (overrides config settings)
- `DOCKER_API_VERSION` - Docker API version
- `DOCKER_CERT_PATH` - path to TLS certificates
- `DOCKER_TLS_VERIFY` - enable TLS verification
- `DEVICE_USERNAME` - default username for network devices
- `DEVICE_PASSWORD` - default password for network devices

## FAQ

### SSH Authentication Issues

**Problem:** SSH connection fails with error:
```
ssh: handshake failed: ssh: unable to authenticate, attempted methods [none], no supported methods remain
```

**Solution:** This error occurs when device credentials are not defined. Set the environment variables and restart:
```bash
# Set device credentials in .env file
echo 'DEVICE_USERNAME=your_device_user' >> .env
echo 'DEVICE_PASSWORD=your_device_password' >> .env

# Or export them directly
export DEVICE_USERNAME="your_device_user"
export DEVICE_PASSWORD="your_device_password"

# Restart the services
make restart
```

### API Returns 404 Not Found

**Problem:** API endpoints return 404 error

**Solution:** Ensure you're using the correct API path with `/api/v0/` prefix:
```bash
# Correct
curl -H "Authorization: Bearer TOKEN" http://localhost:8080/api/v0/health

# Incorrect (will return 404)
curl -H "Authorization: Bearer TOKEN" http://localhost:8080/health
```

### Docker Permission Denied

**Problem:** Container execution fails with Docker permission errors

**Solution:** Add the user to the docker group:
```bash
# For development user
sudo usermod -aG docker $USER

# For production (annet user)
sudo usermod -aG docker annet

# Apply changes (logout/login or use)
newgrp docker

# Restart service if using systemd
make service-restart
```

### MCP Container Won't Start

**Problem:** MCP container fails to build or start

**Solution:** Check Docker Compose version compatibility:
```bash
# Check your docker-compose version
docker-compose --version

# If version is old, update docker-compose.yml version
# Change from version: '3.8' to version: '3.3'
sed -i 's/version: .*/version: "3.3"/' docker-compose.mcp.yml

# Rebuild
make build-mcp
```

### Service Fails on System Boot

**Problem:** Systemd service fails to start after system reboot

**Solution:** Ensure Docker service dependency:
```bash
# Check if Docker is enabled
sudo systemctl is-enabled docker

# Enable Docker to start on boot
sudo systemctl enable docker

# Restart annet-oil service
sudo systemctl daemon-reload
sudo systemctl restart annet-oil.service
```

### Cannot Access Annet Containers

**Problem:** Annet Oil cannot execute commands in Annet containers

**Solution:** Verify containers are running and accessible:
```bash
# Check if containers are running
docker ps | grep annet

# Start Annet containers
make run-annet

# Test container access manually
docker exec annet-default annet --help

# Check container configuration in config.yaml
cat configs/config.yaml | grep -A5 annet_containers
```

## License

MIT License
