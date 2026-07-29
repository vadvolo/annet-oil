// Package opstate collects normalized operational state from network devices.
//
// Raw "show" output is verbose and vendor-specific; feeding it to an AI agent
// wastes tokens and forces the agent to re-parse free text. This package runs a
// small set of read-only commands per vendor, parses them with vendor-specific
// parsers, and returns a single compact, vendor-neutral JSON document — the same
// shape whether the device is a Juniper or a Cisco.
//
// The normalized schema here is deliberately flat and stable so a proto/gRPC
// layer can be generated on top of it without reshaping.
package opstate

import "time"

// StateType names a section of operational state that can be collected
// independently. Cheap sections (facts, interfaces, lldp) are collected by
// default; heavier sections (mac, arp, routes) are opt-in per request.
type StateType string

const (
	Facts      StateType = "facts"
	Interfaces StateType = "interfaces"
	LLDP       StateType = "lldp"
	MAC        StateType = "mac"
	ARP        StateType = "arp"
	Routes     StateType = "routes"
)

// DefaultStates are collected when a request does not name specific sections.
var DefaultStates = []StateType{Facts, Interfaces, LLDP}

// OptionalStates are the heavier sections, collected only when explicitly
// requested (they produce large tables and change often, so they are also the
// ones most worth refreshing with force).
var OptionalStates = []StateType{MAC, ARP, Routes}

// AllStates is every collectable section.
var AllStates = append(append([]StateType{}, DefaultStates...), OptionalStates...)

// State is the normalized operational-state document for one device. Absent
// sections are omitted; per-section collection/parse failures are recorded in
// Errors rather than failing the whole document.
type State struct {
	Host        string         `json:"host"`
	Vendor      string         `json:"vendor"`
	Platform    string         `json:"platform,omitempty"`
	CollectedAt time.Time      `json:"collected_at"`
	Facts       *DeviceFacts   `json:"facts,omitempty"`
	Interfaces  []Interface    `json:"interfaces,omitempty"`
	LLDP        []LLDPNeighbor `json:"lldp_neighbors,omitempty"`
	MAC         []MACEntry     `json:"mac_table,omitempty"`
	ARP         []ARPEntry     `json:"arp_table,omitempty"`
	Routes      []Route        `json:"routes,omitempty"`
	// Sections lists which state types were collected fresh vs served from cache.
	Sections []SectionMeta  `json:"sections,omitempty"`
	Errors   []SectionError `json:"errors,omitempty"`
}

// SectionMeta records how one section was produced.
type SectionMeta struct {
	Type   StateType `json:"type"`
	Cached bool      `json:"cached"`
}

// SectionError records a collection or parse failure for one section.
type SectionError struct {
	Type    StateType `json:"type"`
	Message string    `json:"message"`
}

// DeviceFacts is basic device identity.
type DeviceFacts struct {
	Hostname  string `json:"hostname,omitempty"`
	Vendor    string `json:"vendor,omitempty"`
	Model     string `json:"model,omitempty"`
	Serial    string `json:"serial,omitempty"`
	OSVersion string `json:"os_version,omitempty"`
	Uptime    string `json:"uptime,omitempty"`
}

// Interface is a normalized interface entry. Statuses are lower-cased to
// "up"/"down"; empty fields are omitted.
type Interface struct {
	Name        string   `json:"name"`
	AdminStatus string   `json:"admin_status,omitempty"`
	OperStatus  string   `json:"oper_status,omitempty"`
	Description string   `json:"description,omitempty"`
	MAC         string   `json:"mac,omitempty"`
	MTU         int      `json:"mtu,omitempty"`
	SpeedMbps   int64    `json:"speed_mbps,omitempty"`
	IPv4        []string `json:"ipv4,omitempty"` // CIDR
	IPv6        []string `json:"ipv6,omitempty"` // CIDR
}

// LLDPNeighbor is one discovered neighbor, keyed by the local port.
type LLDPNeighbor struct {
	LocalPort     string `json:"local_port"`
	RemoteSystem  string `json:"remote_system,omitempty"`  // system name
	RemotePort    string `json:"remote_port,omitempty"`    // remote port id / description
	RemoteChassis string `json:"remote_chassis,omitempty"` // chassis id (usually a MAC)
	RemoteMgmtIP  string `json:"remote_mgmt_ip,omitempty"`
}

// MACEntry is one MAC-address-table row.
type MACEntry struct {
	MAC       string `json:"mac"`
	VLAN      int    `json:"vlan,omitempty"`
	Interface string `json:"interface,omitempty"`
	Type      string `json:"type,omitempty"` // dynamic|static
}

// ARPEntry is one ARP/neighbor-table row.
type ARPEntry struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac,omitempty"`
	Interface string `json:"interface,omitempty"`
}

// Route is one routing-table entry.
type Route struct {
	Prefix    string `json:"prefix"`
	NextHop   string `json:"next_hop,omitempty"`
	Interface string `json:"interface,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
}

// apply sets the parsed section into dst. Parsers fill a scratch *State and call
// this so ordering and section bookkeeping live in one place.
func setSection(dst *State, t StateType, src *State) {
	switch t {
	case Facts:
		dst.Facts = src.Facts
	case Interfaces:
		dst.Interfaces = src.Interfaces
	case LLDP:
		dst.LLDP = src.LLDP
	case MAC:
		dst.MAC = src.MAC
	case ARP:
		dst.ARP = src.ARP
	case Routes:
		dst.Routes = src.Routes
	}
}
