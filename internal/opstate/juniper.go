package opstate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// juniperParser parses Junos output. For the default sections (facts,
// interfaces, lldp) it requests native structured output with "| display json",
// which is far more robust than scraping columnar text. The heavier optional
// sections (mac, arp, routes) use the compact "terse" text forms.
type juniperParser struct{}

func (p *juniperParser) Vendor() string { return "juniper" }

func (p *juniperParser) Command(t StateType) (string, bool) {
	switch t {
	case Facts:
		return "show version | display json", true
	case Interfaces:
		return "show interfaces terse | display json", true
	case LLDP:
		return "show lldp neighbors | display json", true
	case MAC:
		return "show ethernet-switching table", true
	case ARP:
		return "show arp no-resolve", true
	case Routes:
		return "show route terse", true
	default:
		return "", false
	}
}

func (p *juniperParser) Parse(t StateType, raw string, dst *State) error {
	switch t {
	case Facts:
		f, err := parseJunosVersion(raw)
		if err != nil {
			return err
		}
		dst.Facts = f
	case Interfaces:
		ifaces, err := parseJunosInterfacesTerse(raw)
		if err != nil {
			return err
		}
		dst.Interfaces = ifaces
	case LLDP:
		n, err := parseJunosLLDP(raw)
		if err != nil {
			return err
		}
		dst.LLDP = n
	case MAC:
		dst.MAC = parseJunosMACTable(raw)
	case ARP:
		dst.ARP = parseJunosARP(raw)
	case Routes:
		dst.Routes = parseJunosRoutesTerse(raw)
	}
	return nil
}

// --- Junos JSON decoding ----------------------------------------------------
//
// Junos "| display json" wraps every scalar as a one-element array of objects
// with a "data" field ([{"data": "value"}]) and every container as an array.
// We decode into typed structs (far fewer allocations than a generic
// map[string]any) using jLeaf for the scalar wrapper. Missing keys decode to
// zero values, so minor schema differences degrade gracefully.

// jLeaf models a Junos [{"data": X}] scalar. str() returns the first value as a
// string; data is any so numeric/bool leaves decode without failing.
type jLeaf []struct {
	Data any `json:"data"`
}

func (l jLeaf) str() string {
	if len(l) == 0 {
		return ""
	}
	return stringify(l[0].Data)
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

type junosVersionDoc struct {
	SoftwareInformation []struct {
		HostName     jLeaf `json:"host-name"`
		ProductModel jLeaf `json:"product-model"`
		JunosVersion jLeaf `json:"junos-version"`
	} `json:"software-information"`
}

func parseJunosVersion(raw string) (*DeviceFacts, error) {
	var doc junosVersionDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("parse junos version json: %w", err)
	}
	f := &DeviceFacts{Vendor: "juniper"}
	if len(doc.SoftwareInformation) > 0 {
		si := doc.SoftwareInformation[0]
		f.Hostname = si.HostName.str()
		f.Model = si.ProductModel.str()
		f.OSVersion = si.JunosVersion.str()
	}
	return f, nil
}

type junosIfDoc struct {
	InterfaceInformation []struct {
		PhysicalInterface []struct {
			Name             jLeaf `json:"name"`
			AdminStatus      jLeaf `json:"admin-status"`
			OperStatus       jLeaf `json:"oper-status"`
			LogicalInterface []struct {
				Name          jLeaf `json:"name"`
				AdminStatus   jLeaf `json:"admin-status"`
				OperStatus    jLeaf `json:"oper-status"`
				AddressFamily []struct {
					AddressFamilyName jLeaf `json:"address-family-name"`
					InterfaceAddress  []struct {
						IfaLocal jLeaf `json:"ifa-local"`
					} `json:"interface-address"`
				} `json:"address-family"`
			} `json:"logical-interface"`
		} `json:"physical-interface"`
	} `json:"interface-information"`
}

func parseJunosInterfacesTerse(raw string) ([]Interface, error) {
	var doc junosIfDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("parse junos interfaces json: %w", err)
	}
	var out []Interface
	for _, info := range doc.InterfaceInformation {
		for _, pi := range info.PhysicalInterface {
			out = append(out, Interface{
				Name:        strings.TrimSpace(pi.Name.str()),
				AdminStatus: normStatus(pi.AdminStatus.str()),
				OperStatus:  normStatus(pi.OperStatus.str()),
			})
			for _, li := range pi.LogicalInterface {
				iface := Interface{
					Name:        strings.TrimSpace(li.Name.str()),
					AdminStatus: normStatus(li.AdminStatus.str()),
					OperStatus:  normStatus(li.OperStatus.str()),
				}
				for _, af := range li.AddressFamily {
					family := af.AddressFamilyName.str()
					for _, ia := range af.InterfaceAddress {
						addr := strings.TrimSpace(ia.IfaLocal.str())
						if addr == "" {
							continue
						}
						switch family {
						case "inet6":
							iface.IPv6 = append(iface.IPv6, addr)
						case "inet":
							iface.IPv4 = append(iface.IPv4, addr)
						}
					}
				}
				out = append(out, iface)
			}
		}
	}
	return out, nil
}

