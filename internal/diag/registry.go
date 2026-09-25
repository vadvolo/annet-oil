package diag

import "annet-oil/internal/diag/diagpb"

// Step is one command in a playbook. Command may contain placeholders
// <PORT>, <PEER>, <CILJ>, <lok> which the engine substitutes from the alarm.
// A step whose command still has an unresolved placeholder after substitution
// is skipped (recorded as STEP_SKIPPED). Category names the semantic role and
// selects the parser via the parse registry.
type Step struct {
	Command  string
	Category string
}

func s(cmd, cat string) Step { return Step{Command: cmd, Category: cat} }

// AF is the family key type for the registry.
type AF = diagpb.Family

const (
	fXR  = diagpb.Family_FAMILY_IOSXR
	fIOS = diagpb.Family_FAMILY_IOS
	fEL  = diagpb.Family_FAMILY_ELTEX
	fROS = diagpb.Family_FAMILY_ROUTEROS
	fFS  = diagpb.Family_FAMILY_FS
)

// playbooks maps alarm -> family -> ordered command steps. Derived verbatim from
// the operational playbook catalog. Availability probe pinging (from the MikroTik
// probes to device/last-hop/core-anchors) is handled additionally by the engine.
var playbooks = map[diagpb.AlarmType]map[AF][]Step{
	// 1. Device availability -------------------------------------------------
	diagpb.AlarmType_ALARM_AVAILABILITY: {
		fXR: {
			s("show version", "facts"),
			s("ping <CILJ>", "ping"),
			s("show route <CILJ>", "routes"),
			s("show arp", "arp"),
			s("show interfaces", "interfaces"),
			s("show interfaces summary", "interfaces_summary"),
			s("show logging", "log"),
			s("show lldp neighbors detail", "lldp"),
			s("show platform", "platform"),
		},
		fIOS: {
			s("show version", "facts"),
			s("ping <CILJ>", "ping"),
			s("show ip route <CILJ>", "routes"),
			s("show ip arp", "arp"),
			s("show interfaces", "interfaces"),
			s("show interfaces counters errors", "interface_errors"),
			s("show logging", "log"),
			s("show lldp neighbors detail", "lldp"),
			s("show cdp neighbors detail", "cdp"),
			s("show spanning-tree detail", "stp"),
			s("show environment", "env"),
		},
		fEL: {
			s("show version", "facts"),
			s("ping <CILJ>", "ping"),
			s("show ip route", "routes"),
			s("show arp", "arp"),
			s("show interfaces", "interfaces"),
			s("show interfaces counters", "interface_errors"),
			s("show logging", "log"),
			s("show lldp neighbors detail", "lldp"),
			s("show spanning-tree", "stp"),
			s("show system", "env"),
		},
		fROS: {
			s("/system identity print", "identity"),
			s("/ping <CILJ> count=5", "ping"),
			s("/ip route print where dst-address in <CILJ>", "routes"),
			s("/ip arp print", "arp"),
			s("/interface print", "interfaces"),
			s("/log print", "log"),
			s("/ip neighbor print detail", "lldp"),
			s("/interface bridge port print", "bridge_port"),
			s("/system health print", "env"),
			s("/system resource print", "resource"),
			s("/snmp print", "snmp"),
			s("/ip firewall filter print", "firewall"),
			s("/ip firewall raw print", "firewall_raw"),
			s(`/log print where topics~"snmp"`, "log_snmp"),
		},
		fFS: {
			s("show version", "facts"),
			s("ping <CILJ>", "ping"),
			s("show ip route", "routes"),
			s("show arp", "arp"),
			s("show interface status", "interfaces"),
			s("show interface counters", "interface_errors"),
			s("show logging", "log"),
			s("show lldp neighbors detail", "lldp"),
			s("show spanning-tree", "stp"),
			s("show system", "env"),
			s("show interface transceiver diagnosis", "optics"),
			s("show temperature", "env"),
			s("show power", "env"),
			s("show fan speed", "env"),
		},
	},

	// 2. Port / optics / load / errors / speed ------------------------------
	diagpb.AlarmType_ALARM_PORT_OPTICS: {
		fXR: {
			s("show interfaces <PORT>", "interface"),
			s("show controllers optics <lok>", "optics"),
			s("show logging", "log"),
			s("show lldp neighbors detail", "lldp"),
			s("show ip interface brief", "interfaces"),
			s("show cef interface", "cef"),
			s("show interfaces <PORT> accounting", "interface_accounting"),
		},
		fIOS: {
			s("show interfaces <PORT>", "interface"),
			s("show interfaces <PORT> transceiver detail", "optics"),
			s("show logging", "log"),
			s("show lldp neighbors detail", "lldp"),
		},
		fEL: {
			s("show interfaces <PORT>", "interface"),
			s("show fiber-ports optical-transceiver detailed", "optics"),
			s("show logging", "log"),
			s("show lldp neighbors", "lldp"),
		},
		fROS: {
			s("/interface ethernet monitor <PORT> once", "optics"),
			s("/interface ethernet print stats-detail", "interface_stats"),
			s("/log print", "log"),
			s("/ip neighbor print detail", "lldp"),
		},
		fFS: {
			s("show interface <PORT> status", "interface"),
			s("show interface <PORT> counters", "interface_errors"),
			s("show interface transceiver diagnosis", "optics"),
			s("show logging", "log"),
			s("show lldp neighbors detail", "lldp"),
		},
	},

	// 3. STP / L2 loop -------------------------------------------------------
	diagpb.AlarmType_ALARM_STP: {
		fIOS: {
			s("show spanning-tree summary", "stp"),
			s("show spanning-tree detail", "stp"),
			s("show spanning-tree blockedports", "stp_blocked"),
			s("show mac address-table dynamic", "mac"),
			s("show logging", "log"),
			s("show spanning-tree interface <PORT>", "stp"),
			s("show interfaces <PORT>", "interface"),
		},
		fEL: {
			s("show spanning-tree", "stp"),
			s("show spanning-tree active", "stp_blocked"),
			s("show mac address-table", "mac"),
			s("show logging", "log"),
			s("show spanning-tree interface <PORT>", "stp"),
		},
		fFS: {
			s("show spanning-tree", "stp"),
			s("show spanning-tree blockedports", "stp_blocked"),
			s("show mac address-table", "mac"),
			s("show logging", "log"),
			s("show spanning-tree interface <PORT>", "stp"),
		},
		fROS: {
			s("/interface bridge port print", "bridge_port"),
			s("/interface bridge host print", "bridge_host"),
			s("/interface bridge monitor [find]", "bridge_monitor"),
			s("/log print", "log"),
		},
	},

	// 4. OSPF ----------------------------------------------------------------
	diagpb.AlarmType_ALARM_OSPF: {
		fXR: {
			s("show ospf neighbor", "ospf"),
			s("show ospf neighbor <PEER> detail", "ospf"),
			s("show ospf interface brief", "ospf_interface"),
			s("show route <PEER>", "routes"),
			s("show logging", "log"),
		},
		fIOS: {
			s("show ip ospf neighbor", "ospf"),
			s("show ip ospf neighbor <PEER>", "ospf"),
			s("show ip ospf interface brief", "ospf_interface"),
			s("show ip route <PEER>", "routes"),
			s("show logging", "log"),
		},
		fEL: {
			s("show ip ospf neighbor", "ospf"),
			s("show ip ospf neighbor <PEER>", "ospf"),
			s("show ip ospf interface brief", "ospf_interface"),
			s("show ip route <PEER>", "routes"),
			s("show logging", "log"),
		},
		fFS: {
			s("show ip ospf neighbor", "ospf"),
			s("show ip ospf neighbor <PEER>", "ospf"),
			s("show ip ospf interface brief", "ospf_interface"),
			s("show ip route <PEER>", "routes"),
			s("show logging", "log"),
		},
		fROS: {
			s("/routing ospf neighbor print detail", "ospf"),
			s("/ip route print where gateway=<PEER>", "routes"),
			s("/ip arp print", "arp"),
			s("/log print", "log"),
		},
	},

	// 5. BGP -----------------------------------------------------------------
	diagpb.AlarmType_ALARM_BGP: {
		fXR: {
			s("show bgp neighbor <PEER>", "bgp"),
			s("show bgp summary", "bgp"),
		},
		fIOS: {
			s("show ip bgp neighbors <PEER>", "bgp"),
			s("show ip bgp summary", "bgp"),
		},
		fEL: {
			s("show ip bgp neighbors <PEER>", "bgp"),
			s("show ip bgp summary", "bgp"),
		},
		fFS: {
			s("show ip bgp neighbors <PEER>", "bgp"),
			s("show ip bgp summary", "bgp"),
		},
		fROS: {
			s("/routing bgp peer print status", "bgp"),       // RouterOS v6
			s("/routing/bgp/connection/print status", "bgp"), // RouterOS v7
			s("/routing/bgp/session/print status", "bgp"),    // RouterOS v7 sessions
		},
	},

	// 6. BFD -----------------------------------------------------------------
	diagpb.AlarmType_ALARM_BFD: {
		fXR: {
			s("show bfd session", "bfd"),
			s("show bfd session detail", "bfd"),
			s("show logging", "log"),
		},
		fIOS: {
			s("show bfd neighbors", "bfd"),
			s("show bfd neighbors details", "bfd"),
			s("show logging", "log"),
		},
		fEL: {
			s("show bfd neighbors", "bfd"),
			s("show bfd neighbors details", "bfd"),
			s("show logging", "log"),
		},
		fFS: {
			s("show bfd neighbors", "bfd"),
			s("show bfd neighbors details", "bfd"),
			s("show logging", "log"),
		},
		fROS: {
			s("/routing bfd session print detail", "bfd"),
			s("/log print", "log"),
		},
	},

	// 7. IS-IS ---------------------------------------------------------------
	diagpb.AlarmType_ALARM_ISIS: {
		fXR: {
			s("show isis neighbors", "isis"),
			s("show isis neighbors detail", "isis"),
			s("show isis interface brief", "isis_interface"),
			s("show logging", "log"),
		},
		fIOS: {
			s("show isis neighbors", "isis"),
			s("show isis neighbors detail", "isis"),
			s("show clns interface", "isis_interface"),
			s("show logging", "log"),
		},
		fEL: {
			s("show isis neighbors", "isis"),
			s("show isis neighbors detail", "isis"),
			s("show clns interface", "isis_interface"),
			s("show logging", "log"),
		},
		fFS: {
			s("show isis neighbors", "isis"),
			s("show isis neighbors detail", "isis"),
			s("show clns interface", "isis_interface"),
			s("show logging", "log"),
		},
		fROS: {
			s("/routing isis neighbor print detail", "isis"),
			s("/ip route print", "routes"),
			s("/log print", "log"),
		},
	},

	// 8. Redundancy (VRRP/HSRP) ---------------------------------------------
	diagpb.AlarmType_ALARM_REDUNDANCY: {
		fXR: {
			s("show vrrp brief", "redundancy"),
			s("show vrrp detail", "redundancy"),
			s("show hsrp brief", "redundancy"),
			s("show hsrp detail", "redundancy"),
			s("show interfaces brief", "interfaces"),
			s("show track", "track"),
			s("show logging", "log"),
		},
		fIOS: {
			s("show standby brief", "redundancy"),
			s("show standby", "redundancy"),
			s("show ip interface brief", "interfaces"),
			s("show track", "track"),
			s("show logging", "log"),
		},
		fEL: {
			s("show standby brief", "redundancy"),
			s("show standby", "redundancy"),
			s("show ip interface brief", "interfaces"),
			s("show track", "track"),
			s("show logging", "log"),
		},
		fFS: {
			s("show standby brief", "redundancy"),
			s("show standby", "redundancy"),
			s("show ip interface brief", "interfaces"),
			s("show track", "track"),
			s("show logging", "log"),
		},
		fROS: {
			s("/interface vrrp print detail", "redundancy"),
			s("/interface print", "interfaces"),
			s("/ip address print", "addresses"),
			s("/ip route print", "routes"),
			s("/log print", "log"),
		},
	},

	// 9. MPLS LDP ------------------------------------------------------------
	diagpb.AlarmType_ALARM_MPLS_LDP: {
		fXR: {
			s("show mpls ldp neighbor", "ldp"),
			s("show mpls ldp neighbor detail", "ldp"),
			s("show mpls ldp discovery", "ldp_discovery"),
			s("show mpls ldp interface", "ldp_interface"),
			s("show mpls forwarding summary", "mpls_forwarding"),
			s("show logging", "log"),
		},
		fIOS: {
			s("show mpls ldp neighbor", "ldp"),
			s("show mpls ldp discovery", "ldp_discovery"),
			s("show mpls forwarding-table", "mpls_forwarding"),
			s("show logging", "log"),
		},
		fROS: {
			s("/mpls ldp neighbor print detail", "ldp"),
			s("/mpls ldp interface print", "ldp_interface"),
			s("/mpls forwarding-table print", "mpls_forwarding"),
			s("/log print", "log"),
		},
	},

	// 10. L2VPN / pseudowire -------------------------------------------------
	diagpb.AlarmType_ALARM_L2VPN: {
		fXR: {
			s("show l2vpn xconnect", "l2vpn"),
			s("show l2vpn xconnect detail", "l2vpn"),
			s("show l2vpn bridge-domain", "l2vpn_bd"),
			s("show mpls ldp neighbor", "ldp"),
			s("show logging", "log"),
		},
		fIOS: {
			s("show mpls l2transport vc", "l2vpn"),
			s("show mpls l2transport vc detail", "l2vpn"),
			s("show mpls ldp neighbor", "ldp"),
			s("show logging", "log"),
		},
		fROS: {
			s("/interface vpls print detail", "l2vpn"),
			s("/mpls ldp neighbor print detail", "ldp"),
			s("/mpls forwarding-table print", "mpls_forwarding"),
			s("/interface print", "interfaces"),
			s("/log print", "log"),
		},
	},

	// 11. CPU / memory -------------------------------------------------------
	diagpb.AlarmType_ALARM_CPU_MEM: {
		fXR: {
			s("show processes cpu", "cpu"),
			s("show processes cpu location 0/RP0/CPU0", "cpu"),
			s("show memory summary", "memory"),
			s("show processes memory", "memory"),
			s("show redundancy", "redundancy_rp"),
			s("show logging", "log"),
		},
		fIOS: {
			s("show processes cpu sorted", "cpu"),
			s("show processes memory", "memory"),
			s("show logging", "log"),
		},
		fEL: {
			s("show cpu utilization", "cpu"),
			s("show memory", "memory"),
		},
		fFS: {
			s("show processes cpu", "cpu"),
			s("show memory", "memory"),
		},
		fROS: {
			s("/system resource print", "resource"),
			s("/tool profile duration=5", "profile"),
			s("/log print", "log"),
		},
	},

	// 12. Hardware / environment --------------------------------------------
	diagpb.AlarmType_ALARM_HARDWARE_ENV: {
		fXR: {
			s("show platform", "platform"),
			s("show inventory", "inventory"),
			s("show environment all", "env"),
			s("show logging", "log"),
		},
		fIOS: {
			s("show environment", "env"),
			s("show power", "env"),
			s("show inventory", "inventory"),
			s("show logging", "log"),
		},
		fEL: {
			s("show environment", "env"),
			s("show power", "env"),
			s("show fan", "env"),
			s("show system", "env"),
		},
		fFS: {
			s("show environment", "env"),
			s("show power", "env"),
			s("show fan speed", "env"),
			s("show system", "env"),
		},
		fROS: {
			s("/system health print", "env"),
			s("/system resource print", "resource"),
			s("/system routerboard print", "routerboard"),
			s("/interface ethernet print", "interfaces"),
			s("/interface print", "interfaces"),
			s("/log print", "log"),
		},
	},

	// 13. Restart / uptime ---------------------------------------------------
	diagpb.AlarmType_ALARM_RESTART_UPTIME: {
		fXR: {
			s("show version", "facts"),
			s("show context", "context"),
			s("show logging", "log"),
		},
		fIOS: {
			s("show version", "facts"),
			s("show logging", "log"),
		},
		fEL: {
			s("show version", "facts"),
			s("show system", "env"),
			s("show logging", "log"),
		},
		fFS: {
			s("show version", "facts"),
			s("show system", "env"),
			s("show logging", "log"),
		},
		fROS: {
			s("/system resource print", "resource"),
			s("/log print", "log"),
		},
	},
}

