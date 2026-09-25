package diag

import (
	"regexp"
	"strings"

	"annet-oil/internal/diag/diagpb"
)

func init() {
	regParse(diagpb.Family_FAMILY_IOSXR, "facts", func(raw string) *diagpb.Parsed { return factsCisco(raw, true) })
	regParse(diagpb.Family_FAMILY_IOS, "facts", func(raw string) *diagpb.Parsed { return factsCisco(raw, false) })
	regParse(diagpb.Family_FAMILY_ELTEX, "facts", factsEltex)
	regParse(diagpb.Family_FAMILY_FS, "facts", factsFS)
	// RouterOS identity -> hostname facts.
	regParse(diagpb.Family_FAMILY_ROUTEROS, "identity", factsRouterOSIdentity)
}

var (
	reXRVersion  = regexp.MustCompile(`Cisco IOS XR Software.*?Version\s+([^\s\[]+)`)
	reIOSVersion = regexp.MustCompile(`Cisco IOS.*?Version\s+([^\s,]+)`)
	reUptime     = regexp.MustCompile(`(?m)^(\S.*?)\s+uptime is\s+(.+)$`)
	reImage      = regexp.MustCompile(`System image file is\s+"([^"]+)"`)
	reSerialCol  = regexp.MustCompile(`(?i)System serial number\s*:\s*(\S+)`)
	reModelNum   = regexp.MustCompile(`(?i)Model number\s*:\s*(\S+)`)
	reCiscoProc  = regexp.MustCompile(`(?m)^[Cc]isco\s+(\S+)\s+.*processor`)
	reRestarted  = regexp.MustCompile(`System restarted at\s+(.+)`)
	reReturnedTo = regexp.MustCompile(`System returned to ROM by\s+(.+)`)
)

func factsCisco(raw string, xr bool) *diagpb.Parsed {
	raw = stripHeader(raw)
	f := &diagpb.DeviceFacts{Vendor: "cisco"}
	if m := (map[bool]*regexp.Regexp{true: reXRVersion, false: reIOSVersion}[xr]).FindStringSubmatch(raw); m != nil {
		f.OsVersion = m[1]
	}
	if m := reUptime.FindStringSubmatch(raw); m != nil {
		f.Hostname = strings.TrimSpace(m[1])
		f.Uptime = strings.TrimSpace(m[2])
		f.UptimeSeconds = parseUptimeToSeconds(f.Uptime)
	}
	if m := reImage.FindStringSubmatch(raw); m != nil {
		f.Image = m[1]
	}
	if m := reSerialCol.FindStringSubmatch(raw); m != nil {
		f.Serial = m[1]
	}
	if m := reModelNum.FindStringSubmatch(raw); m != nil {
		f.Model = m[1]
	} else if m := reCiscoProc.FindStringSubmatch(raw); m != nil {
		f.Model = m[1]
	}
	if m := reRestarted.FindStringSubmatch(raw); m != nil {
		f.RestartedAt = strings.TrimSpace(m[1])
	}
	if m := reReturnedTo.FindStringSubmatch(raw); m != nil {
		f.ReloadReason = strings.TrimSpace(m[1])
	}
	if f.OsVersion == "" && f.Hostname == "" && f.Model == "" {
		return nil
	}
	return pFacts(f)
}

var (
	reEltexActive  = regexp.MustCompile(`(?m)Active-image:.*?\n\s*Version:\s*(\S+)`)
	reEltexVerLine = regexp.MustCompile(`(?m)^\s*Version:\s*(\S+)`)
)

