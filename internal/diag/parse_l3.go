package diag

import (
	"regexp"
	"strings"

	"annet-oil/internal/diag/diagpb"
)

func init() {
	cisco := []diagpb.Family{diagpb.Family_FAMILY_IOSXR, diagpb.Family_FAMILY_IOS}
	elfs := []diagpb.Family{diagpb.Family_FAMILY_ELTEX, diagpb.Family_FAMILY_FS}

	regParseAll(cisco, "interfaces", interfacesCisco)
	regParseAll(elfs, "interfaces", interfacesEltex)
	regParse(diagpb.Family_FAMILY_ROUTEROS, "interfaces", interfacesRouterOS)

	regParseAll(cisco, "arp", arpCisco)
	regParseAll(elfs, "arp", arpCisco)
	regParse(diagpb.Family_FAMILY_ROUTEROS, "arp", arpRouterOS)

	regParseAll(cisco, "mac", macCisco)
	regParseAll(elfs, "mac", macCisco)

	regParseAll(cisco, "routes", routesCisco)
	regParseAll(elfs, "routes", routesCisco)
	regParse(diagpb.Family_FAMILY_ROUTEROS, "routes", routesRouterOS)
}

// ---- interfaces -----------------------------------------------------------

// interfacesCisco handles both "show ip interface brief" (two column layouts)
// and the full "show interfaces" block output.
func interfacesCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	if strings.Contains(raw, "line protocol is") {
		return interfacesCiscoFull(raw)
	}
	return interfacesCiscoBrief(raw)
}

func interfacesCiscoBrief(raw string) *diagpb.Parsed {
	var items []*diagpb.Interface
	iosLayout := false
	sc := scanLines(raw)
	for sc.Scan() {
		line := sc.Text()
		tl := strings.TrimSpace(line)
		if tl == "" {
			continue
		}
		if strings.HasPrefix(tl, "Interface") {
			iosLayout = strings.Contains(tl, "OK?")
			continue
		}
		f := fields(tl)
		if len(f) < 2 {
			continue
		}
		it := &diagpb.Interface{Name: f[0]}
		if f[1] != "unassigned" && f[1] != "unnumbered" {
			it.Ipv4 = []string{f[1]}
		}
		if iosLayout {
			// Interface IP OK? Method Status Protocol
			if len(f) >= 6 {
				it.AdminStatus = normUpDown(f[4])
				it.OperStatus = normUpDown(f[5])
			}
		} else {
			// Interface IP Status Protocol [Vrf]
			if len(f) >= 4 {
				it.AdminStatus = normUpDown(f[2])
				it.OperStatus = normUpDown(f[3])
			}
			if len(f) >= 5 {
				it.Vrf = f[4]
			}
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		return nil
	}
	return pInterfaces(items)
}

var (
	reIfHead   = regexp.MustCompile(`^(\S+) is (administratively down|up|down)(?:, line protocol is (\S+))?`)
	reIfDesc   = regexp.MustCompile(`Description:\s*(.+)`)
	reIfMAC    = regexp.MustCompile(`address is ([0-9a-fA-F.:]+)`)
	reIfMTU    = regexp.MustCompile(`MTU\s+(\d+)`)
	reIfInErr  = regexp.MustCompile(`(\d+) input errors`)
	reIfOutErr = regexp.MustCompile(`(\d+) output errors`)
)

func interfacesCiscoFull(raw string) *diagpb.Parsed {
	var items []*diagpb.Interface
	var cur *diagpb.Interface
	flush := func() {
		if cur != nil {
			items = append(items, cur)
		}
	}
	sc := scanLines(raw)
	for sc.Scan() {
		line := sc.Text()
		if m := reIfHead.FindStringSubmatch(line); m != nil {
			flush()
			cur = &diagpb.Interface{Name: m[1]}
			if strings.HasPrefix(m[2], "admin") {
				cur.AdminStatus = "down"
			} else {
				cur.AdminStatus = normUpDown(m[2])
			}
			if m[3] != "" {
				cur.OperStatus = normUpDown(m[3])
			}
			continue
		}
		if cur == nil {
			continue
		}
		if m := reIfDesc.FindStringSubmatch(line); m != nil {
			cur.Description = strings.TrimSpace(m[1])
		}
		if m := reIfMAC.FindStringSubmatch(line); m != nil && cur.Mac == "" {
			cur.Mac = m[1]
		}
		if m := reIfMTU.FindStringSubmatch(line); m != nil && cur.Mtu == 0 {
			cur.Mtu = int32(atoi(m[1]))
		}
		if m := reIfInErr.FindStringSubmatch(line); m != nil {
			cur.InErrors = atoi64(m[1])
		}
		if m := reIfOutErr.FindStringSubmatch(line); m != nil {
			cur.OutErrors = atoi64(m[1])
		}
	}
	flush()
	if len(items) == 0 {
		return nil
	}
	return pInterfaces(items)
}

// interfacesEltex parses "show interfaces status" (Eltex/FS).
// Columns: Port Type Duplex Speed Neg ctrl State Uptime ... Port-Mode(VLAN)
func interfacesEltex(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	var items []*diagpb.Interface
	reVlan := regexp.MustCompile(`\((\d+)\)`)
	sc := scanLines(raw)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" || strings.HasPrefix(tl, "Port") || strings.HasPrefix(tl, "---") || strings.HasPrefix(tl, "Flow") {
			continue
		}
		f := fields(tl)
		if len(f) < 2 {
			continue
		}
		it := &diagpb.Interface{Name: f[0]}
		// State column contains "Up" / "Down (nc)" / "Down (adm)".
		low := strings.ToLower(tl)
		switch {
		case strings.Contains(low, "down (adm"):
			it.AdminStatus, it.OperStatus = "down", "down"
		case strings.Contains(low, "up"):
			it.AdminStatus, it.OperStatus = "up", "up"
		case strings.Contains(low, "down"):
			it.AdminStatus, it.OperStatus = "up", "down"
		}
		if len(f) >= 4 {
			it.Duplex = strings.ToLower(f[2])
			it.SpeedMbps = atoi64(f[3])
		}
		if m := reVlan.FindStringSubmatch(tl); m != nil {
			it.Vlan = int32(atoi(m[1]))
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		return nil
	}
	return pInterfaces(items)
}

