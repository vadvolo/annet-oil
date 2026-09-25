package diag

import (
	"regexp"
	"strings"

	"annet-oil/internal/diag/diagpb"
)

func init() {
	ciscoLike := []diagpb.Family{diagpb.Family_FAMILY_IOSXR, diagpb.Family_FAMILY_IOS, diagpb.Family_FAMILY_ELTEX, diagpb.Family_FAMILY_FS}

	regParseAll(ciscoLike, "log", logCisco)
	regParse(diagpb.Family_FAMILY_ROUTEROS, "log", logRouterOS)

	regParseAll(ciscoLike, "ping", pingCisco)
	regParse(diagpb.Family_FAMILY_ROUTEROS, "ping", pingRouterOS)

	regParseAll(ciscoLike, "traceroute", tracerouteCisco)
	regParse(diagpb.Family_FAMILY_ROUTEROS, "traceroute", tracerouteRouterOS)

	regParseAll(ciscoLike, "stp", stpCisco)
}

// ---- logs -----------------------------------------------------------------

const maxLogEntries = 200

var reCiscoLog = regexp.MustCompile(`%([A-Z0-9_]+)-(\d)-([A-Z0-9_]+):\s*(.*)$`)
var reCiscoLogTime = regexp.MustCompile(`^[*.]?([A-Z][a-z]{2}\s+\d+\s+[\d:.]+|\d{4}\s\w{3}\s+\d+\s[\d:.]+)`)

var sevName = map[string]string{
	"0": "emergency", "1": "alert", "2": "critical", "3": "error",
	"4": "warning", "5": "notice", "6": "info", "7": "debug",
}

func logCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	l := &diagpb.LogEntries{}
	sc := scanLines(raw)
	for sc.Scan() {
		line := sc.Text()
		tl := strings.TrimSpace(line)
		if tl == "" {
			continue
		}
		e := &diagpb.LogEntry{Message: tl}
		if m := reCiscoLog.FindStringSubmatch(line); m != nil {
			e.Facility = m[1]
			e.Severity = sevName[m[2]]
			e.Message = strings.TrimSpace(m[4])
		}
		if m := reCiscoLogTime.FindStringSubmatch(tl); m != nil {
			e.Timestamp = strings.TrimSpace(m[1])
		}
		l.Items = append(l.Items, e)
		if len(l.Items) >= maxLogEntries {
			break
		}
	}
	if len(l.Items) == 0 {
		return nil
	}
	l.Shown = int32(len(l.Items))
	return pLogs(l)
}

func logRouterOS(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	l := &diagpb.LogEntries{}
	sc := scanLines(raw)
	// Rows: "<time> <topics> <message>", e.g. "sep/25 10:00:00 system,info ...".
	reRow := regexp.MustCompile(`^\s*(?:\d+\s+)?(\S+\s+[\d:]+|\S+)\s+([a-z,]+)\s+(.*)$`)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" || strings.HasPrefix(tl, "Flags") || strings.Contains(tl, "TIME") {
			continue
		}
		e := &diagpb.LogEntry{Message: tl}
		if m := reRow.FindStringSubmatch(tl); m != nil {
			e.Timestamp = m[1]
			topics := m[2]
			e.Facility = topics
			for _, sev := range []string{"error", "warning", "critical", "info", "debug"} {
				if strings.Contains(topics, sev) {
					e.Severity = sev
				}
			}
			e.Message = strings.TrimSpace(m[3])
		}
		l.Items = append(l.Items, e)
		if len(l.Items) >= maxLogEntries {
			break
		}
	}
	if len(l.Items) == 0 {
		return nil
	}
	l.Shown = int32(len(l.Items))
	return pLogs(l)
}

// ---- ping -----------------------------------------------------------------

var (
	reCiscoPing = regexp.MustCompile(`(?i)Success rate is (\d+) percent \((\d+)/(\d+)\)`)
	reCiscoRTT  = regexp.MustCompile(`(?i)min/avg/max = ([\d.]+)/([\d.]+)/([\d.]+)`)
)

func pingCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	p := &diagpb.PingResult{}
	if m := reCiscoPing.FindStringSubmatch(raw); m != nil {
		p.PacketsReceived = int32(atoi(m[2]))
		p.PacketsSent = int32(atoi(m[3]))
		if p.PacketsSent > 0 {
			p.LossPct = float64(p.PacketsSent-p.PacketsReceived) / float64(p.PacketsSent) * 100
		}
		p.Success = atoi(m[1]) > 0
	} else {
		return nil
	}
	if m := reCiscoRTT.FindStringSubmatch(raw); m != nil {
		p.RttMinMs = atof(m[1])
		p.RttAvgMs = atof(m[2])
		p.RttMaxMs = atof(m[3])
	}
	return pPing(p)
}

