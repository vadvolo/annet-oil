import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  TextContent,
  Tool,
} from '@modelcontextprotocol/sdk/types.js';
import { z } from 'zod';
import dotenv from 'dotenv';
import { AnnetOilClient, CommandRequest, CommandResponse, CheckResult } from './client.js';
import { CommandValidator } from './command-whitelist.js';
import { logger, createRequestLogger } from './logger.js';

dotenv.config();

const API_URL = process.env.ANNET_OIL_API_URL || 'http://localhost:8080';
const AUTH_TOKEN = process.env.ANNET_OIL_AUTH_TOKEN || 'change-me-in-production';

logger.info({ api_url: API_URL }, 'MCP Server starting');

const annetClient = new AnnetOilClient({
  apiUrl: API_URL,
  authToken: AUTH_TOKEN,
  timeout: 60000,
});

const CommandRequestSchema = z.object({
  filters: z.array(z.string()).optional().describe('Device hostnames or patterns to target'),
  generators: z.array(z.string()).optional().describe('Generator filters to apply (e.g., interfaces, routing)'),
  exclude_generators: z.array(z.string()).optional().describe('Exclude specific generators from execution'),
  container: z.string().optional().describe('Specific container to use'),
  dry_run: z.boolean().optional().describe('Perform dry run without making changes'),
  parallel: z.boolean().optional().describe('Execute in parallel mode'),
  timeout: z.number().optional().describe('Command timeout in seconds'),
  quiet: z.boolean().optional().describe('Suppress stderr warnings'),
});

const commandInputSchema = {
  type: 'object' as const,
  properties: {
    filters: {
      type: 'array',
      items: { type: 'string' },
      description: 'Device hostnames or patterns to target',
    },
    generators: {
      type: 'array',
      items: { type: 'string' },
      description: 'Generator filters to apply (e.g., interfaces, routing)',
    },
    exclude_generators: {
      type: 'array',
      items: { type: 'string' },
      description: 'Exclude specific generators from execution',
    },
    container: {
      type: 'string',
      description: 'Specific container to use',
    },
    dry_run: {
      type: 'boolean',
      description: 'Perform dry run without making changes',
    },
    parallel: {
      type: 'boolean',
      description: 'Execute in parallel mode',
    },
    timeout: {
      type: 'number',
      description: 'Command timeout in seconds',
    },
    quiet: {
      type: 'boolean',
      description: 'Suppress stderr warnings',
    },
  },
};

const commandValidator = new CommandValidator();