// gaps documents alarm+family combinations that intentionally have no playbook,
// with the manual step / reason shown on the card.
var gaps = map[diagpb.AlarmType]map[AF]string{
	diagpb.AlarmType_ALARM_STP: {
		fXR: "No STP playbook on IOS-XR — inspect L2 manually on the attached switch.",
	},
	diagpb.AlarmType_ALARM_MPLS_LDP: {
		fEL: "No MPLS LDP playbook on Eltex — manual step.",
		fFS: "No MPLS LDP playbook on FS — manual step.",
	},
	diagpb.AlarmType_ALARM_L2VPN: {
		fEL: "No L2VPN playbook on Eltex — manual step.",
		fFS: "No L2VPN playbook on FS — manual step.",
	},
}

// AllFamilies lists the concrete device families that have playbooks.
var AllFamilies = []diagpb.Family{fXR, fIOS, fEL, fROS, fFS}

// AllAlarms lists every alarm type in catalog order.
var AllAlarms = []diagpb.AlarmType{
	diagpb.AlarmType_ALARM_AVAILABILITY,
	diagpb.AlarmType_ALARM_PORT_OPTICS,
	diagpb.AlarmType_ALARM_STP,
	diagpb.AlarmType_ALARM_OSPF,
	diagpb.AlarmType_ALARM_BGP,
	diagpb.AlarmType_ALARM_BFD,
	diagpb.AlarmType_ALARM_ISIS,
	diagpb.AlarmType_ALARM_REDUNDANCY,
	diagpb.AlarmType_ALARM_MPLS_LDP,
	diagpb.AlarmType_ALARM_L2VPN,
	diagpb.AlarmType_ALARM_CPU_MEM,
	diagpb.AlarmType_ALARM_HARDWARE_ENV,
	diagpb.AlarmType_ALARM_RESTART_UPTIME,
}

// Playbook returns the steps for an alarm+family, and any documented gap note.
func Playbook(a diagpb.AlarmType, f diagpb.Family) (steps []Step, gap string) {
	if byFam, ok := playbooks[a]; ok {
		steps = byFam[f]
	}
	if byFam, ok := gaps[a]; ok {
		gap = byFam[f]
	}
	return steps, gap
}