func factsEltex(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	f := &diagpb.DeviceFacts{Vendor: "eltex"}
	if m := reEltexActive.FindStringSubmatch(raw); m != nil {
		f.OsVersion = m[1]
	} else if m := reEltexVerLine.FindStringSubmatch(raw); m != nil {
		f.OsVersion = m[1]
	}
	// Eltex "show system" often carries model/uptime; capture if present here too.
	if m := regexp.MustCompile(`(?i)System Description:\s*(.+)`).FindStringSubmatch(raw); m != nil {
		f.Model = strings.TrimSpace(m[1])
	}
	if m := regexp.MustCompile(`(?i)System Up Time.*?:\s*(.+)`).FindStringSubmatch(raw); m != nil {
		f.Uptime = strings.TrimSpace(m[1])
		f.UptimeSeconds = parseUptimeToSeconds(f.Uptime)
	}
	if f.OsVersion == "" && f.Model == "" {
		return nil
	}
	return pFacts(f)
}

func factsFS(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	f := &diagpb.DeviceFacts{Vendor: "fs"}
	if m := regexp.MustCompile(`(?i)System software version\s*:\s*(.+)`).FindStringSubmatch(raw); m != nil {
		f.OsVersion = strings.TrimSpace(m[1])
	}
	if m := regexp.MustCompile(`(?i)System serial number\s*:\s*(\S+)`).FindStringSubmatch(raw); m != nil {
		f.Serial = m[1]
	}
	if m := regexp.MustCompile(`(?i)System uptime\s*:\s*(.+)`).FindStringSubmatch(raw); m != nil {
		f.Uptime = strings.TrimSpace(m[1])
		f.UptimeSeconds = parseUptimeToSeconds(f.Uptime)
	}
	if m := regexp.MustCompile(`\(([A-Z0-9-]+)\)\s+By FS`).FindStringSubmatch(raw); m != nil {
		f.Model = m[1]
	}
	if f.OsVersion == "" && f.Serial == "" && f.Model == "" {
		return nil
	}
	return pFacts(f)
}

func factsRouterOSIdentity(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	if m := regexp.MustCompile(`(?i)name:\s*(.+)`).FindStringSubmatch(raw); m != nil {
		return pFacts(&diagpb.DeviceFacts{Vendor: "mikrotik", Hostname: strings.TrimSpace(m[1])})
	}
	return nil
}

// parseUptimeToSeconds handles the common uptime spellings:
//   - Cisco:    "3 years, 7 weeks, 3 days, 1 hour, 28 minutes"
//   - RouterOS: "9w4d16h24m54s"
//   - FS:       "90:14:37:48" (d:h:m:s)  or "14:37:48" (h:m:s)
var (
	reUptimeWord    = regexp.MustCompile(`(\d+)\s*(year|week|day|hour|minute|second)s?`)
	reUptimeCompact = regexp.MustCompile(`(\d+)\s*([wdhms])`)
	reUptimeColon   = regexp.MustCompile(`^\s*(\d+):(\d+):(\d+):(\d+)\s*$|^\s*(\d+):(\d+):(\d+)\s*$`)
)

func parseUptimeToSeconds(s string) int64 {
	s = strings.TrimSpace(s)
	unit := map[string]int64{"year": 31536000, "week": 604800, "day": 86400, "hour": 3600, "minute": 60, "second": 1}
	var total int64
	if ms := reUptimeWord.FindAllStringSubmatch(s, -1); len(ms) > 0 {
		for _, m := range ms {
			total += atoi64(m[1]) * unit[m[2]]
		}
		return total
	}
	if m := reUptimeColon.FindStringSubmatch(s); m != nil {
		if m[1] != "" {
			return atoi64(m[1])*86400 + atoi64(m[2])*3600 + atoi64(m[3])*60 + atoi64(m[4])
		}
		return atoi64(m[5])*3600 + atoi64(m[6])*60 + atoi64(m[7])
	}
	compact := map[string]int64{"w": 604800, "d": 86400, "h": 3600, "m": 60, "s": 1}
	if ms := reUptimeCompact.FindAllStringSubmatch(s, -1); len(ms) > 0 {
		for _, m := range ms {
			total += atoi64(m[1]) * compact[m[2]]
		}
	}
	return total
}
