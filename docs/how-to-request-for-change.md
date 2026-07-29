# Request for Change (RFC) Workflow

This guide describes how to use annet-oil's RFC workflow for managing network configuration changes with proper change management practices.

## Overview

The RFC workflow integrates annet-oil with Jira to provide:
- Change request tracking
- Configuration diff documentation
- Approval process
- Deployment audit trail

## Prerequisites

1. Jira integration configured (see [how-to-jira.md](how-to-jira.md))
2. Devices in inventory (see [inventory.md](inventory.md))
3. Annet containers running

## Workflow Stages

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Create    │ -> │  Generate   │ -> │   Submit    │ -> │   Deploy    │ -> │    Close    │
│    RFC      │    │    Diff     │    │  for Review │    │  (approved) │    │     RFC     │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

## Step-by-Step Guide

### Step 1: Create RFC Ticket

Create a new RFC ticket in Jira with affected devices:

**Using MCP:**
```
annet_rfc_create(
  summary="Update BGP configuration for new transit peer",
  description="Add BGP peering with AS65001 on core routers for redundant transit",
  devices=["core-rtr-1", "core-rtr-2"],
  priority="Medium"
)
```

**Using API:**
```bash
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "summary": "Update BGP configuration for new transit peer",
    "description": "Add BGP peering with AS65001 on core routers",
    "devices": ["core-rtr-1", "core-rtr-2"],
    "priority": "Medium"
  }' \
  http://localhost:8080/api/v0/rfc/create
```

**Response:**
```json
{
  "ticket_key": "NET-123",
  "url": "https://company.atlassian.net/browse/NET-123"
}
```

### Step 2: Generate Configuration Diff

Generate the configuration diff for each affected device:

**Using MCP:**
```
annet_diff(filters=["core-rtr-1"])
```

**Using API:**
```bash
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"filters": ["core-rtr-1"]}' \
  http://localhost:8080/api/v0/diff
```

Save the diff output for the next step.

### Step 3: Post the Diff (or any note) to the RFC

Post the generated diff to the RFC ticket as a comment. The comment body can be
anything — a diff, a note, a status update — and supports Jira wiki markup (wrap
diffs in `{code}...{code}`):

**Using MCP:**
```
annet_rfc_post_comment(
  ticket_key="NET-123",
  comment="*Configuration diff for core-rtr-1*\n{code}\n[paste diff output here]\n{code}"
)
```

**Using API:**
```bash
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "ticket_key": "NET-123",
    "comment": "*Configuration diff for core-rtr-1*\n{code}\n- delete protocols bgp\n+ set protocols bgp group TRANSIT...\n{code}"
  }' \
  http://localhost:8080/api/v0/rfc/comment
```

Repeat for each device in the RFC.

### Step 4: Check RFC Status

Verify the ticket status and available transitions:

**Using MCP:**
```
annet_rfc_status(ticket_key="NET-123")
```

**Response:**
```
RFC Status: NET-123

Summary: Update BGP configuration for new transit peer
Status: Open

Available transitions: In Review, In Progress, Done
```

### Step 5: Submit for Review

Submit the RFC for approval:

**Using MCP:**
```
annet_rfc_submit(
  ticket_key="NET-123",
  comment="Diff attached for all devices. Tested in lab environment. Ready for review."
)
```

**Using API:**
```bash
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"comment": "Ready for review"}' \
  http://localhost:8080/api/v0/rfc/submit/NET-123
```

### Step 6: Wait for Approval

The ticket is now in review status. Wait for approval through your normal change management process:

- CAB (Change Advisory Board) review
- Peer review
- Manager approval
- Maintenance window scheduling

### Step 7: Deploy Changes

After approval, deploy the changes:

**Using MCP:**
```
annet_deploy(filters=["core-rtr-1", "core-rtr-2"])
```

**Using API:**
```bash
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"filters": ["core-rtr-1", "core-rtr-2"]}' \
  http://localhost:8080/api/v0/deploy
```

### Step 8: Close RFC

After successful deployment, close the RFC:

**Using MCP:**
```
annet_rfc_close(
  ticket_key="NET-123",
  resolution="Deployed successfully to core-rtr-1 and core-rtr-2. BGP sessions established."
)
```

**Using API:**
```bash
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"resolution": "Deployed successfully"}' \
  http://localhost:8080/api/v0/rfc/close/NET-123
```

## Complete Example Session

Here's a complete RFC workflow using MCP tools:

