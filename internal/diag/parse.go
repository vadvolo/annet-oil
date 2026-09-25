package diag

import (
	"bufio"
	"strconv"
	"strings"

	"annet-oil/internal/diag/diagpb"
)

// parseFunc turns raw command output into a typed Parsed payload, or nil when it
// cannot extract anything (the caller keeps raw_output regardless).
type parseFunc func(raw string) *diagpb.Parsed

// parsers is keyed by "<family>|<category>". A family value of 0 (unspecified)
// registers a family-agnostic parser used as a fallback.
var parsers = map[string]parseFunc{}

func pkey(f diagpb.Family, category string) string {
	return strconv.Itoa(int(f)) + "|" + category
}

// regParse registers a parser. Called from init() in the per-family files.
func regParse(f diagpb.Family, category string, fn parseFunc) {
	parsers[pkey(f, category)] = fn
}

// regParseAll registers the same parser for several families.
func regParseAll(fams []diagpb.Family, category string, fn parseFunc) {
	for _, f := range fams {
		parsers[pkey(f, category)] = fn
	}
}

// parseCategory dispatches to the most specific registered parser.
func parseCategory(family diagpb.Family, category, raw string) *diagpb.Parsed {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if fn := parsers[pkey(family, category)]; fn != nil {
		return fn(raw)
	}
	if fn := parsers[pkey(diagpb.Family_FAMILY_UNSPECIFIED, category)]; fn != nil {
		return fn(raw)
	}
	return nil
}

// ---- shared helpers -------------------------------------------------------

const maxLineBuffer = 8 * 1024 * 1024

func scanLines(raw string) *bufio.Scanner {
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBuffer)
	return sc
}

// stripHeader drops the collector's "### CMD:" / "### status:" fixture header
// lines if present, so parsers work on both live output and saved fixtures.
func stripHeader(raw string) string {
	var b strings.Builder
	sc := scanLines(raw)
	for sc.Scan() {
		l := sc.Text()
		if strings.HasPrefix(l, "### CMD:") || strings.HasPrefix(l, "### status:") {
			continue
		}
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func atof(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// fields splits on runs of whitespace.
func fields(s string) []string { return strings.Fields(s) }

// normUpDown normalizes status text to "up"/"down" (else lowercased text).
func normUpDown(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case s == "":
		return ""
	case strings.HasPrefix(s, "up") || s == "connected" || s == "active" || s == "running":
		return "up"
	case strings.HasPrefix(s, "down") || s == "notconnect" || strings.Contains(s, "admin") || s == "disabled" || s == "shutdown":
		return "down"
	default:
		return s
	}
}

// parseKV parses RouterOS "key: value" / "key=value" tokens from a blob into a map.
// Handles both the column "name: value" print form and the detail key=value form.
func parseKVColon(raw string) map[string]string {
	m := map[string]string{}
	sc := scanLines(raw)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if i := strings.Index(line, ":"); i > 0 {
			k := strings.TrimSpace(line[:i])
			v := strings.TrimSpace(line[i+1:])
			if k != "" && !strings.Contains(k, " ") || k != "" {
				m[k] = v
			}
		}
	}
	return m
}

// facts/interfaces/etc. constructors wrap a payload in a Parsed oneof.
func pFacts(f *diagpb.DeviceFacts) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Facts{Facts: f}}
}
func pInterfaces(items []*diagpb.Interface) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Interfaces{Interfaces: &diagpb.InterfaceList{Items: items}}}
}
func pNeighbors(n *diagpb.NeighborList) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Neighbors{Neighbors: n}}
}
func pOptics(items []*diagpb.Optics) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Optics{Optics: &diagpb.OpticsList{Items: items}}}
}
func pRoutes(r *diagpb.RouteInfo) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Routes{Routes: r}}
}
func pArp(items []*diagpb.ArpEntry) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Arp{Arp: &diagpb.ArpList{Items: items}}}
}
func pMac(items []*diagpb.MacEntry) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Mac{Mac: &diagpb.MacList{Items: items}}}
}
func pStp(st *diagpb.StpInfo) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Stp{Stp: st}}
}
func pEnv(items []*diagpb.EnvSensor) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Env{Env: &diagpb.EnvSensors{Items: items}}}
}
func pResources(r *diagpb.ResourceUsage) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Resources{Resources: r}}
}
func pLogs(l *diagpb.LogEntries) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Logs{Logs: l}}
}
func pPing(p *diagpb.PingResult) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Ping{Ping: p}}
}
func pTraceroute(t *diagpb.TracerouteResult) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Traceroute{Traceroute: t}}
}
func pRedundancy(r *diagpb.RedundancyInfo) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Redundancy{Redundancy: r}}
}
func pInventory(items []*diagpb.InventoryItem) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Inventory{Inventory: &diagpb.InventoryList{Items: items}}}
}
func pPlatform(items []*diagpb.PlatformCard) *diagpb.Parsed {
	return &diagpb.Parsed{Payload: &diagpb.Parsed_Platform{Platform: &diagpb.PlatformList{Items: items}}}
}
