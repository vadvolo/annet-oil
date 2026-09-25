package diag

import (
	"regexp"
	"strings"

	"annet-oil/internal/diag/diagpb"
)

func init() {
	ciscoLike := []diagpb.Family{diagpb.Family_FAMILY_IOSXR, diagpb.Family_FAMILY_IOS, diagpb.Family_FAMILY_ELTEX, diagpb.Family_FAMILY_FS}

	regParseAll(ciscoLike, "lldp", lldpCiscoDetail)
	regParse(diagpb.Family_FAMILY_IOS, "cdp", cdpCiscoDetail)
	regParseAll(ciscoLike, "bgp", bgpCisco)
	regParseAll(ciscoLike, "ospf", ospfCisco)
	regParseAll(ciscoLike, "isis", isisCisco)
	regParseAll(ciscoLike, "bfd", bfdCisco)

	regParse(diagpb.Family_FAMILY_ROUTEROS, "lldp", neighRouterOS("lldp"))
	regParse(diagpb.Family_FAMILY_ROUTEROS, "bgp", neighRouterOS("bgp"))
	regParse(diagpb.Family_FAMILY_ROUTEROS, "ospf", neighRouterOS("ospf"))
	regParse(diagpb.Family_FAMILY_ROUTEROS, "isis", neighRouterOS("isis"))
	regParse(diagpb.Family_FAMILY_ROUTEROS, "bfd", neighRouterOS("bfd"))
}

// ---- LLDP (Cisco detail) --------------------------------------------------