const tools: Tool[] = [
  {
    name: 'annet_gen',
    description: 'Generate network device configuration using Annet',
    inputSchema: commandInputSchema,
  },
  {
    name: 'annet_diff',
    description: 'Show configuration differences between generated and current configuration',
    inputSchema: commandInputSchema,
  },
  {
    name: 'annet_patch',
    description: 'Apply configuration patches to network devices',
    inputSchema: commandInputSchema,
  },
  {
    name: 'annet_deploy',
    description: 'Deploy configuration changes to network devices',
    inputSchema: commandInputSchema,
  },
  {
    name: 'annet_containers',
    description: 'Get status of Annet containers',
    inputSchema: {
      type: 'object' as const,
      properties: {},
    },
  },
  {
    name: 'annet_routing',
    description: 'Get routing information for devices',
    inputSchema: {
      type: 'object' as const,
      properties: {
        hostname: {
          type: 'string',
          description: 'Specific hostname to check routing for',
        },
      },
    },
  },
  {
    name: 'annet_health',
    description: 'Check health status of Annet Oil API',
    inputSchema: {
      type: 'object' as const,
      properties: {},
    },
  },
  {
    name: 'annet_execute',
    description: 'Execute whitelisted show/diagnostic commands on network devices',
    inputSchema: {
      type: 'object' as const,
      properties: {
        command: {
          type: 'string',
          description: 'Command to execute (must be whitelisted - show commands only)',
        },
        host: {
          type: 'string',
          description: 'Device hostname or IP address to execute command on',
        },
        filters: {
          type: 'array',
          items: { type: 'string' },
          description: 'Device hostnames or patterns to target (alternative to host parameter)',
        },
        container: {
          type: 'string',
          description: 'Specific container to use',
        },
        timeout: {
          type: 'number',
          description: 'Command timeout in seconds',
        },
      },
      required: ['command'],
    },
  },
  {
    name: 'annet_list_allowed_commands',
    description: 'List categories of allowed commands that can be executed',
    inputSchema: {
      type: 'object' as const,
      properties: {},
    },
  },
  {
    name: 'annet_inventory',
    description: 'List network devices from inventory with optional filtering',
    inputSchema: {
      type: 'object' as const,
      properties: {
        vendor: {
          type: 'string',
          description: 'Filter by vendor (e.g., cisco, juniper, arista)',
        },
        platform: {
          type: 'string',
          description: 'Filter by platform (e.g., ios, eos, junos)',
        },
        pattern: {
          type: 'string',
          description: 'Filter by hostname pattern (supports wildcards like lab-*)',
        },
      },
    },
  },
  {
    name: 'annet_check',
    description: 'Check device availability: probe configured ports (22, 23, 10022, 21022, ...) and verify SSH login. Returns reachability, open ports, login status, timestamp and any error. Address the device by hostname or IP.',
    inputSchema: {
      type: 'object' as const,
      properties: {
        host: {
          type: 'string',
          description: 'Device hostname or IP address to check',
        },
        ports: {
          type: 'array',
          items: { type: 'number' },
          description: 'Extra TCP ports to probe (the device\'s own inventory port is always included)',
        },
        login: {
          type: 'boolean',
          description: 'Attempt SSH login verification (default: true)',
        },
        timeout: {
          type: 'number',
          description: 'Per-port TCP dial timeout in seconds',
        },
      },
      required: ['host'],
    },
  },
  {
    name: 'annet_rfc_create',
    description: 'Create a new RFC (Request for Change) ticket in Jira for network configuration changes',
    inputSchema: {
      type: 'object' as const,
      properties: {
        summary: {
          type: 'string',
          description: 'RFC ticket summary/title',
        },
        description: {
          type: 'string',
          description: 'Detailed description of the change',
        },
        devices: {
          type: 'array',
          items: { type: 'string' },
          description: 'List of devices affected by this change',
        },
        priority: {
          type: 'string',
          description: 'Priority level (Low, Medium, High, Critical)',
        },
      },
      required: ['summary', 'devices'],
    },
  },
  {
    name: 'annet_rfc_attach_diff',
    description: 'Attach configuration diff to an existing RFC ticket',
    inputSchema: {
      type: 'object' as const,
      properties: {
        ticket_key: {
          type: 'string',
          description: 'Jira ticket key (e.g., NET-123)',
        },
        device: {
          type: 'string',
          description: 'Device hostname',
        },
        diff: {
          type: 'string',
          description: 'Configuration diff content',
        },
      },
      required: ['ticket_key', 'device', 'diff'],
    },
  },
  {
    name: 'annet_rfc_status',
    description: 'Get RFC ticket status and details',
    inputSchema: {
      type: 'object' as const,
      properties: {
        ticket_key: {
          type: 'string',
          description: 'Jira ticket key (e.g., NET-123)',
        },
      },
      required: ['ticket_key'],
    },
  },
  {
    name: 'annet_rfc_submit',
    description: 'Submit RFC for approval (transition to review status)',
    inputSchema: {
      type: 'object' as const,
      properties: {
        ticket_key: {
          type: 'string',
          description: 'Jira ticket key (e.g., NET-123)',
        },
        comment: {
          type: 'string',
          description: 'Optional comment when submitting for review',
        },
      },
      required: ['ticket_key'],
    },
  },
  {
    name: 'annet_rfc_close',
    description: 'Close RFC ticket after successful deployment',
    inputSchema: {
      type: 'object' as const,
      properties: {
        ticket_key: {
          type: 'string',
          description: 'Jira ticket key (e.g., NET-123)',
        },
        resolution: {
          type: 'string',
          description: 'Resolution comment (e.g., "Deployed successfully")',
        },
      },
      required: ['ticket_key'],
    },
  },
];