type junosLLDPDoc struct {
	LLDPNeighborsInformation []struct {
		LLDPNeighborInformation []struct {
			LocalPortID     jLeaf `json:"lldp-local-port-id"`
			LocalInterface  jLeaf `json:"lldp-local-interface"`
			RemoteChassisID jLeaf `json:"lldp-remote-chassis-id"`
			RemotePortID    jLeaf `json:"lldp-remote-port-id"`
			RemotePortDesc  jLeaf `json:"lldp-remote-port-description"`
			RemoteSystem    jLeaf `json:"lldp-remote-system-name"`
		} `json:"lldp-neighbor-information"`
	} `json:"lldp-neighbors-information"`
}

func parseJunosLLDP(raw string) ([]LLDPNeighbor, error) {
	var doc junosLLDPDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("parse junos lldp json: %w", err)
	}
	var out []LLDPNeighbor
	for _, info := range doc.LLDPNeighborsInformation {
		for _, ni := range info.LLDPNeighborInformation {
			local := ni.LocalPortID.str()
			if local == "" {
				local = ni.LocalInterface.str()
			}
			remotePort := ni.RemotePortDesc.str()
			if remotePort == "" {
				remotePort = ni.RemotePortID.str()
			}
			out = append(out, LLDPNeighbor{
				LocalPort:     local,
				RemoteSystem:  ni.RemoteSystem.str(),
				RemotePort:    remotePort,
				RemoteChassis: ni.RemoteChassisID.str(),
			})
		}
	}
	return out, nil
}

var junosMACRe = regexp.MustCompile(`(?i)^\s*(\S+)\s+([0-9a-f:]{17})\s+(\S+)\s+.*?(\S+)\s*$`)

// parseJunosMACTable parses "show ethernet-switching table" (best effort; Junos
// VLAN identifiers are names, so numeric VLANs are extracted when present).
func parseJunosMACTable(raw string) []MACEntry {
	var out []MACEntry
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := sc.Text()
		m := junosMACRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		mac := strings.ToLower(m[2])
		out = append(out, MACEntry{
			VLAN:      trailingInt(m[1]),
			MAC:       mac,
			Type:      strings.ToLower(m[3]),
			Interface: m[4],
		})
	}
	return out
}

// "show arp no-resolve" columns are: MAC Address, Address, Interface, Flags —
// so the interface is the token immediately after the IP (no name column).
var junosARPRe = regexp.MustCompile(`(?i)^([0-9a-f:]{17})\s+(\d+\.\d+\.\d+\.\d+)\s+(\S+)`)

func parseJunosARP(raw string) []ARPEntry {
	var out []ARPEntry
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		m := junosARPRe.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		out = append(out, ARPEntry{
			MAC:       strings.ToLower(m[1]),
			IP:        m[2],
			Interface: m[3],
		})
	}
	return out
}

var (
	junosRouteDestRe = regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+/\d+|[0-9a-fA-F:]+/\d+)`)
	junosRouteProto  = map[string]string{
		"D": "direct", "L": "local", "O": "ospf", "B": "bgp",
		"S": "static", "K": "kernel", "R": "rip", "I": "isis",
	}
)

// parseJunosRoutesTerse parses "show route terse". Lines look like:
//
//   - ? 10.0.0.0/24        D   0                          >ge-0/0/0.0
//   - ? 10.1.0.0/24        O  10        2                 >10.0.0.2
func parseJunosRoutesTerse(raw string) []Route {
	var out []Route
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := sc.Text()
		dest := junosRouteDestRe.FindString(line)
		if dest == "" {
			continue
		}
		fields := strings.Fields(line)
		r := Route{Prefix: dest}
		// Protocol letter is the field right after the destination.
		for i, f := range fields {
			if f == dest && i+1 < len(fields) {
				if p, ok := junosRouteProto[fields[i+1]]; ok {
					r.Protocol = p
				}
				break
			}
		}
		// Next hop is the last field, prefixed with ">".
		if last := fields[len(fields)-1]; strings.HasPrefix(last, ">") {
			nh := strings.TrimPrefix(last, ">")
			if strings.Contains(nh, ".") && !strings.Contains(nh, "/") && isIPish(nh) {
				r.NextHop = nh
			} else {
				r.Interface = nh
			}
		}
		out = append(out, r)
	}
	return out
}

// trailingInt returns the trailing integer in s (e.g. "v100" -> 100), or 0.
func trailingInt(s string) int {
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	if i == len(s) {
		return 0
	}
	n, _ := strconv.Atoi(s[i:])
	return n
}

func isIPish(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9') && r != '.' {
			return false
		}
	}
	return strings.Count(s, ".") == 3
}
