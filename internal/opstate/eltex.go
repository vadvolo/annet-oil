package opstate

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

// eltexParser parses Eltex MES-family switch output. The CLI is Cisco-like but
// not identical: MAC/ARP/interface tables use different columns, so those get
// dedicated parsers, while "show ip route" is close enough to Cisco to reuse
// parseCiscoRoutes. Like the Mikrotik parser, these were written from
// documented Eltex output and are best-effort, not verified against a device.
type eltexParser struct{}

func (p *eltexParser) Vendor() string { return "eltex" }

func (p *eltexParser) Command(t StateType) (string, bool) {
	switch t {
	case Facts:
		return "show version", true
	case Interfaces:
		return "show interfaces status", true
	case LLDP:
		return "show lldp neighbors", true
	case MAC:
		return "show mac address-table", true
	case ARP:
		return "show ip arp", true
	case Routes:
		return "show ip route", true
	default:
		return "", false
	}
}

func (p *eltexParser) Parse(t StateType, raw string, dst *State) error {
	switch t {
	case Facts:
		dst.Facts = parseEltexVersion(raw)
	case Interfaces:
		dst.Interfaces = parseEltexIfStatus(raw)
	case LLDP:
		dst.LLDP = parseEltexLLDP(raw)
	case MAC:
		dst.MAC = parseEltexMACTable(raw)
	case ARP:
		dst.ARP = parseEltexARP(raw)
	case Routes:
		dst.Routes = parseCiscoRoutes(raw) // Eltex "show ip route" is Cisco-shaped.
	}
	return nil
}

var (
	// eltexPortRe matches an Eltex port/interface token (gi1/0/1, te1/0/1,
	// fa0/1, xg1, Po1/Ch1, vlan10, oob). The digit after the prefix keeps the
	// literal header word "Port" from matching the "po" (Port-channel) alias.
	eltexPortRe = regexp.MustCompile(`(?i)^(?:(?:gi|te|fa|xg|po|ch|vlan)\d\S*|oob)$`)

	eltexVersionRe = regexp.MustCompile(`(?i)(?:sw\s+version|version)[:\s]+v?(\d[\w.\-()]*)`)
	eltexModelRe   = regexp.MustCompile(`(?i)eltex[^\w]+(\S+)`)

	eltexIPv4Re = regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)
	eltexMACRe  = regexp.MustCompile(`([0-9a-fA-F]{2}(?::[0-9a-fA-F]{2}){5})`)

	// eltexMACRowRe matches a MAC-address-table row: vlan, mac, port, type.
	eltexMACRowRe = regexp.MustCompile(`^\s*(\d+)\s+([0-9a-fA-F:]{17})\s+(\S+)\s+(\S+)`)
)

func parseEltexVersion(raw string) *DeviceFacts {
	f := &DeviceFacts{Vendor: "eltex"}
	if m := eltexVersionRe.FindStringSubmatch(raw); m != nil {
		f.OSVersion = m[1]
	}
	if m := eltexModelRe.FindStringSubmatch(raw); m != nil {
		f.Model = m[1]
	}
	return f
}

// parseEltexIfStatus parses "show interfaces status". The status column is
// "Up"/"Down"; speed (Mbps) is the numeric column when the link is up. There is
// no admin-state column in this table, so AdminStatus is left empty.
func parseEltexIfStatus(raw string) []Interface {
	var out []Interface
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || !eltexPortRe.MatchString(fields[0]) {
			continue
		}
		iface := Interface{Name: fields[0]}
		for _, fld := range fields[1:] {
			switch strings.ToLower(fld) {
			case "up":
				iface.OperStatus = "up"
			case "down":
				iface.OperStatus = "down"
			}
			if iface.SpeedMbps == 0 {
				if n, err := strconv.Atoi(fld); err == nil && n >= 10 {
					iface.SpeedMbps = int64(n)
				}
			}
		}
		out = append(out, iface)
	}
	return out
}

// parseEltexLLDP parses "show lldp neighbors" columns:
// Port | Device ID (chassis) | Port ID | System Name | Capabilities | TTL.
func parseEltexLLDP(raw string) []LLDPNeighbor {
	var out []LLDPNeighbor
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		fields := strings.Fields(line)
		if len(fields) < 4 || !eltexPortRe.MatchString(fields[0]) {
			continue
		}
		out = append(out, LLDPNeighbor{
			LocalPort:     fields[0],
			RemoteChassis: strings.ToLower(fields[1]),
			RemotePort:    fields[2],
			RemoteSystem:  fields[3],
		})
	}
	return out
}

// parseEltexMACTable parses "show mac address-table": vlan, mac, port, type.
func parseEltexMACTable(raw string) []MACEntry {
	var out []MACEntry
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		m := eltexMACRowRe.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		out = append(out, MACEntry{
			VLAN:      atoiOrZero(m[1]),
			MAC:       strings.ToLower(m[2]),
			Interface: m[3],
			Type:      strings.ToLower(m[4]),
		})
	}
	return out
}

// parseEltexARP parses "show ip arp". Column order varies across firmware, so
// the IP and MAC are matched by shape and the interface is the first port-like
// token on the row.
func parseEltexARP(raw string) []ARPEntry {
	var out []ARPEntry
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := sc.Text()
		ip := eltexIPv4Re.FindString(line)
		mac := eltexMACRe.FindString(line)
		if ip == "" || mac == "" {
			continue
		}
		e := ARPEntry{IP: ip, MAC: strings.ToLower(mac)}
		// Prefer the physical egress port over the VLAN column, but fall back to
		// the VLAN interface if that is the only port-like token on the row.
		var vlanFallback string
		for fld := range strings.FieldsSeq(line) {
			if !eltexPortRe.MatchString(fld) {
				continue
			}
			if strings.HasPrefix(strings.ToLower(fld), "vlan") {
				if vlanFallback == "" {
					vlanFallback = fld
				}
				continue
			}
			e.Interface = fld
			break
		}
		if e.Interface == "" {
			e.Interface = vlanFallback
		}
		out = append(out, e)
	}
	return out
}