function formatCommandResponse(response: CommandResponse): string {
  let output = `Command ${response.success ? 'succeeded' : 'failed'}\n`;
  output += `Total hosts: ${response.total_hosts}, Success: ${response.success_hosts}, Failed: ${response.failed_hosts}\n\n`;

  if (response.error) {
    output += `Error: ${response.error}\n\n`;
  }

  if (response.results) {
    for (const [hostname, result] of Object.entries(response.results)) {
      output += `=== ${hostname} ===\n`;
      output += `Container: ${result.container}\n`;
      output += `Exit Code: ${result.exit_code}\n`;

      if (result.stdout) {
        output += `\nOutput:\n${result.stdout}\n`;
      }

      if (result.stderr) {
        output += `\nWarnings/Errors:\n${result.stderr}\n`;
      }

      if (result.error) {
        output += `\nError: ${result.error}\n`;
      }

      output += '\n';
    }
  }

  return output;
}

function formatCheckResult(result: CheckResult): string {
  let output = `Device Check: ${result.hostname}`;
  if (result.ip && result.ip !== result.hostname) {
    output += ` (${result.ip})`;
  }
  output += '\n\n';
  output += `Timestamp:  ${result.timestamp}\n`;
  output += `Reachable:  ${result.reachable ? 'yes' : 'no'}\n`;
  output += `Login:      ${result.login}\n`;
  output += `Duration:   ${result.duration_ms}ms\n`;

  output += '\nPorts:\n';
  for (const p of result.ports) {
    if (p.open) {
      output += `  ${p.port}: open${p.latency_ms !== undefined ? ` (${p.latency_ms}ms)` : ''}\n`;
    } else {
      output += `  ${p.port}: closed${p.error ? ` (${p.error})` : ''}\n`;
    }
  }

  if (result.error) {
    output += `\nError [${result.error.type}]: ${result.error.message}\n`;
  }

  return output;
}

