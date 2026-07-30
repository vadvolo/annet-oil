package opstate

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

// mikrotikParser parses MikroTik RouterOS output. RouterOS has no "show"
// commands and no native JSON; instead every table is read with "<path> print
// terse", which emits one record per line as space-separated key=value pairs
// preceded by single-letter flags (e.g. "0 R name=ether1 mtu=1500 ..."). The
// facts command "/system resource print" is the exception — it prints a plain
// "label: value" block. All parsers are best-effort and were written from
// documented RouterOS output, not verified against a live device.
//
// The command strings are identical across RouterOS v6 and v7, and the terse
// key=value format keeps most sections version-independent. The exception is
// routes: v7's routing rewrite encodes the route origin in lowercase flags
// (c/s) where v6 used uppercase (C/S). The parser is therefore version-aware —
// the collector supplies the major version (via versionAware) before parsing.
type mikrotikParser struct {
	// major is the RouterOS major version (6 or 7), or 0 when unknown.
	major int
}

func (p *mikrotikParser) Vendor() string { return "mikrotik" }

// MajorVersionFromFacts reads the major version from "/system resource print"
// facts (e.g. "7.15.3 (stable)" -> 7).
func (p *mikrotikParser) MajorVersionFromFacts(f *DeviceFacts) int {
	if f == nil {
		return 0
	}
	return leadingInt(f.OSVersion)
}

func (p *mikrotikParser) SetMajorVersion(major int) { p.major = major }

// Command returns the RouterOS command for a state type.
//
// The commands are deliberately kept short and do NOT pass "without-paging":
// RouterOS echoes a typed command character-by-character over the interactive
// shell, redrawing the whole prompt line each time, and gnetcli must match that
// noisy echo against the command it sent. Longer commands mean more redraws and
// a higher chance a network read splits mid-redraw, which gnetcli surfaces as an
// intermittent "generic_error" (an EchoReadException). Paging is unnecessary
// anyway — gnetcli's ros driver registers a pager matcher and auto-answers it,
// so full output is still collected.

func (p *mikrotikParser) Command(t StateType) (string, bool) {
	switch t {
	case Facts:
		return "/system resource print", true
	case Interfaces:
		return "/interface print terse", true
	case LLDP:
		return "/ip neighbor print terse", true
	case MAC:
		return "/interface bridge host print terse", true
	case ARP:
		return "/ip arp print terse", true
	case Routes:
		return "/ip route print terse", true
	default:
		return "", false
	}
}

func (p *mikrotikParser) Parse(t StateType, raw string, dst *State) error {
	switch t {
	case Facts:
		dst.Facts = parseROSResource(raw)
	case Interfaces:
		dst.Interfaces = parseROSInterfaces(raw)
	case LLDP:
		dst.LLDP = parseROSNeighbors(raw)
	case MAC:
		dst.MAC = parseROSBridgeHosts(raw)
	case ARP:
		dst.ARP = parseROSARP(raw)
	case Routes:
		dst.Routes = p.parseROSRoutes(raw)
	}
	return nil
}

// --- RouterOS "print terse" decoding ----------------------------------------

// rosRecord is one "print terse" line: the leading flag letters (e.g. "AS"
// meaning active+static) plus its key=value fields.
type rosRecord struct {
	flags  string
	fields map[string]string
}

// rosKVRe matches one key=value pair; quoted values (comments) keep their spaces.
var rosKVRe = regexp.MustCompile(`([A-Za-z0-9._-]+)=("[^"]*"|\S+)`)

// parseROSTerse splits "print terse" output into records. The header line
// ("Flags: ...") and column banners are skipped; the leading record index
// number is dropped and any remaining single-letter tokens become flags.
func parseROSTerse(raw string) []rosRecord {
	var out []rosRecord
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // route/arp tables get long
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "Flags:") || strings.HasPrefix(line, "Columns:") {
			continue
		}
		kv := rosKVRe.FindAllStringSubmatch(line, -1)
		if len(kv) == 0 {
			continue
		}
		rec := rosRecord{fields: make(map[string]string, len(kv))}
		// Everything before the first key=value is the index + flag letters.
		if p := strings.Index(line, kv[0][1]+"="); p >= 0 {
			for f := range strings.FieldsSeq(line[:p]) {
				if _, err := strconv.Atoi(f); err == nil {
					continue // record index
				}
				rec.flags += f
			}
		}
		for _, m := range kv {
			rec.fields[m[1]] = strings.Trim(m[2], `"`)
		}
		out = append(out, rec)
	}
	return out
}

// parseROSResource parses "/system resource print" ("label: value" per line).
func parseROSResource(raw string) *DeviceFacts {
	f := &DeviceFacts{Vendor: "mikrotik"}
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "version":
			f.OSVersion = v
		case "board-name":
			f.Model = v
		case "uptime":
			f.Uptime = v
		}
	}
	return f
}