var (
	reRosName   = regexp.MustCompile(`name="([^"]+)"`)
	reRosType   = regexp.MustCompile(`\btype="?([a-z]+)"?`)
	reRosMac    = regexp.MustCompile(`mac-address=([0-9A-Fa-f:]+)`)
	reRosMtu    = regexp.MustCompile(`\bmtu=(\d+)`)
	reRosLinkDn = regexp.MustCompile(`link-downs=(\d+)`)
)

// interfacesRouterOS parses "/interface print detail" (numbered key=value blocks).
func interfacesRouterOS(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	var items []*diagpb.Interface
	// Each record starts with a leading index number and flag letters.
	reRec := regexp.MustCompile(`(?m)^\s*\d+\s+([A-Za-z ]*?)\s*name=`)
	// Split into records at each " N  FLAGS name="...
	idx := reRec.FindAllStringIndex(raw, -1)
	if len(idx) == 0 {
		return nil
	}
	for i, loc := range idx {
		end := len(raw)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		block := raw[loc[0]:end]
		flags := strings.TrimSpace(reRec.FindStringSubmatch(block)[1])
		it := &diagpb.Interface{}
		if m := reRosName.FindStringSubmatch(block); m != nil {
			it.Name = m[1]
		}
		if m := reRosType.FindStringSubmatch(block); m != nil {
			it.Description = m[1] // interface type as a hint
		}
		if m := reRosMac.FindStringSubmatch(block); m != nil {
			it.Mac = m[1]
		}
		if m := reRosMtu.FindStringSubmatch(block); m != nil {
			it.Mtu = int32(atoi(m[1]))
		}
		if m := reRosLinkDn.FindStringSubmatch(block); m != nil {
			it.LinkDowns = atoi64(m[1])
		}
		// Flags: X=disabled, R=running, D=dynamic, S=slave.
		if strings.Contains(flags, "X") {
			it.AdminStatus = "down"
		} else {
			it.AdminStatus = "up"
		}
		if strings.Contains(flags, "R") {
			it.OperStatus = "up"
		} else {
			it.OperStatus = "down"
		}
		if it.Name != "" {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		return nil
	}
	return pInterfaces(items)
}

// ---- arp ------------------------------------------------------------------

var reMAC = regexp.MustCompile(`([0-9a-fA-F]{2}[:.-]){2,}[0-9a-fA-F.:-]+`)
var reIPv4 = regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)