var (
	reLLDPLocal   = regexp.MustCompile(`(?i)Local Interface:\s*(\S+)`)
	reLLDPChassis = regexp.MustCompile(`(?i)Chassis id:\s*(\S+)`)
	reLLDPPort    = regexp.MustCompile(`(?i)Port id:\s*(\S+)`)
	reLLDPSys     = regexp.MustCompile(`(?i)System Name:\s*(.+)`)
	reLLDPMgmt    = regexp.MustCompile(`(?i)(?:Management Address(?:es)?|IP):\s*(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
)

func lldpCiscoDetail(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	nl := &diagpb.NeighborList{Protocol: "lldp"}
	// Split into per-neighbor blocks on "Local Interface:" boundaries.
	locs := reLLDPLocal.FindAllStringIndex(raw, -1)
	if len(locs) == 0 {
		// Fall back to table form (Eltex "show lldp neighbors").
		return lldpTable(raw)
	}
	for i, loc := range locs {
		end := len(raw)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := raw[loc[0]:end]
		n := &diagpb.Neighbor{Protocol: "lldp"}
		if m := reLLDPLocal.FindStringSubmatch(block); m != nil {
			n.LocalPort = m[1]
		}
		if m := reLLDPChassis.FindStringSubmatch(block); m != nil {
			n.RemoteChassis = m[1]
		}
		if m := reLLDPPort.FindStringSubmatch(block); m != nil {
			n.RemotePort = m[1]
		}
		if m := reLLDPSys.FindStringSubmatch(block); m != nil {
			n.RemoteSystem = strings.TrimSpace(m[1])
		}
		if m := reLLDPMgmt.FindStringSubmatch(block); m != nil {
			n.RemoteMgmtIp = m[1]
		}
		nl.Items = append(nl.Items, n)
	}
	if len(nl.Items) == 0 {
		return nil
	}
	return pNeighbors(nl)
}

// lldpTable parses the Eltex/FS "show lldp neighbors" column form.
func lldpTable(raw string) *diagpb.Parsed {
	nl := &diagpb.NeighborList{Protocol: "lldp"}
	sc := scanLines(raw)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" || strings.HasPrefix(tl, "Port") || strings.HasPrefix(tl, "---") ||
			strings.Contains(tl, "Chassis") || strings.HasPrefix(tl, "System") {
			continue
		}
		f := fields(tl)
		if len(f) < 2 {
			continue
		}
		nl.Items = append(nl.Items, &diagpb.Neighbor{
			Protocol:     "lldp",
			LocalPort:    f[0],
			RemotePort:   f[1],
			RemoteSystem: strings.Join(f[2:], " "),
		})
	}
	if len(nl.Items) == 0 {
		return nil
	}
	return pNeighbors(nl)
}

// ---- CDP (Cisco detail) ---------------------------------------------------

func cdpCiscoDetail(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	nl := &diagpb.NeighborList{Protocol: "cdp"}
	blocks := regexp.MustCompile(`(?m)^Device ID:`).Split(raw, -1)
	reDev := regexp.MustCompile(`(?m)^\s*(\S.*)$`)
	reIP := regexp.MustCompile(`(?i)IP(?:v4)? address:\s*(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
	rePlat := regexp.MustCompile(`(?i)Platform:\s*([^,]+)`)
	reIface := regexp.MustCompile(`(?i)Interface:\s*(\S+?),\s*Port ID \(outgoing port\):\s*(\S+)`)
	for i, b := range blocks {
		if i == 0 && !strings.Contains(b, "IP") {
			continue
		}
		n := &diagpb.Neighbor{Protocol: "cdp"}
		if m := reDev.FindStringSubmatch(strings.TrimLeft(b, " \t")); m != nil {
			n.RemoteSystem = strings.TrimSpace(m[1])
		}
		if m := reIP.FindStringSubmatch(b); m != nil {
			n.RemoteMgmtIp = m[1]
		}
		if m := rePlat.FindStringSubmatch(b); m != nil {
			n.RemotePlatform = strings.TrimSpace(m[1])
		}
		if m := reIface.FindStringSubmatch(b); m != nil {
			n.LocalPort = m[1]
			n.RemotePort = m[2]
		}
		if n.RemoteSystem != "" {
			nl.Items = append(nl.Items, n)
		}
	}
	if len(nl.Items) == 0 {
		return nil
	}
	return pNeighbors(nl)
}

// ---- BGP ------------------------------------------------------------------

var (
	reBGPRouterID = regexp.MustCompile(`(?i)BGP router identifier\s+(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}).*?local AS number\s+(\d+)`)
	reBGPNeighHdr = regexp.MustCompile(`(?i)^Neighbor\s+`)
	reNum         = regexp.MustCompile(`^\d+$`)
	reBGPUptime   = regexp.MustCompile(`^(\d+[wdhy][\dwdhmy]*|\d{2}:\d{2}:\d{2}|never)$`)
)

func bgpCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	nl := &diagpb.NeighborList{Protocol: "bgp"}
	if m := reBGPRouterID.FindStringSubmatch(raw); m != nil {
		nl.RouterId = m[1]
		nl.LocalAs = atoi64(m[2])
	}
	inTable := false
	sc := scanLines(raw)
	for sc.Scan() {
		line := sc.Text()
		tl := strings.TrimSpace(line)
		if reBGPNeighHdr.MatchString(tl) {
			inTable = true
			continue
		}
		if !inTable || tl == "" {
			continue
		}
		f := fields(tl)
		// Row starts with a neighbor IP.
		if len(f) < 3 || reIPv4.FindString(f[0]) != f[0] {
			continue
		}
		n := &diagpb.Neighbor{Protocol: "bgp", Peer: f[0]}
		// AS is column index 2 in both layouts: XR "Neighbor Spk AS ...",
		// IOS "Neighbor V AS ...".
		if len(f) > 2 && reNum.MatchString(f[2]) {
			n.RemoteAs = atoi64(f[2])
		}
		// Locate the Up/Down column by its time pattern (e.g. 41w4d, 9w2d,
		// 00:00:00, never). Everything after it is the State/PfxRcd column,
		// which is either a prefix count (Established) or a state that may span
		// multiple tokens (e.g. "Idle (Admin)").
		upIdx := -1
		for i := len(f) - 1; i >= 2; i-- {
			if reBGPUptime.MatchString(f[i]) {
				upIdx = i
				break
			}
		}
		if upIdx >= 0 {
			n.Uptime = f[upIdx]
			rest := f[upIdx+1:]
			switch {
			case len(rest) == 1 && reNum.MatchString(rest[0]):
				n.State = "Established"
				n.PrefixesReceived = atoi64(rest[0])
			case len(rest) > 0:
				n.State = strings.Join(rest, " ")
			}
		} else {
			// No time column matched: fall back to the last token as state.
			last := f[len(f)-1]
			if reNum.MatchString(last) {
				n.State = "Established"
				n.PrefixesReceived = atoi64(last)
			} else {
				n.State = last
			}
		}
		nl.Items = append(nl.Items, n)
	}
	if len(nl.Items) == 0 && nl.RouterId == "" {
		return nil
	}
	return pNeighbors(nl)
}

// ---- OSPF -----------------------------------------------------------------

func ospfCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	nl := &diagpb.NeighborList{Protocol: "ospf"}
	sc := scanLines(raw)
	reUpFor := regexp.MustCompile(`(?i)Neighbor is up for\s+(\S+)`)
	var last *diagpb.Neighbor
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" {
			continue
		}
		if m := reUpFor.FindStringSubmatch(tl); m != nil && last != nil {
			last.Uptime = m[1]
			continue
		}
		f := fields(tl)
		// Row: NeighborID Pri State DeadTime Address Interface
		if len(f) >= 6 && reIPv4.FindString(f[0]) == f[0] && strings.Contains(f[2], "/") {
			n := &diagpb.Neighbor{
				Protocol:     "ospf",
				Peer:         f[0],
				State:        f[2],
				LocalAddress: f[4],
				Interface:    f[5],
			}
			nl.Items = append(nl.Items, n)
			last = n
		}
	}
	if len(nl.Items) == 0 {
		return nil
	}
	return pNeighbors(nl)
}

