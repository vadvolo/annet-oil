export interface CommandWhitelistConfig {
  patterns: RegExp[];
  description: string;
}

// Whitelist configuration for allowed commands
export const COMMAND_WHITELIST: CommandWhitelistConfig[] = [
  // Allow ALL show commands - safe read-only operations
  {
    patterns: [
      /^show\s+.*$/i,  // Matches ANY show command with any parameters
    ],
    description: 'All show commands (read-only operations)',
  },
  // Show commands - safe read-only operations (kept for specific categorization)
  {
    patterns: [
      /^show\s+version$/i,
      /^show\s+inventory$/i,
      // Allow ALL show interfaces commands with any options (including Juniper's extensive, detail, etc.)
      /^show\s+interfaces?\s*.*$/i,  // Matches any show interface(s) command
      /^show\s+ip\s+interfaces?(\s+.*)?$/i,
      /^show\s+ipv6\s+interfaces?(\s+.*)?$/i,
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
      /^show\s+run\s+interface\s+\S+$/i,  // Shortened form
      /^show\s+run\s+int\s+\S+$/i,  // Even shorter form
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
      /^show\s+ip\s+route\s+bgp$/i,
      /^show\s+ip\s+bgp$/i,
      /^show\s+ip\s+bgp\s+summary$/i,
      /^show\s+ip\s+bgp\s+neighbors?$/i,
      /^show\s+ip\s+bgp\s+neighbors?\s+\d{1,3}(\.\d{1,3}){3}$/i,
      /^show\s+ip\s+bgp\s+neighbors?\s+\d{1,3}(\.\d{1,3}){3}\s+(routes|advertised-routes)$/i,
      /^show\s+ip\s+ospf$/i,
      /^show\s+ip\s+ospf\s+neighbors?$/i,
      /^show\s+ip\s+eigrp\s+neighbors?$/i,
      /^show\s+route-map$/i,
      /^show\s+route-map\s+\S+$/i,
    ],
    description: 'Routing protocol commands',
  },
  {
    patterns: [
      /^show\s+vlan$/i,
      /^show\s+vlan\s+brief$/i,
      /^show\s+vlan\s+id\s+\d+$/i,
      
      // Spanning Tree - Cisco
      /^show\s+spanning-tree$/i,
      /^show\s+spanning-tree\s+brief$/i,
      /^show\s+spanning-tree\s+summary$/i,
      /^show\s+spanning-tree\s+detail$/i,
      /^show\s+spanning-tree\s+vlan\s+\d+$/i,
      /^show\s+spanning-tree\s+interface\s+\S+$/i,
      /^show\s+spanning-tree\s+root$/i,
      /^show\s+spanning-tree\s+blockedports$/i,
      /^show\s+spanning-tree\s+inconsistentports$/i,
      /^show\s+spanning-tree\s+mst$/i,
      /^show\s+spanning-tree\s+mst\s+configuration$/i,
      
      // Spanning Tree - Juniper (RSTP/MSTP/VSTP)
      /^show\s+spanning-tree\s+bridge$/i,
      /^show\s+spanning-tree\s+statistics$/i,
      /^show\s+spanning-tree\s+mstp\s+configuration$/i,
      /^show\s+spanning-tree\s+interface$/i,
      /^show\s+spanning-tree\s+interface\s+\S+$/i,
      
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
      /^show\s+ip\s+access-lists?\s+\S+$/i,
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
  // MikroTik RouterOS - read-only commands.
  // RouterOS uses "<path> print" / "export" / "monitor" instead of "show".
  // These are non-mutating debug/monitoring operations. A leading "/" is optional.
  // e.g. "system resource print", "/system resource print", "/export"
  {
    patterns: [
      /^\/?[a-z0-9 /-]+\s+print(\s+.*)?$/i,      // any "... print" command
      /^\/?([a-z0-9 /-]+\s+)?export(\s+.*)?$/i,  // config export (bare "/export" or "<path> export")
      /^\/?[a-z0-9 /-]+\s+monitor(\s+.*)?$/i,    // e.g. interface ethernet monitor
      /^\/?system\s+resource\s+print$/i,         // explicit, for reference
      /^\/?ping\s+\S+.*$/i,                       // RouterOS ping (with optional count/interface args)
      /^\/?tool\s+traceroute\s+\S+.*$/i,         // RouterOS traceroute
    ],
    description: 'MikroTik RouterOS read-only commands',
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
      'show ip route bgp',
      'show running-config',
      'show vlan',
      'show ip bgp',
      'show ip bgp summary',
      'show ip bgp neighbors',
      'show route-map',
      'show ip access-lists',
      'show logging',
      'show cdp neighbors',
      'ping',
    ];

    return commonCommands.filter(cmd =>
      cmd.toLowerCase().startsWith(lowerPartial)
    );
  }
}