var (
	reRosPingSummary = regexp.MustCompile(`(?i)sent=(\d+).*?received=(\d+).*?packet-loss=(\d+)%`)
	reRosPingRTT     = regexp.MustCompile(`(?i)min-rtt=([\d.]+)m?s.*?avg-rtt=([\d.]+)m?s.*?max-rtt=([\d.]+)m?s`)
)

func pingRouterOS(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	p := &diagpb.PingResult{}
	// Collapse to one line for the summary regex (RouterOS prints it across the tail).
	flat := strings.Join(strings.Fields(raw), " ")
	if m := reRosPingSummary.FindStringSubmatch(flat); m != nil {
		p.PacketsSent = int32(atoi(m[1]))
		p.PacketsReceived = int32(atoi(m[2]))
		p.LossPct = atof(m[3])
		p.Success = p.PacketsReceived > 0
	} else {
		return nil
	}
	if m := reRosPingRTT.FindStringSubmatch(flat); m != nil {
		p.RttMinMs = atof(m[1])
		p.RttAvgMs = atof(m[2])
		p.RttMaxMs = atof(m[3])
	}
	return pPing(p)
}

// ---- traceroute -----------------------------------------------------------

func tracerouteCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	t := &diagpb.TracerouteResult{}
	sc := scanLines(raw)
	reHop := regexp.MustCompile(`^\s*(\d+)\s+(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
	for sc.Scan() {
		if m := reHop.FindStringSubmatch(sc.Text()); m != nil {
			t.Hops = append(t.Hops, &diagpb.TracerouteHop{Index: int32(atoi(m[1])), Address: m[2]})
		}
	}
	if len(t.Hops) == 0 {
		return nil
	}
	return pTraceroute(t)
}

func tracerouteRouterOS(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	t := &diagpb.TracerouteResult{}
	sc := scanLines(raw)
	idx := 0
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" || strings.Contains(tl, "ADDRESS") || strings.HasPrefix(tl, "#") {
			continue
		}
		addr := reIPv4.FindString(tl)
		if addr == "" {
			continue
		}
		idx++
		hop := &diagpb.TracerouteHop{Index: int32(idx), Address: addr}
		// A time like "1.2ms" may appear.
		if m := regexp.MustCompile(`([\d.]+)ms`).FindStringSubmatch(tl); m != nil {
			hop.RttMs = atof(m[1])
		}
		t.Hops = append(t.Hops, hop)
	}
	if len(t.Hops) == 0 {
		return nil
	}
	return pTraceroute(t)
}

// ---- STP ------------------------------------------------------------------

func stpCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	st := &diagpb.StpInfo{}
	inst := &diagpb.StpInstance{}
	low := strings.ToLower(raw)
	if strings.Contains(low, "this bridge is the root") || strings.Contains(low, "we are the root") {
		inst.IsRoot = true
	}
	if m := regexp.MustCompile(`(?i)Root ID\s+.*?(\S{4}\.\S{4}\.\S{4})`).FindStringSubmatch(raw); m != nil {
		inst.RootId = m[1]
	}
	if m := regexp.MustCompile(`(?i)Bridge ID\s+.*?(\S{4}\.\S{4}\.\S{4})`).FindStringSubmatch(raw); m != nil {
		inst.BridgeId = m[1]
	}
	// Per-port state rows: "Gi0/1  Desg FWD 4 ..." or Eltex "gi1/0/1  ... Forwarding".
	rePort := regexp.MustCompile(`(?i)^(\S+)\s+(Root|Desg|Altn|Back|Designated|Alternate|Backup|Disabled)\s+(FWD|BLK|LRN|LIS|DIS|Forwarding|Blocking|Learning|Listening|Disabled)`)
	sc := scanLines(raw)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if m := rePort.FindStringSubmatch(tl); m != nil {
			inst.Ports = append(inst.Ports, &diagpb.StpPort{
				Name: m[1], Role: strings.ToLower(m[2]), State: strings.ToLower(m[3]),
			})
		}
		// blocked ports lines
		if strings.Contains(strings.ToLower(tl), "block") {
			for _, f := range fields(tl) {
				if regexp.MustCompile(`^(Gi|Te|Fa|Eth|gi|te|fa|eth)\S*`).MatchString(f) {
					st.BlockedPorts = append(st.BlockedPorts, f)
				}
			}
		}
	}
	if inst.RootId != "" || inst.BridgeId != "" || inst.IsRoot || len(inst.Ports) > 0 {
		st.Instances = append(st.Instances, inst)
	}
	if len(st.Instances) == 0 && len(st.BlockedPorts) == 0 {
		return nil
	}
	return pStp(st)
}
