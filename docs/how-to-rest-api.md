# Annet Oil REST API Guide

## Overview

Annet Oil provides a REST API for executing network configuration management commands. The API supports `gen`, `diff`, `patch`, and `deploy` operations through HTTP endpoints.

## Starting the REST API Server

### Start API server only
```bash
annet-oil server api
```

### Start both API and SSH servers
```bash
annet-oil server start
```

The API server listens on the configured host and port (default: `0.0.0.0:8080`).

## Authentication

The API uses token-based authentication. Include the auth token in the `Authorization` header:

```bash
Authorization: Bearer YOUR_AUTH_TOKEN
```

The auth token is configured in your `annet-oil.yaml` configuration file:
```yaml
server:
  api:
    bind: 0.0.0.0
    port: 8080
    auth_token: "your-secure-token-here"
```

## API Endpoints

All endpoints are prefixed with `/api/v0/`

### Generate Configuration

**Endpoint:** `GET /api/v0/gen` or `POST /api/v0/gen`

Generate configuration for network devices.

#### Query Parameters (GET)
- `filters` - Comma-separated list of hostnames to target
- `container` - Specific container to use (optional)
- `parallel` - Execute in parallel mode (`true`/`false`)
- `timeout` - Command timeout in seconds
- `quiet` - Suppress stderr warnings (`true`/`false`)

#### Request Body (POST)
```json
{
  "filters": ["hostname1", "hostname2"],
  "generators": ["generator1", "generator2"],
  "container": "container_name",
  "parallel": true,
  "timeout": 60,
  "quiet": false,
  "extra_args": ["--arg1", "value1"],
  "environment": {
    "ENV_VAR": "value"
  }
}
```

#### Response
```json
{
  "success": true,
  "results": {
    "hostname1": {
      "container": "default",
      "exit_code": 0,
      "stdout": "Generated configuration...",
      "stderr": "",
      "duration": "1.5s"
    }
  },
  "total_hosts": 1,
  "success_hosts": 1,
  "failed_hosts": 0
}
```

### Diff Configuration

**Endpoint:** `GET /api/v0/diff` or `POST /api/v0/diff`

Show differences between current and desired configuration.

#### Query Parameters (GET)
Same as `/gen` endpoint

#### Request Body (POST)
Same structure as `/gen` endpoint

#### Response
```json
{
  "success": true,
  "results": {
    "hostname1": {
      "container": "default",
      "exit_code": 0,
      "stdout": "--- Current\n+++ Desired\n@@ -1,3 +1,4 @@\n+interface Ethernet1\n...",
      "stderr": "",
      "duration": "2.1s"
    }
  },
  "total_hosts": 1,
  "success_hosts": 1,
  "failed_hosts": 0
}
```

### Patch Configuration

**Endpoint:** `POST /api/v0/patch`

Apply configuration patches to devices.

#### Request Body
```json
{
  "filters": ["hostname1", "hostname2"],
  "container": "container_name",
  "dry_run": false,
  "parallel": true,
  "timeout": 120,
  "quiet": false
}
```

Additional parameter:
- `dry_run` - Perform dry run without applying changes (`true`/`false`)

#### Response
Same structure as other endpoints

### Deploy Configuration

**Endpoint:** `POST /api/v0/deploy`

Deploy configuration to devices.

#### Request Body
```json
{
  "filters": ["hostname1", "hostname2"],
  "container": "container_name",
  "dry_run": false,
  "parallel": true,
  "timeout": 300,
  "quiet": false
}
```

Additional parameter:
- `dry_run` - Perform dry run without deploying (`true`/`false`)

Note: The deploy command automatically includes `--no-ask-deploy` flag to skip interactive confirmation.

#### Response
Same structure as other endpoints

## Additional Endpoints

### Container Status

**Endpoint:** `GET /api/v0/containers`

Get status of all configured containers.

#### Response
```json
{
  "default": {
    "name": "default",
    "image": "annet:latest",
    "status": "running",
    "uptime": "2h15m"
  }
}
```

### Routing Information

**Endpoint:** `GET /api/v0/routing`

Get routing information for hostnames.

#### Query Parameters
- `hostnames` - Comma-separated list of hostnames

#### Response
```json
{
  "hostname1": "container1",
  "hostname2": "default"
}
```

### Health Check

**Endpoint:** `GET /api/v0/health`

Health check endpoint.

#### Response
```json
{
  "status": "healthy"
}
```

## Examples

### Generate configuration using curl

