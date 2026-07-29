package opstate

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

// ciscoParser parses Cisco IOS / IOS-XE / IOS-XR text output. Cisco output is
// tabular free text, so parsing is regex/field based (there is no reliable
// native JSON on classic IOS).
type ciscoParser struct{}

func (p *ciscoParser) Vendor() string { return "cisco" }

func (p *ciscoParser) Command(t StateType) (string, bool) {
	switch t {
	case Facts:
		return "show version", true
	case Interfaces:
		return "show ip interface brief", true
	case LLDP:
		return "show lldp neighbors detail", true
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

func (p *ciscoParser) Parse(t StateType, raw string, dst *State) error {
	switch t {
	case Facts:
		dst.Facts = parseCiscoVersion(raw)
	case Interfaces:
		dst.Interfaces = parseCiscoIPIntBrief(raw)
	case LLDP:
		dst.LLDP = parseCiscoLLDPDetail(raw)
	case MAC:
		dst.MAC = parseCiscoMACTable(raw)
	case ARP:
		dst.ARP = parseCiscoARP(raw)
	case Routes:
		dst.Routes = parseCiscoRoutes(raw)
	}
	return nil
}

var (
	ciscoVersionRe  = regexp.MustCompile(`(?i)(?:cisco )?ios[ -].*version\s+([^\s,]+)`)
	ciscoModelRe    = regexp.MustCompile(`(?i)cisco\s+(\S+)\s+\(.*\)\s+processor`)
	ciscoSerialRe   = regexp.MustCompile(`(?i)processor board id\s+(\S+)`)
	ciscoHostnameRe = regexp.MustCompile(`^(\S+)\s+uptime is\s+(.+)$`)
)

func parseCiscoVersion(raw string) *DeviceFacts {
	f := &DeviceFacts{Vendor: "cisco"}
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := sc.Text()
		if m := ciscoVersionRe.FindStringSubmatch(line); m != nil && f.OSVersion == "" {
			f.OSVersion = m[1]
		}
		if m := ciscoModelRe.FindStringSubmatch(line); m != nil {
			f.Model = m[1]
		}
		if m := ciscoSerialRe.FindStringSubmatch(line); m != nil {
			f.Serial = m[1]
		}
		if m := ciscoHostnameRe.FindStringSubmatch(line); m != nil {
			f.Hostname = m[1]
			f.Uptime = strings.TrimSpace(m[2])
		}
	}
	return f
}

// parseCiscoIPIntBrief parses "show ip interface brief". The Status column may
// be two words ("administratively down"), so fields are parsed positionally
// from both ends rather than with a fixed-width regex.
func parseCiscoIPIntBrief(raw string) []Interface {
	var out []Interface
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		if strings.EqualFold(fields[0], "Interface") { // header
			continue
		}
		name := fields[0]
		ip := fields[1]
		protocol := fields[len(fields)-1]
		status := strings.Join(fields[4:len(fields)-1], " ")

		iface := Interface{
			Name:       name,
			OperStatus: normStatus(protocol),
		}
		if strings.Contains(strings.ToLower(status), "administratively") {
			iface.AdminStatus = "down"
		} else {
			iface.AdminStatus = "up"
		}
		if ip != "" && !strings.EqualFold(ip, "unassigned") {
			iface.IPv4 = append(iface.IPv4, ip)
		}
		out = append(out, iface)
	}
	return out
}

// parseCiscoLLDPDetail parses "show lldp neighbors detail".
func parseCiscoLLDPDetail(raw string) []LLDPNeighbor {
	var out []LLDPNeighbor
	var cur *LLDPNeighbor
	flush := func() {
		if cur != nil && cur.LocalPort != "" {
			out = append(out, *cur)
		}
	}
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Local Intf:") {
			flush()
			cur = &LLDPNeighbor{LocalPort: valueAfterColon(line)}
			continue
		}
		if cur == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "Port id:"):
			cur.RemotePort = valueAfterColon(line)
		case strings.HasPrefix(line, "System Name:"):
			cur.RemoteSystem = valueAfterColon(line)
		case strings.HasPrefix(line, "Chassis id:"):
			cur.RemoteChassis = valueAfterColon(line)
		case strings.HasPrefix(line, "Management Addresses:"):
			// The address is on the following "IP: x.x.x.x" line.
		case strings.HasPrefix(line, "IP:") && cur.RemoteMgmtIP == "":
			cur.RemoteMgmtIP = valueAfterColon(line)
		}
	}
	flush()
	return out
}

var ciscoMACRe = regexp.MustCompile(`^\s*(?:\*\s+)?(\d+)\s+([0-9a-fA-F.:]{4,})\s+(\w+)\s+(\S+)`)

func parseCiscoMACTable(raw string) []MACEntry {
	var out []MACEntry
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		m := ciscoMACRe.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		vlan, _ := strconv.Atoi(m[1])
		out = append(out, MACEntry{
			VLAN:      vlan,
			MAC:       strings.ToLower(m[2]),
			Type:      strings.ToLower(m[3]),
			Interface: m[4],
		})
	}
	return out
}

var ciscoARPRe = regexp.MustCompile(`^Internet\s+(\S+)\s+\S+\s+([0-9a-fA-F.:]+)\s+\S+\s+(\S+)`)

func parseCiscoARP(raw string) []ARPEntry {
	var out []ARPEntry
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		m := ciscoARPRe.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		out = append(out, ARPEntry{
			IP:        m[1],
			MAC:       strings.ToLower(m[2]),
			Interface: m[3],
		})
	}
	return out
}

var (
	ciscoRouteViaRe    = regexp.MustCompile(`^([A-Za-z*]{1,3})\s*(?:\S+\s+)?(\d+\.\d+\.\d+\.\d+/\d+)\s+.*via\s+(\d+\.\d+\.\d+\.\d+)(?:,.*?,\s*(\S+))?`)
	ciscoRouteConnRe   = regexp.MustCompile(`^([A-Za-z*]{1,3})\s*(?:\S+\s+)?(\d+\.\d+\.\d+\.\d+/\d+)\s+is\s+directly\s+connected,\s*(\S+)`)
	ciscoRouteProtoMap = map[string]string{
		"O": "ospf", "B": "bgp", "C": "connected", "S": "static",
		"D": "eigrp", "R": "rip", "L": "local", "i": "isis",
	}
)

// parseCiscoRoutes parses "show ip route" (best effort: connected and single
// next-hop routes; the leading code letter maps to a protocol name).
func parseCiscoRoutes(raw string) []Route {
	var out []Route
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := sc.Text()
		if m := ciscoRouteConnRe.FindStringSubmatch(line); m != nil {
			out = append(out, Route{
				Prefix:    m[2],
				Interface: m[3],
				Protocol:  ciscoProto(m[1]),
			})
			continue
		}
		if m := ciscoRouteViaRe.FindStringSubmatch(line); m != nil {
			out = append(out, Route{
				Prefix:    m[2],
				NextHop:   m[3],
				Interface: m[4],
				Protocol:  ciscoProto(m[1]),
			})
		}
	}
	return out
}

func ciscoProto(code string) string {
	code = strings.TrimSpace(strings.TrimSuffix(code, "*"))
	if code == "" {
		return ""
	}
	if p, ok := ciscoRouteProtoMap[code[:1]]; ok {
		return p
	}
	return ""
}

// valueAfterColon returns the trimmed text after the first ":" in a line.
func valueAfterColon(line string) string {
	if _, after, ok := strings.Cut(line, ":"); ok {
		return strings.TrimSpace(after)
	}
	return ""
}