// arpCisco parses "show arp" / "show ip arp" (Cisco/Eltex/FS variants).
func arpCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	var items []*diagpb.ArpEntry
	sc := scanLines(raw)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" || strings.HasPrefix(tl, "Protocol") || strings.HasPrefix(tl, "Address") ||
			strings.HasPrefix(tl, "Total") || strings.HasPrefix(tl, "VLAN") || strings.HasPrefix(tl, "---") {
			continue
		}
		ip := reIPv4.FindString(tl)
		mac := reMAC.FindString(tl)
		if ip == "" || mac == "" {
			continue
		}
		e := &diagpb.ArpEntry{Ip: ip, Mac: mac}
		f := fields(tl)
		if len(f) > 0 {
			e.Interface = f[len(f)-1] // last column is usually the interface
		}
		items = append(items, e)
	}
	if len(items) == 0 {
		return nil
	}
	return pArp(items)
}

// arpRouterOS parses "/ip arp print" columns: # FLAGS ADDRESS MAC-ADDRESS INTERFACE
func arpRouterOS(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	var items []*diagpb.ArpEntry
	sc := scanLines(raw)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		ip := reIPv4.FindString(tl)
		if ip == "" || strings.HasPrefix(tl, "Flags") || strings.Contains(tl, "ADDRESS") {
			continue
		}
		e := &diagpb.ArpEntry{Ip: ip, Mac: reMAC.FindString(tl)}
		f := fields(tl)
		if len(f) > 0 {
			e.Interface = f[len(f)-1]
		}
		items = append(items, e)
	}
	if len(items) == 0 {
		return nil
	}
	return pArp(items)
}

// ---- mac ------------------------------------------------------------------

// macCisco parses "show mac address-table" (Cisco/Eltex/FS): Vlan Mac Type Ports.
func macCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	var items []*diagpb.MacEntry
	sc := scanLines(raw)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		mac := reMAC.FindString(tl)
		if mac == "" || strings.Contains(strings.ToLower(tl), "mac address") {
			continue
		}
		e := &diagpb.MacEntry{Mac: mac}
		f := fields(tl)
		if len(f) >= 1 {
			e.Vlan = int32(atoi(f[0]))
		}
		if len(f) >= 1 {
			e.Interface = f[len(f)-1]
		}
		low := strings.ToLower(tl)
		switch {
		case strings.Contains(low, "dynamic"):
			e.Type = "dynamic"
		case strings.Contains(low, "static"):
			e.Type = "static"
		}
		items = append(items, e)
	}
	if len(items) == 0 {
		return nil
	}
	return pMac(items)
}

// ---- routes ---------------------------------------------------------------

var reRouteSummary = regexp.MustCompile(`(?i)Total.*?(\d+)`)

// routesCisco parses "show route <t>" / "show ip route <t>" and route summaries.
func routesCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	info := &diagpb.RouteInfo{}
	sc := scanLines(raw)
	reVia := regexp.MustCompile(`via\s+(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
	rePrefix := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/\d+)`)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" {
			continue
		}
		if m := reRouteSummary.FindStringSubmatch(tl); m != nil && strings.Contains(strings.ToLower(tl), "route") {
			info.TotalCount = int32(atoi(m[1]))
		}
		if px := rePrefix.FindString(tl); px != "" {
			r := &diagpb.Route{Prefix: px}
			if v := reVia.FindStringSubmatch(tl); v != nil {
				r.NextHop = v[1]
			}
			f := fields(tl)
			if len(f) > 0 {
				r.Protocol = f[0]
			}
			info.Items = append(info.Items, r)
		}
	}
	if len(info.Items) == 0 && info.TotalCount == 0 {
		return nil
	}
	return pRoutes(info)
}

// routesRouterOS parses "/ip route print" (or count-only).
func routesRouterOS(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	info := &diagpb.RouteInfo{}
	// count-only returns a bare number.
	if n := strings.TrimSpace(raw); regexp.MustCompile(`^\d+$`).MatchString(n) {
		info.TotalCount = int32(atoi(n))
		return pRoutes(info)
	}
	rePrefix := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/\d+)`)
	sc := scanLines(raw)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if px := rePrefix.FindString(tl); px != "" {
			r := &diagpb.Route{Prefix: px}
			ips := reIPv4.FindAllString(tl, -1)
			if len(ips) >= 2 {
				r.NextHop = ips[len(ips)-1]
			}
			info.Items = append(info.Items, r)
		}
	}
	if len(info.Items) == 0 {
		return nil
	}
	return pRoutes(info)
}
