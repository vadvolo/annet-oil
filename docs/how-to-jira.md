# Jira Integration

annet-oil integrates with Jira for RFC (Request for Change) workflow management.

## Configuration

Add the `integrations.jira` section to your `config.yaml`:

```yaml
integrations:
  jira:
    enabled: true
    url: "https://company.atlassian.net"
    email: "service-account@company.com"
    token: "${JIRA_API_TOKEN}"
    project_key: "NET"
    issue_type: "Task"
```

## Options

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | bool | Yes | Enable/disable Jira integration |
| `url` | string | Yes | Jira instance URL |
| `email` | string | Yes | Email for API authentication |
| `token` | string | Yes | API token |
| `project_key` | string | Yes | Default project for new tickets |
| `issue_type` | string | Yes | Default issue type (Task, Story, etc.) |

## Getting API Token

1. Go to https://id.atlassian.com/manage-profile/security/api-tokens
2. Click "Create API token"
3. Give it a name (e.g., "annet-oil")
4. Copy the token and store securely

## Environment Variables

Store sensitive values in environment variables:

```bash
export JIRA_API_TOKEN="your-api-token-here"
```

Reference in config:
```yaml
integrations:
  jira:
    token: "${JIRA_API_TOKEN}"
```

## RFC Workflow

### 1. Create RFC Ticket

Using MCP:
```
annet_rfc_create(
  summary="Update BGP configuration on core routers",
  description="Add new BGP peer for transit provider",
  devices=["core-rtr-1", "core-rtr-2"],
  priority="Medium"
)
```

Using API:
```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "summary": "Update BGP configuration",
    "description": "Add new BGP peer",
    "devices": ["core-rtr-1", "core-rtr-2"],
    "priority": "Medium"
  }' \
  http://localhost:8080/api/v0/rfc/create
```

### 2. Generate and Attach Diff

```
# Generate diff
annet_diff(filters=["core-rtr-1"])

# Attach to ticket
annet_rfc_attach_diff(
  ticket_key="NET-123",
  device="core-rtr-1",
  diff="<diff output from previous command>"
)
```

### 3. Submit for Review

```
annet_rfc_submit(
  ticket_key="NET-123",
  comment="Ready for review. Tested in lab environment."
)
```

### 4. Deploy (After Approval)

```
annet_deploy(filters=["core-rtr-1", "core-rtr-2"])
```

### 5. Close RFC

```
annet_rfc_close(
  ticket_key="NET-123",
  resolution="Deployed successfully to all devices"
)
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `annet_rfc_create` | Create new RFC ticket |
| `annet_rfc_attach_diff` | Attach config diff to ticket |
| `annet_rfc_status` | Get ticket status and transitions |
| `annet_rfc_submit` | Submit for review/approval |
| `annet_rfc_close` | Close ticket after deployment |

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v0/rfc/create` | Create RFC |
| POST | `/api/v0/rfc/attach-diff` | Attach diff |
| GET | `/api/v0/rfc/status/{key}` | Get status |
| POST | `/api/v0/rfc/submit/{key}` | Submit for review |
| POST | `/api/v0/rfc/close/{key}` | Close ticket |
| POST | `/api/v0/rfc/deploy-comment/{key}` | Add deploy result |

## Ticket Format

Created tickets include:

**Summary:** User-provided

**Description:**
```
User-provided description

*Affected Devices:*
* device-1
* device-2
```

**Labels:** `rfc`, `network-change`, `annet-oil`

**Diff Comments:**
```
*Configuration Diff for device-1*

{code}
- old config line
+ new config line
{code}
```

**Deploy Comments:**
```
*Deployment Executed*

||Field||Value||
|User|alice|
|Hosts|[device-1, device-2]|
|Result|success|
|Time|2024-01-15T10:30:00Z|
```

## Workflow Transitions

The integration attempts to transition tickets through these statuses:

| Action | Target Statuses (in order of preference) |
|--------|----------------------------------------|
| Submit | In Review, Pending Approval, Review, In Progress |
| Close | Done, Closed |

If your Jira workflow uses different status names, the transition may not occur automatically.

## Troubleshooting

### 503 Service Unavailable

Jira integration not configured or disabled:
```yaml
integrations:
  jira:
    enabled: true  # Must be true
    url: "..."     # Must be set
    token: "..."   # Must be set
```

### 401 Authentication Failed

- Check email and token are correct
- Ensure token hasn't expired
- Verify token has required permissions

### Transition Not Available

Your Jira workflow may use different status names. Check available transitions:
```
annet_rfc_status(ticket_key="NET-123")
```

### Project Not Found

Verify `project_key` matches an existing Jira project.
