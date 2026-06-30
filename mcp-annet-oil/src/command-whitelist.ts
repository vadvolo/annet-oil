export interface CommandWhitelistConfig {
  patterns: RegExp[];
  description: string;
}

// Whitelist configuration for allowed commands
export const COMMAND_WHITELIST: CommandWhitelistConfig[] = [
  // Show commands - safe read-only operations
  {
    patterns: [
      /^show\s+version$/i,
      /^show\s+inventory$/i,
      /^show\s+interfaces?(\s+status)?$/i,
      /^show\s+interfaces?\s+brief$/i,
      /^show\s+interfaces?\s+description$/i,
      /^show\s+interfaces?\s+\S+$/i,
      /^show\s+ip\s+interfaces?(\s+brief)?$/i,
      /^show\s+ipv6\s+interfaces?(\s+brief)?$/i,
    ],
    description: 'Interface information commands',
  },
  {
    patterns: [
      /^show\s+running-config$/i,
      /^show\s+startup-config$/i,
      /^show\s+config$/i,
      /^show\s+configuration$/i,
      /^show\s+running-config\s+interface\s+\S+$/i,
      /^show\s+running-config\s+\|\s+section\s+\S+$/i,
    ],
    description: 'Configuration display commands',
  },
  {
    patterns: [
      /^show\s+ip\s+route$/i,
      /^show\s+ipv6\s+route$/i,
      /^show\s+ip\s+route\s+\S+$/i,
      /^show\s+ipv6\s+route\s+\S+$/i,
      /^show\s+ip\s+bgp$/i,
      /^show\s+ip\s+bgp\s+summary$/i,
      /^show\s+ip\s+bgp\s+neighbors?$/i,
      /^show\s+ip\s+ospf$/i,
      /^show\s+ip\s+ospf\s+neighbors?$/i,
      /^show\s+ip\s+eigrp\s+neighbors?$/i,
    ],
    description: 'Routing protocol commands',
  },
  {
    patterns: [
      /^show\s+vlan$/i,
      /^show\s+vlan\s+brief$/i,
      /^show\s+vlan\s+id\s+\d+$/i,
      /^show\s+spanning-tree$/i,
      /^show\s+spanning-tree\s+brief$/i,
      /^show\s+spanning-tree\s+vlan\s+\d+$/i,
      /^show\s+vpc$/i,
      /^show\s+vpc\s+brief$/i,
      /^show\s+port-channel\s+summary$/i,
    ],
    description: 'Layer 2 and switching commands',
  },
  {
    patterns: [
      /^show\s+mac\s+address-table$/i,
      /^show\s+mac\s+address-table\s+dynamic$/i,
      /^show\s+mac\s+address-table\s+static$/i,
      /^show\s+mac\s+address-table\s+vlan\s+\d+$/i,
      /^show\s+arp$/i,
      /^show\s+ip\s+arp$/i,
      /^show\s+ipv6\s+neighbors?$/i,
    ],
    description: 'MAC and ARP table commands',
  },
  {
    patterns: [
      /^show\s+cdp\s+neighbors?$/i,
      /^show\s+cdp\s+neighbors?\s+detail$/i,
      /^show\s+lldp\s+neighbors?$/i,
      /^show\s+lldp\s+neighbors?\s+detail$/i,
    ],
    description: 'Neighbor discovery commands',
  },
  {
    patterns: [
      /^show\s+logging$/i,
      /^show\s+log$/i,
      /^show\s+logging\s+last\s+\d+$/i,
      /^show\s+tech-support$/i,
      /^show\s+processes\s+cpu$/i,
      /^show\s+processes\s+memory$/i,
      /^show\s+memory$/i,
      /^show\s+environment$/i,
      /^show\s+environment\s+temperature$/i,
      /^show\s+environment\s+power$/i,
      /^show\s+environment\s+fan$/i,
    ],
    description: 'System monitoring and diagnostics',
  },
  {
    patterns: [
      /^show\s+ntp\s+status$/i,
      /^show\s+ntp\s+associations?$/i,
      /^show\s+clock$/i,
      /^show\s+snmp$/i,
      /^show\s+snmp\s+community$/i,
      /^show\s+users?$/i,
      /^show\s+tacacs$/i,
      /^show\s+radius$/i,
      /^show\s+aaa$/i,
    ],
    description: 'Management and security commands',
  },
  {
    patterns: [
      /^show\s+access-lists?$/i,
      /^show\s+ip\s+access-lists?$/i,
      /^show\s+access-lists?\s+\S+$/i,
      /^show\s+firewall$/i,
      /^show\s+crypto$/i,
      /^show\s+crypto\s+ipsec$/i,
      /^show\s+crypto\s+isakmp$/i,
    ],
    description: 'Security and ACL commands',
  },
  // Diagnostic and ping commands
  {
    patterns: [
      /^ping\s+[\d\.]+$/i,
      /^ping\s+[a-fA-F0-9:]+$/i,
      /^ping\s+\S+$/i,
      /^traceroute\s+[\d\.]+$/i,
      /^traceroute\s+[a-fA-F0-9:]+$/i,
      /^traceroute\s+\S+$/i,
    ],
    description: 'Network connectivity testing',
  },
];

export class CommandValidator {
  private whitelist: CommandWhitelistConfig[];

  constructor(customWhitelist?: CommandWhitelistConfig[]) {
    this.whitelist = customWhitelist || COMMAND_WHITELIST;
  }

  /**
   * Validate if a command is allowed based on whitelist patterns
   */
  isAllowed(command: string): boolean {
    const trimmedCommand = command.trim();

    // Check against all whitelist patterns
    for (const config of this.whitelist) {
      for (const pattern of config.patterns) {
        if (pattern.test(trimmedCommand)) {
          return true;
        }
      }
    }

    return false;
  }

  /**
   * Get all allowed command categories
   */
  getCategories(): string[] {
    return this.whitelist.map(config => config.description);
  }

  /**
   * Validate multiple commands and return validation results
   */
  validateCommands(commands: string[]): {
    allowed: string[];
    blocked: string[];
  } {
    const allowed: string[] = [];
    const blocked: string[] = [];

    for (const command of commands) {
      if (this.isAllowed(command)) {
        allowed.push(command);
      } else {
        blocked.push(command);
      }
    }

    return { allowed, blocked };
  }

  /**
   * Add custom patterns to the whitelist
   */
  addPatterns(patterns: RegExp[], description: string): void {
    this.whitelist.push({ patterns, description });
  }

  /**
   * Get suggested commands based on partial input
   */
  getSuggestions(partial: string): string[] {
    const suggestions: string[] = [];
    const lowerPartial = partial.toLowerCase();

    // Common show commands that match the partial input
    const commonCommands = [
      'show version',
      'show interfaces',
      'show ip route',
      'show running-config',
      'show vlan',
      'show ip bgp summary',
      'show logging',
      'show cdp neighbors',
    ];

    return commonCommands.filter(cmd =>
      cmd.toLowerCase().startsWith(lowerPartial)
    );
  }
}