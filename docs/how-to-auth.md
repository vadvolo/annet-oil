# Authentication and Authorization (RBAC)

annet-oil supports role-based access control (RBAC) to manage who can access which endpoints, devices, and commands.

## Configuration

Add the `auth` section to your `config.yaml`:

```yaml
auth:
  roles:
    - name: "operator"
      device_scopes: ["*"]
      methods: ["/gen", "/diff", "/patch", "/deploy", "/containers", "/routing", "/health", "/execute", "/inventory"]
      commands: ["show *", "ping *", "traceroute *"]
    
    - name: "viewer"
      device_scopes: ["*"]
      methods: ["/gen", "/diff", "/routing", "/health", "/containers", "/inventory"]
      commands: ["show *"]
    
    - name: "lab-admin"
      device_scopes: ["lab-*", "test-*"]
      methods: ["*"]
      commands: ["*"]

  users:
    - name: "alice"
      token: "token-alice-xxxxx"
      role: "operator"
    
    - name: "bob"
      token: "token-bob-xxxxx"
      role: "viewer"

  groups:
    - name: "netops"
      role: "operator"
      members: ["alice", "charlie"]
```

## Concepts

### Roles

Roles define what a user can do:

| Field | Description |
|-------|-------------|
| `name` | Unique role identifier |
| `device_scopes` | Device hostname patterns user can target |
| `methods` | API endpoints user can access |
| `commands` | CLI commands user can execute |

#### Device Scopes

Patterns that match device hostnames:
- `*` - all devices
- `lab-*` - devices starting with "lab-"
- `prod-rtr-*` - devices matching pattern

#### Methods

API endpoints:
- `*` - all endpoints
- `/gen`, `/diff`, `/patch`, `/deploy` - annet operations
- `/execute` - command execution
- `/inventory`, `/routing`, `/containers` - read operations
- `/rfc/*` - RFC workflow

#### Commands

CLI commands allowed via `/execute`:
- `*` - all commands
- `show *` - all show commands
- `ping *`, `traceroute *` - diagnostic commands

### Users

Users authenticate with Bearer tokens:

```yaml
users:
  - name: "alice"
    token: "secure-random-token-here"
    role: "operator"
```

### Groups

Groups allow assigning roles to multiple users:

```yaml
groups:
  - name: "netops"
    role: "operator"
    members: ["alice", "bob", "charlie"]
```

Users inherit the role from their group if no explicit role is set.

## Built-in Roles

### admin (implicit)

Full access to everything. Assigned to:
- Users with explicit `role: admin`
- Legacy token from `server.api.auth_token`

### Custom Roles

Define custom roles for your organization:

```yaml
roles:
  # Read-only access
  - name: "readonly"
    device_scopes: ["*"]
    methods: ["/gen", "/diff", "/routing", "/health", "/inventory"]
    commands: ["show *"]

  # Lab environment full access
  - name: "lab-admin"
    device_scopes: ["lab-*", "dev-*", "test-*"]
    methods: ["*"]
    commands: ["*"]

  # Production operators - no deploy
  - name: "prod-viewer"
    device_scopes: ["prod-*"]
    methods: ["/gen", "/diff", "/routing", "/health"]
    commands: ["show *"]

  # Full production access
  - name: "prod-admin"
    device_scopes: ["prod-*"]
    methods: ["*"]
    commands: ["*"]
```

## Authentication

### Bearer Token

All API requests require a Bearer token:

```bash
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v0/health
```

### Legacy Token

For backward compatibility, the `server.api.auth_token` still works and grants admin access:

```yaml
server:
  api:
    auth_token: "legacy-admin-token"
```

## Authorization Flow

1. Request arrives with `Authorization: Bearer <token>`
2. Token is looked up in users list
3. User's role is determined (explicit or from group)
4. RBAC middleware checks:
   - Can user access this endpoint? (`methods`)
   - Can user target these devices? (`device_scopes`)
   - Can user run this command? (`commands`)

## Examples

### Minimal Setup (Single Admin)

```yaml
server:
  api:
    auth_token: "my-secret-token"

auth:
  users: []
  groups: []
```

### Team Setup

```yaml
auth:
  roles:
    - name: "operator"
      device_scopes: ["*"]
      methods: ["/gen", "/diff", "/patch", "/deploy", "/execute", "/health", "/routing", "/inventory"]
      commands: ["show *", "ping *"]
    
    - name: "viewer"
      device_scopes: ["*"]
      methods: ["/gen", "/diff", "/health", "/routing", "/inventory"]
      commands: ["show *"]

  users:
    - name: "alice"
      token: "${ALICE_TOKEN}"
      role: "operator"
    
    - name: "bob"
      token: "${BOB_TOKEN}"
      role: "viewer"
    
    - name: "admin"
      token: "${ADMIN_TOKEN}"
      role: "admin"
```

### Multi-Environment Setup

```yaml
auth:
  roles:
    - name: "lab-full"
      device_scopes: ["lab-*"]
      methods: ["*"]
      commands: ["*"]
    
    - name: "prod-readonly"
      device_scopes: ["prod-*"]
      methods: ["/gen", "/diff", "/routing", "/inventory"]
      commands: ["show *"]
    
    - name: "prod-operator"
      device_scopes: ["prod-*"]
      methods: ["*"]
      commands: ["*"]

  groups:
    - name: "developers"
      role: "lab-full"
      members: ["dev1", "dev2", "dev3"]
    
    - name: "operators"
      role: "prod-operator"
      members: ["ops1", "ops2"]

  users:
    - name: "dev1"
      token: "${DEV1_TOKEN}"
    - name: "dev2"
      token: "${DEV2_TOKEN}"
    - name: "ops1"
      token: "${OPS1_TOKEN}"
    - name: "ops2"
      token: "${OPS2_TOKEN}"
```

## MCP Integration

When using MCP tools, the auth token is passed via environment variable:

```bash
export ANNET_OIL_AUTH_TOKEN="user-token-here"
```

Tools are filtered based on user's role - viewers won't see `annet_deploy` or `annet_patch`.

## Troubleshooting

### 401 Unauthorized

- Check token is correct
- Ensure `Authorization: Bearer <token>` format

### 403 Forbidden

- User's role doesn't allow this endpoint
- Check `methods` in role definition

### Empty Device List

- User's `device_scopes` don't match any devices
- Check patterns in role definition
