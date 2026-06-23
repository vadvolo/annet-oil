# Filtered Generators Support in Annet Oil

## Overview

Annet Oil now supports filtering generators for `gen`, `diff`, `patch`, and `deploy` commands. This allows you to include or exclude specific generators when executing commands.

## API Usage

### Include Specific Generators

Use the `generators` field to include only specific generators:

```json
{
  "filters": ["router1.example.com"],
  "generators": ["description", "interfaces"],
  "dry_run": true
}
```

This translates to the annet command:
```bash
annet gen -g description -g interfaces router1.example.com
```

### Exclude Specific Generators

Use the `exclude_generators` field to exclude specific generators:

```json
{
  "filters": ["router1.example.com"],
  "exclude_generators": ["hostname", "acl"],
  "dry_run": true
}
```

This translates to the annet command:
```bash
annet gen -G hostname -G acl router1.example.com
```

### Combine Include and Exclude

You can combine both `generators` and `exclude_generators`:

```json
{
  "filters": ["router1.example.com"],
  "generators": ["interfaces", "routing", "vlans"],
  "exclude_generators": ["acl"],
  "dry_run": true
}
```

This translates to:
```bash
annet gen -g interfaces -g routing -g vlans -G acl router1.example.com
```

## MCP Server Usage

The MCP server tools (`annet_gen`, `annet_diff`, `annet_patch`, `annet_deploy`) all support the new filtering parameters:

```typescript
{
  filters?: string[];           // Device hostnames or patterns
  generators?: string[];        // Include these generators (-g)
  exclude_generators?: string[]; // Exclude these generators (-G)
  container?: string;           // Specific container to use
  dry_run?: boolean;           // Dry run mode
  parallel?: boolean;          // Parallel execution
  timeout?: number;            // Command timeout
  quiet?: boolean;             // Suppress warnings
}
```

## Testing

### Python Test Script

Run the Python test script to verify API functionality:

```bash
./test_filtered_generators.py
```

### Node.js Test Script

Run the Node.js test script to verify MCP client functionality:

```bash
cd mcp-annet-oil
node test-filtered.js
```

## Examples

### Example 1: Generate only interface descriptions

```bash
curl -X POST http://localhost:8080/api/v0/gen \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "filters": ["switch1.example.com"],
    "generators": ["description"]
  }'
```

### Example 2: Generate everything except hostname

```bash
curl -X POST http://localhost:8080/api/v0/gen \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "filters": ["switch1.example.com"],
    "exclude_generators": ["hostname"]
  }'
```

### Example 3: Diff only VLAN configuration

```bash
curl -X POST http://localhost:8080/api/v0/diff \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "filters": ["switch1.example.com"],
    "generators": ["vlans"]
  }'
```

## Implementation Details

1. **Backend (Go)**: The `CommandRequest` struct in `/internal/annet/commands.go` includes `Generators` and `ExcludeGenerators` fields that are passed to the annet command using `-g` and `-G` flags respectively.

2. **MCP Server (TypeScript)**: The MCP server in `/mcp-annet-oil/src/index.ts` includes the `exclude_generators` field in both the Zod schema and JSON schema for tool definitions.

3. **MCP Client (TypeScript)**: The client interface in `/mcp-annet-oil/src/client.ts` includes the `exclude_generators` field in the `CommandRequest` interface.

## Common Generator Names

Common generator names that can be filtered (actual names depend on your annet configuration):

- `hostname` - Device hostname configuration
- `description` - Interface descriptions
- `interfaces` - Interface configuration
- `routing` - Routing protocols (BGP, OSPF, etc.)
- `vlans` - VLAN configuration
- `acl` - Access control lists
- `qos` - Quality of Service configuration
- `snmp` - SNMP configuration
- `logging` - Logging configuration
- `ntp` - NTP configuration
- `dns` - DNS configuration

Check your annet configuration for the complete list of available generators.