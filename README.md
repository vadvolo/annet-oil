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
│   annet-default │   annet-telnet  │   annet-orion   │
│   (default)     │   (telnet dev.) │   (orion dev.)  │
└─────────────────┴─────────────────┴─────────────────┘
```

## Quick Start

### Installation

1. Clone the repository:
```bash
git clone <repo-url>
cd annet-oil
```

2. Set up the environment:
```bash
make setup
```

3. Build the project:
```bash
make build
```

### Docker

1. Start all services:
```bash
make docker-run
```

2. Check status:
```bash
make docker-logs
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

## License

MIT License