func parseROSInterfaces(raw string) []Interface {
	var out []Interface
	for _, rec := range parseROSTerse(raw) {
		name := rec.fields["name"]
		if name == "" {
			continue
		}
		iface := Interface{
			Name:        name,
			Description: rec.fields["comment"],
			MAC:         strings.ToLower(rec.fields["mac-address"]),
		}
		// Flags: X disabled -> admin down; R running -> oper up.
		if strings.Contains(rec.flags, "X") {
			iface.AdminStatus = "down"
		} else {
			iface.AdminStatus = "up"
		}
		if strings.Contains(rec.flags, "R") {
			iface.OperStatus = "up"
		} else {
			iface.OperStatus = "down"
		}
		mtu := rec.fields["actual-mtu"]
		if mtu == "" {
			mtu = rec.fields["mtu"]
		}
		iface.MTU, _ = strconv.Atoi(mtu)
		out = append(out, iface)
	}
	return out
}

// parseROSNeighbors parses "/ip neighbor print terse" (MNDP/CDP/LLDP discovery).
func parseROSNeighbors(raw string) []LLDPNeighbor {
	var out []LLDPNeighbor
	for _, rec := range parseROSTerse(raw) {
		local := rec.fields["interface"]
		if local == "" {
			continue
		}
		out = append(out, LLDPNeighbor{
			LocalPort:     local,
			RemoteSystem:  rec.fields["identity"],
			RemotePort:    rec.fields["interface-name"],
			RemoteChassis: strings.ToLower(rec.fields["mac-address"]),
			RemoteMgmtIP:  rec.fields["address"],
		})
	}
	return out
}

// parseROSBridgeHosts parses "/interface bridge host print terse".
func parseROSBridgeHosts(raw string) []MACEntry {
	var out []MACEntry
	for _, rec := range parseROSTerse(raw) {
		mac := rec.fields["mac-address"]
		if mac == "" {
			continue
		}
		e := MACEntry{
			MAC:       strings.ToLower(mac),
			Interface: rec.fields["on-interface"],
			VLAN:      atoiOrZero(rec.fields["vid"]),
		}
		switch rec.fields["dynamic"] {
		case "yes":
			e.Type = "dynamic"
		case "no":
			e.Type = "static"
		}
		out = append(out, e)
	}
	return out
}

// parseROSARP parses "/ip arp print terse".
func parseROSARP(raw string) []ARPEntry {
	var out []ARPEntry
	for _, rec := range parseROSTerse(raw) {
		ip := rec.fields["address"]
		if ip == "" {
			continue
		}
		out = append(out, ARPEntry{
			IP:        ip,
			MAC:       strings.ToLower(rec.fields["mac-address"]),
			Interface: rec.fields["interface"],
		})
	}
	return out
}

// parseROSRoutes parses "/ip route print terse". The protocol comes from the
// route flags (version-dependent, see rosRouteProto); the gateway field is
// either a next-hop IP or an egress interface name. RouterOS v7 also emits
// "immediate-gw", used as a fallback when "gateway" is absent.
func (p *mikrotikParser) parseROSRoutes(raw string) []Route {
	var out []Route
	for _, rec := range parseROSTerse(raw) {
		dst := rec.fields["dst-address"]
		if dst == "" {
			continue
		}
		r := Route{Prefix: dst, Protocol: rosRouteProto(rec.flags, p.major)}
		gw := rec.fields["gateway"]
		if gw == "" {
			gw = rec.fields["immediate-gw"]
		}
		if gw = strings.SplitN(gw, "%", 2)[0]; gw != "" {
			if isIPish(gw) {
				r.NextHop = gw
			} else {
				r.Interface = gw
			}
		}
		out = append(out, r)
	}
	return out
}

// rosRouteProto maps RouterOS route flags to a protocol name. v7's routing
// rewrite encodes the origin in lowercase flags (c connect, s static); v6 used
// uppercase C/S for those (ospf/bgp/rip are lowercase in both). When the major
// version is unknown (0), either case is accepted.
func rosRouteProto(flags string, major int) string {
	has := func(set string) bool { return strings.ContainsAny(flags, set) }
	switch {
	case major >= 7:
		switch {
		case has("c"):
			return "connected"
		case has("s"):
			return "static"
		}
	case major == 6:
		switch {
		case has("C"):
			return "connected"
		case has("S"):
			return "static"
		}
	default: // unknown version: accept either case
		switch {
		case has("Cc"):
			return "connected"
		case has("Ss"):
			return "static"
		}
	}
	switch {
	case has("o"):
		return "ospf"
	case has("b"):
		return "bgp"
	case has("r"):
		return "rip"
	case major >= 7 && has("d"):
		return "dhcp"
	}
	return ""
}

func atoiOrZero(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// leadingInt returns the leading integer in s (e.g. "7.15.3 (stable)" -> 7), or 0.
func leadingInt(s string) int {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	n, _ := strconv.Atoi(s[:i])
	return n
}