```bash
# GET request with query parameters
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/v0/gen?filters=router1,router2&parallel=true"

# POST request with JSON body
curl -X POST -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"filters": ["router1", "router2"], "parallel": true}' \
  http://localhost:8080/api/v0/gen
```

### Show diff for specific devices

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/v0/diff?filters=switch1,switch2"
```

### Apply patches with dry run

```bash
curl -X POST -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"filters": ["device1"], "dry_run": true}' \
  http://localhost:8080/api/v0/patch
```

### Deploy configuration

```bash
curl -X POST -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"filters": ["device1", "device2"], "parallel": true, "timeout": 300}' \
  http://localhost:8080/api/v0/deploy
```

## Python Client Example

```python
import requests
import json

class AnnetOilClient:
    def __init__(self, base_url, auth_token):
        self.base_url = base_url
        self.headers = {
            'Authorization': f'Bearer {auth_token}',
            'Content-Type': 'application/json'
        }

    def gen(self, filters, **kwargs):
        """Generate configuration for devices"""
        payload = {'filters': filters, **kwargs}
        response = requests.post(
            f'{self.base_url}/api/v0/gen',
            headers=self.headers,
            json=payload
        )
        return response.json()

    def diff(self, filters, **kwargs):
        """Show configuration differences"""
        payload = {'filters': filters, **kwargs}
        response = requests.post(
            f'{self.base_url}/api/v0/diff',
            headers=self.headers,
            json=payload
        )
        return response.json()

    def patch(self, filters, dry_run=False, **kwargs):
        """Apply configuration patches"""
        payload = {'filters': filters, 'dry_run': dry_run, **kwargs}
        response = requests.post(
            f'{self.base_url}/api/v0/patch',
            headers=self.headers,
            json=payload
        )
        return response.json()

    def deploy(self, filters, dry_run=False, **kwargs):
        """Deploy configuration"""
        payload = {'filters': filters, 'dry_run': dry_run, **kwargs}
        response = requests.post(
            f'{self.base_url}/api/v0/deploy',
            headers=self.headers,
            json=payload
        )
        return response.json()

# Usage example
client = AnnetOilClient('http://localhost:8080', 'your-auth-token')

# Generate configuration
result = client.gen(['router1', 'router2'], parallel=True)
print(json.dumps(result, indent=2))

# Show diff
diff_result = client.diff(['switch1'])
print(diff_result['results']['switch1']['stdout'])

# Deploy with dry run
deploy_result = client.deploy(['device1'], dry_run=True, timeout=300)
if deploy_result['success']:
    print("Dry run successful")
```

## Error Handling

The API returns appropriate HTTP status codes:

- `200 OK` - Request successful
- `400 Bad Request` - Invalid request parameters
- `401 Unauthorized` - Missing or invalid auth token
- `500 Internal Server Error` - Server error

Error response format:
```json
{
  "success": false,
  "error": "Error message description"
}
```

## Container Routing

The API automatically routes requests to appropriate containers based on:

1. **Explicit container**: If `container` parameter is specified
2. **Hostname routing**: Based on routing rules in configuration
3. **Default container**: Falls back to default container if no routing match

## Configuration

Configure the REST API in your `annet-oil.yaml`:

```yaml
server:
  api:
    bind: 0.0.0.0
    port: 8080
    auth_token: "secure-token-here"

containers:
  - name: default
    image: annet:latest
    command: ["annet"]

  - name: legacy
    image: annet:legacy
    command: ["annet"]

routing:
  rules:
    - pattern: "legacy-*"
      container: legacy
    - pattern: "*"
      container: default
```

## Best Practices

1. **Use HTTPS in production** - Deploy behind a reverse proxy with TLS termination
2. **Secure auth tokens** - Use strong, randomly generated tokens
3. **Set appropriate timeouts** - Configure timeouts based on your network size
4. **Use parallel mode carefully** - Can increase load on both API server and network devices
5. **Monitor API logs** - Track usage and errors for troubleshooting
6. **Rate limiting** - Consider implementing rate limiting for production deployments
7. **Use dry run** - Always test with `dry_run: true` before actual deployment

## Troubleshooting

### Check API server status
```bash
curl http://localhost:8080/api/v0/health
```

### View container status
```bash
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/api/v0/containers
```

### Debug request/response
```bash
curl -v -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/api/v0/gen?filters=device1
```

### Common Issues

1. **401 Unauthorized** - Check auth token in request header matches configuration
2. **Container not found** - Verify container name exists in configuration
3. **Timeout errors** - Increase timeout parameter for slow operations
4. **No results** - Check hostname filters and routing rules