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
import { AnnetOilClient, CommandRequest, CommandResponse } from './client.js';

dotenv.config();

const API_URL = process.env.ANNET_OIL_API_URL || 'http://localhost:8080';
const AUTH_TOKEN = process.env.ANNET_OIL_AUTH_TOKEN || 'change-me-in-production';

const annetClient = new AnnetOilClient({
  apiUrl: API_URL,
  authToken: AUTH_TOKEN,
  timeout: 60000,
});

const CommandRequestSchema = z.object({
  filters: z.array(z.string()).optional().describe('Device hostnames or patterns to target'),
  generators: z.array(z.string()).optional().describe('Generator filters to apply (e.g., interfaces, routing)'),
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

async function main() {
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
    return { tools };
  });

  server.setRequestHandler(CallToolRequestSchema, async (request) => {
    const { name, arguments: args } = request.params;

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

  console.error('MCP Annet Oil Server started');
  console.error(`API URL: ${API_URL}`);
}

main().catch((error) => {
  console.error('Server error:', error);
  process.exit(1);
});