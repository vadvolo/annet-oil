# Inventory Configuration

The inventory file defines network devices that annet-oil can connect to and manage. It stores device credentials, connection parameters, and metadata.

## File Location

Default path: `resources/inventory.yaml`

Can be configured in `config.yaml`:
```yaml
storage:
  inventory_file: "./resources/inventory.yaml"
```

## Structure

```yaml
devices:
  - hostname: router-1
    ip: 10.0.0.1
    port: 22                    # Optional, default: 22
    vendor: juniper
    platform: junos
    credentials:
      login: "${DEVICE_USERNAME}"
      password: "${DEVICE_PASSWORD}"
    description: "Core router"

default_credentials:
  login: "${DEVICE_USERNAME}"
  password: "${DEVICE_PASSWORD}"
```

## Device Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `hostname` | string | Yes | - | Device hostname (used for matching and display) |
| `ip` | string | Yes | - | IP address or FQDN for connection |
| `port` | int | No | 22 | SSH port for connection |
| `vendor` | string | Yes | - | Device vendor (juniper, cisco, arista, huawei) |
| `platform` | string | Yes | - | Platform type (junos, ios, eos, vrp) |
| `credentials` | object | No | default_credentials | Device-specific credentials |
| `description` | string | No | - | Human-readable description |

### Credentials

```yaml
credentials:
  login: "admin"
  password: "secret"
```

If not specified, `default_credentials` from the file root will be used.

## Environment Variables

The inventory file supports environment variable expansion using `${VAR_NAME}` syntax:

```yaml
devices:
  - hostname: router-1
    ip: 10.0.0.1
    credentials:
      login: "${DEVICE_USERNAME}"
      password: "${DEVICE_PASSWORD}"
```

Set environment variables before running:
```bash
export DEVICE_USERNAME=admin
export DEVICE_PASSWORD=secret
```

## Custom SSH Port

For devices running SSH on non-standard ports:

```yaml
devices:
  - hostname: lab-router
    ip: 10.0.0.1
    port: 2222              # Custom SSH port
    vendor: juniper
    platform: junos

  - hostname: prod-router
    ip: 10.1.0.1
    # port not specified - uses default 22
    vendor: cisco
    platform: ios
```

## Example

```yaml
devices:
  # Production devices
  - hostname: utn-lon-core-rtr-1
    ip: 10.44.33.1
    vendor: juniper
    platform: junos
    description: "London core router"

  - hostname: utn-tll-core-rtr-1
    ip: 10.45.33.1
    vendor: juniper
    platform: junos
    description: "Tallinn core router"

  # Lab devices with custom port
  - hostname: lab-rtr-1
    ip: 192.168.1.10
    port: 2222
    vendor: cisco
    platform: ios
    credentials:
      login: "labuser"
      password: "labpass"
    description: "Lab router via jump host"

  # Device with SSH key auth (empty password)
  - hostname: secure-rtr-1
    ip: 10.50.0.1
    vendor: arista
    platform: eos
    credentials:
      login: "admin"
      password: ""
    description: "Uses SSH key authentication"

default_credentials:
  login: "${DEVICE_USERNAME}"
  password: "${DEVICE_PASSWORD}"
```

## API Access

Query inventory via REST API:

```bash
# List all devices
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v0/inventory

# Filter by vendor
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v0/inventory?vendor=juniper"

# Filter by pattern
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v0/inventory?pattern=lab-*"
```

## MCP Tool

Use `annet_inventory` MCP tool to query devices:

```
annet_inventory(vendor="juniper", platform="junos")
annet_inventory(pattern="utn-tll-*")
```

## Device Matching

When executing commands, devices are matched in this order:
1. Exact hostname match
2. Exact IP match
3. Partial hostname match (contains)

If no match found, a default device is created using `default_credentials`.