```
User: I need to add a new VLAN 100 to switches sw-1 and sw-2

AI: I'll create an RFC for this change.

> annet_rfc_create(
>   summary="Add VLAN 100 to access switches",
>   description="Create VLAN 100 for new department network segment",
>   devices=["sw-1", "sw-2"]
> )

RFC Created: NET-456

Now let me generate the configuration diff for each switch.

> annet_diff(filters=["sw-1"])

Diff for sw-1:
+ set vlans VLAN100 vlan-id 100
+ set vlans VLAN100 description "Department Network"

> annet_rfc_post_comment(ticket_key="NET-456", comment="*Diff for sw-1*\n{code}...{code}")

Comment posted to NET-456.

> annet_diff(filters=["sw-2"])
> annet_rfc_post_comment(ticket_key="NET-456", comment="*Diff for sw-2*\n{code}...{code}")

Both diffs posted. Submitting for review.

> annet_rfc_submit(ticket_key="NET-456", comment="VLAN 100 configuration ready for review")

NET-456 submitted for review. Please get approval before deployment.

--- After approval ---

User: NET-456 is approved, please deploy

AI: Deploying the approved changes.

> annet_deploy(filters=["sw-1", "sw-2"])

Deployment successful.

> annet_rfc_close(ticket_key="NET-456", resolution="VLAN 100 deployed to sw-1 and sw-2")

RFC NET-456 closed.
```

## Jira Ticket Contents

After the workflow, the Jira ticket contains:

### Description
```
Create VLAN 100 for new department network segment

*Affected Devices:*
* sw-1
* sw-2
```

### Comments

**Diff for sw-1:**
```
*Configuration Diff for sw-1*

{code}
+ set vlans VLAN100 vlan-id 100
+ set vlans VLAN100 description "Department Network"
{code}
```

**Diff for sw-2:**
```
*Configuration Diff for sw-2*

{code}
+ set vlans VLAN100 vlan-id 100
+ set vlans VLAN100 description "Department Network"
{code}
```

**Deployment Result:**
```
*Deployment Executed*

||Field||Value||
|User|alice|
|Hosts|[sw-1, sw-2]|
|Result|success|
|Time|2024-01-15T14:30:00Z|
```

## Emergency Changes

For emergency changes that cannot wait for normal approval:

1. Create RFC with priority "Critical"
2. Get verbal approval from on-call manager
3. Deploy immediately
4. Document approval in RFC comment
5. Complete formal approval post-deployment

```
annet_rfc_create(
  summary="[EMERGENCY] Fix BGP session down",
  description="BGP session to transit provider down. Requires immediate fix.",
  devices=["core-rtr-1"],
  priority="Critical"
)

# Deploy immediately after verbal approval
annet_deploy(filters=["core-rtr-1"])

# Document
annet_rfc_submit(
  ticket_key="NET-789",
  comment="Emergency change approved verbally by John Smith (on-call manager) at 03:15 UTC"
)
```

## Best Practices

### Before Creating RFC
- Verify changes in lab environment
- Identify all affected devices
- Plan rollback procedure

### During Review
- Include test results in comments
- Specify maintenance window
- List rollback steps

### During Deployment
- Deploy one device at a time for critical changes
- Verify each device before proceeding
- Monitor for issues

### After Deployment
- Verify services are working
- Close RFC with detailed resolution
- Update documentation if needed

## Rollback Procedure

If deployment fails:

1. Stop deployment to remaining devices
2. Add failure comment to RFC
3. Generate rollback diff
4. Deploy rollback
5. Update RFC with rollback details

```
# Deployment failed
annet_rfc_post_comment(
  ticket_key="NET-123",
  comment="*DEPLOYMENT FAILED on sw-1:* BGP session did not establish"
)

# Generate rollback (previous config)
annet_diff(filters=["sw-1"])  # Will show how to revert

# Deploy rollback
annet_deploy(filters=["sw-1"])

# Update RFC
annet_rfc_submit(
  ticket_key="NET-123",
  comment="Deployment failed on sw-1. Rolled back to previous configuration. Root cause analysis in progress."
)
```

## MCP Tools Reference

| Tool | Purpose |
|------|---------|
| `annet_rfc_create` | Create new RFC ticket |
| `annet_diff` | Generate configuration diff |
| `annet_rfc_post_comment` | Post a comment (diff, note, status) to RFC |
| `annet_rfc_status` | Check RFC status |
| `annet_rfc_submit` | Submit for review |
| `annet_deploy` | Deploy changes |
| `annet_rfc_close` | Close RFC |

## API Endpoints Reference

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/v0/rfc/create` | Create RFC |
| POST | `/api/v0/diff` | Generate diff |
| POST | `/api/v0/rfc/comment` | Post a comment |
| GET | `/api/v0/rfc/status/{key}` | Get status |
| POST | `/api/v0/rfc/submit/{key}` | Submit for review |
| POST | `/api/v0/deploy` | Deploy changes |
| POST | `/api/v0/rfc/close/{key}` | Close RFC |

## See Also

- [how-to-jira.md](how-to-jira.md) - Jira configuration
- [how-to-auth.md](how-to-auth.md) - RBAC for RFC access control
- [inventory.md](inventory.md) - Device inventory