async function main() {
  logger.debug('Initializing MCP server');
  
  const server = new Server(
    {
      name: 'mcp-annet-oil',
      version: '1.0.0',
    },
    {
      capabilities: {
        tools: {},
      },
    }
  );

  server.setRequestHandler(ListToolsRequestSchema, async () => {
    logger.debug({ tools: tools.map(t => t.name) }, 'ListTools request');
    return { tools };
  });

  server.setRequestHandler(CallToolRequestSchema, async (request) => {
    const { name, arguments: args } = request.params;
    const reqLogger = createRequestLogger();
    reqLogger.info({ tool: name, args }, 'CallTool request');

    try {
      switch (name) {
        case 'annet_gen': {
          const params = CommandRequestSchema.parse(args);
          const response = await annetClient.gen(params as CommandRequest);
          return {
            content: [
              {
                type: 'text',
                text: formatCommandResponse(response),
              } as TextContent,
            ],
          };
        }

        case 'annet_diff': {
          const params = CommandRequestSchema.parse(args);
          const response = await annetClient.diff(params as CommandRequest);
          return {
            content: [
              {
                type: 'text',
                text: formatCommandResponse(response),
              } as TextContent,
            ],
          };
        }

        case 'annet_patch': {
          const params = CommandRequestSchema.parse(args);
          const response = await annetClient.patch(params as CommandRequest);
          return {
            content: [
              {
                type: 'text',
                text: formatCommandResponse(response),
              } as TextContent,
            ],
          };
        }

        case 'annet_deploy': {
          const params = CommandRequestSchema.parse(args);
          const response = await annetClient.deploy(params as CommandRequest);
          return {
            content: [
              {
                type: 'text',
                text: formatCommandResponse(response),
              } as TextContent,
            ],
          };
        }

        case 'annet_containers': {
          const containers = await annetClient.getContainers();
          let output = 'Annet Container Status:\n\n';

          for (const [name, status] of Object.entries(containers)) {
            output += `Container: ${name}\n`;
            output += `  Container Name: ${status.container_name}\n`;
            output += `  Running: ${status.running ? 'Yes' : 'No'}\n`;
            output += `  Configured: ${status.configured ? 'Yes' : 'No'}\n`;
            output += `  Status: ${status.status}\n`;
            if (status.error) {
              output += `  Error: ${status.error}\n`;
            }
            output += '\n';
          }

          return {
            content: [
              {
                type: 'text',
                text: output,
              } as TextContent,
            ],
          };
        }

        case 'annet_routing': {
          const { hostname } = args as { hostname?: string };
          const routing = await annetClient.getRouting(hostname);

          let output = 'Routing Information:\n\n';

          if (Array.isArray(routing)) {
            for (const route of routing) {
              output += `Hostname: ${route.hostname}\n`;
              if (route.container) {
                output += `  Container: ${route.container}\n`;
              }
              if (route.routes && route.routes.length > 0) {
                output += '  Routes:\n';
                for (const r of route.routes) {
                  output += `    ${r.prefix} -> ${r.container}\n`;
                }
              }
              output += '\n';
            }
          } else {
            output += `Hostname: ${routing.hostname}\n`;
            if (routing.container) {
              output += `  Container: ${routing.container}\n`;
            } else {
              output += '  Container: (default)\n';
            }
          }

          return {
            content: [
              {
                type: 'text',
                text: output,
              } as TextContent,
            ],
          };
        }

        case 'annet_health': {
          const health = await annetClient.health();
          return {
            content: [
              {
                type: 'text',
                text: `Annet Oil API Health: ${health.status}`,
              } as TextContent,
            ],
          };
        }

        case 'annet_execute': {
          const { command, host, filters, container, timeout } = args as {
            command: string;
            host?: string;
            filters?: string[];
            container?: string;
            timeout?: number;
          };

          // Validate the command against whitelist
          if (!commandValidator.isAllowed(command)) {
            return {
              content: [
                {
                  type: 'text',
                  text: `Command not allowed: "${command}"\n\nOnly whitelisted show and diagnostic commands are permitted.\nUse 'annet_list_allowed_commands' to see allowed command categories.`,
                } as TextContent,
              ],
              isError: true,
            };
          }

          // If host is provided directly, use it as filters
          const effectiveFilters = host ? [host] : filters;

          const request: CommandRequest = {
            command,
            filters: effectiveFilters,
            container,
            timeout,
          };

          const response = await annetClient.executeCommand(request);
          return {
            content: [
              {
                type: 'text',
                text: formatCommandResponse(response),
              } as TextContent,
            ],
          };
        }

        case 'annet_list_allowed_commands': {
          const categories = commandValidator.getCategories();
          let output = 'Allowed Command Categories:\n\n';
          categories.forEach((category, index) => {
            output += `${index + 1}. ${category}\n`;
          });
          output += '\nExamples of allowed commands:\n';
          output += '  - show version\n';
          output += '  - show interfaces\n';
          output += '  - show ip route\n';
          output += '  - show running-config\n';
          output += '  - show vlan\n';
          output += '  - show ip bgp summary\n';
          output += '  - show logging\n';
          output += '  - ping 192.168.1.1\n';
          output += '  - traceroute 10.0.0.1\n';

          return {
            content: [
              {
                type: 'text',
                text: output,
              } as TextContent,
            ],
          };
        }

        case 'annet_inventory': {
          const { vendor, platform, pattern } = args as {
            vendor?: string;
            platform?: string;
            pattern?: string;
          };

          const inventory = await annetClient.getInventory(vendor, platform, pattern);

          let output = `Network Inventory (${inventory.total} devices):\n\n`;

          for (const device of inventory.devices) {
            output += `${device.hostname}\n`;
            output += `  IP: ${device.ip}\n`;
            output += `  Vendor: ${device.vendor}\n`;
            output += `  Platform: ${device.platform}\n`;
            if (device.description) {
              output += `  Description: ${device.description}\n`;
            }
            output += '\n';
          }

          return {
            content: [
              {
                type: 'text',
                text: output,
              } as TextContent,
            ],
          };
        }

        case 'annet_check': {
          const { host, ports, login, timeout } = args as {
            host: string;
            ports?: number[];
            login?: boolean;
            timeout?: number;
          };

          const result = await annetClient.check({ host, ports, login, timeout });

          return {
            content: [
              {
                type: 'text',
                text: formatCheckResult(result),
              } as TextContent,
            ],
            isError: !result.reachable || result.login === 'failed',
          };
        }

        case 'annet_rfc_create': {
          const { summary, description, devices, priority } = args as {
            summary: string;
            description?: string;
            devices: string[];
            priority?: string;
          };

          const rfc = await annetClient.createRFC({
            summary,
            description,
            devices,
            priority,
          });

          return {
            content: [
              {
                type: 'text',
                text: `RFC Created Successfully\n\nTicket: ${rfc.ticket_key}\nURL: ${rfc.url}\n\nNext steps:\n1. Generate diff with annet_diff for affected devices\n2. Attach diff to RFC with annet_rfc_attach_diff\n3. Submit for review with annet_rfc_submit`,
              } as TextContent,
            ],
          };
        }

        case 'annet_rfc_attach_diff': {
          const { ticket_key, device, diff } = args as {
            ticket_key: string;
            device: string;
            diff: string;
          };

          await annetClient.attachDiffToRFC(ticket_key, device, diff);

          return {
            content: [
              {
                type: 'text',
                text: `Diff attached to ${ticket_key} for device ${device}`,
              } as TextContent,
            ],
          };
        }

        case 'annet_rfc_status': {
          const { ticket_key } = args as { ticket_key: string };

          const status = await annetClient.getRFCStatus(ticket_key);

          let output = `RFC Status: ${ticket_key}\n\n`;
          output += `Summary: ${status.summary}\n`;
          output += `Status: ${status.status}\n`;
          if (status.transitions.length > 0) {
            output += `\nAvailable transitions: ${status.transitions.join(', ')}`;
          }

          return {
            content: [
              {
                type: 'text',
                text: output,
              } as TextContent,
            ],
          };
        }

        case 'annet_rfc_submit': {
          const { ticket_key, comment } = args as {
            ticket_key: string;
            comment?: string;
          };

          await annetClient.submitRFCForReview(ticket_key, comment);

          return {
            content: [
              {
                type: 'text',
                text: `RFC ${ticket_key} submitted for review`,
              } as TextContent,
            ],
          };
        }

        case 'annet_rfc_close': {
          const { ticket_key, resolution } = args as {
            ticket_key: string;
            resolution?: string;
          };

          await annetClient.closeRFC(ticket_key, resolution);

          return {
            content: [
              {
                type: 'text',
                text: `RFC ${ticket_key} closed${resolution ? ': ' + resolution : ''}`,
              } as TextContent,
            ],
          };
        }

        default:
          throw new Error(`Unknown tool: ${name}`);
      }
    } catch (error) {
      return {
        content: [
          {
            type: 'text',
            text: `Error executing ${name}: ${error instanceof Error ? error.message : String(error)}`,
          } as TextContent,
        ],
        isError: true,
      };
    }
  });

  const transport = new StdioServerTransport();
  await server.connect(transport);

  logger.info({ api_url: API_URL }, 'MCP Annet Oil Server started');
}

main().catch((error) => {
  logger.fatal({ error }, 'Server error');
  process.exit(1);
});