// ---- IS-IS ----------------------------------------------------------------

func isisCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	if strings.Contains(raw, "No IS-IS") {
		return nil
	}
	nl := &diagpb.NeighborList{Protocol: "isis"}
	sc := scanLines(raw)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" || strings.HasPrefix(tl, "System Id") || strings.HasPrefix(tl, "IS-IS") {
			continue
		}
		f := fields(tl)
		if len(f) < 3 {
			continue
		}
		// System-Id Interface SNPA State Holdtime Type
		n := &diagpb.Neighbor{Protocol: "isis", Peer: f[0], Interface: f[1]}
		for _, c := range f {
			if c == "Up" || c == "Down" || c == "Init" {
				n.State = c
			}
		}
		nl.Items = append(nl.Items, n)
	}
	if len(nl.Items) == 0 {
		return nil
	}
	return pNeighbors(nl)
}

// ---- BFD ------------------------------------------------------------------

func bfdCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	nl := &diagpb.NeighborList{Protocol: "bfd"}
	sc := scanLines(raw)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" || strings.HasPrefix(tl, "Interface") || strings.HasPrefix(tl, "---") ||
			strings.HasPrefix(tl, "Echo") || strings.HasPrefix(tl, "OurAddr") {
			continue
		}
		ip := reIPv4.FindString(tl)
		if ip == "" {
			continue
		}
		n := &diagpb.Neighbor{Protocol: "bfd", Peer: ip}
		low := strings.ToUpper(tl)
		switch {
		case strings.Contains(low, "DOWN"):
			n.State = "DOWN"
		case strings.Contains(low, "UP"):
			n.State = "UP"
		}
		f := fields(tl)
		if len(f) > 0 {
			// interface is usually the first token on XR ("BE60.135"), last on IOS.
			if !strings.Contains(f[0], ".") || reIPv4.FindString(f[0]) == "" {
				n.Interface = f[0]
			}
		}
		nl.Items = append(nl.Items, n)
	}
	if len(nl.Items) == 0 {
		return nil
	}
	return pNeighbors(nl)
}

// ---- RouterOS neighbors (generic key=value detail) ------------------------

func neighRouterOS(proto string) parseFunc {
	return func(raw string) *diagpb.Parsed {
		raw = stripHeader(raw)
		nl := &diagpb.NeighborList{Protocol: proto}
		// RouterOS "print detail" records are separated by blank lines and use
		// key=value tokens. Split on records beginning with a leading index.
		reRec := regexp.MustCompile(`(?m)^\s*\d+\s`)
		locs := reRec.FindAllStringIndex(raw, -1)
		if len(locs) == 0 {
			return nil
		}
		kv := func(block, key string) string {
			m := regexp.MustCompile(key + `=("[^"]*"|\S+)`).FindStringSubmatch(block)
			if m == nil {
				return ""
			}
			return strings.Trim(m[1], `"`)
		}
		for i, loc := range locs {
			end := len(raw)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			block := raw[loc[0]:end]
			n := &diagpb.Neighbor{Protocol: proto}
			n.Peer = firstNonEmpty(kv(block, "address"), kv(block, "remote-address"), kv(block, "remote-router-id"))
			n.RemoteSystem = firstNonEmpty(kv(block, "identity"), kv(block, "name"))
			n.RemotePlatform = kv(block, "platform")
			n.Interface = kv(block, "interface")
			n.State = firstNonEmpty(kv(block, "state"), kv(block, "status"))
			n.Uptime = kv(block, "uptime")
			if v := kv(block, "mac-address"); v != "" {
				n.RemoteChassis = v
			}
			if n.Peer != "" || n.RemoteSystem != "" {
				nl.Items = append(nl.Items, n)
			}
		}
		if len(nl.Items) == 0 {
			return nil
		}
		return pNeighbors(nl)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
