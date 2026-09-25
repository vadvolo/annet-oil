package diag

import (
	"regexp"
	"strings"

	"annet-oil/internal/diag/diagpb"
)

func init() {
	cisco := []diagpb.Family{diagpb.Family_FAMILY_IOSXR, diagpb.Family_FAMILY_IOS}
	ciscoLike := []diagpb.Family{diagpb.Family_FAMILY_IOSXR, diagpb.Family_FAMILY_IOS, diagpb.Family_FAMILY_ELTEX, diagpb.Family_FAMILY_FS}

	regParse(diagpb.Family_FAMILY_IOSXR, "platform", platformXR)
	regParseAll(cisco, "inventory", inventoryCisco)
	regParseAll(ciscoLike, "cpu", cpuCisco)
	regParseAll(ciscoLike, "memory", memoryCisco)
	regParseAll(ciscoLike, "env", envCisco)
	regParseAll(ciscoLike, "redundancy", redundancyCisco)

	regParse(diagpb.Family_FAMILY_ROUTEROS, "resource", resourceRouterOS)
	regParse(diagpb.Family_FAMILY_ROUTEROS, "env", healthRouterOS) // /system health print
	regParse(diagpb.Family_FAMILY_ROUTEROS, "redundancy", redundancyRouterOS)
}

// ---- Cisco platform / inventory -------------------------------------------

func platformXR(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	var items []*diagpb.PlatformCard
	sc := scanLines(raw)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" || strings.HasPrefix(tl, "Node") || strings.HasPrefix(tl, "---") {
			continue
		}
		f := fields(tl)
		if len(f) < 3 {
			continue
		}
		c := &diagpb.PlatformCard{Node: f[0], Type: f[1]}
		// State can be multiple words ("IOS XR RUN"); config state is the tail token.
		c.State = strings.Join(f[2:len(f)-1], " ")
		c.ConfigState = f[len(f)-1]
		if c.State == "" {
			c.State = f[len(f)-1]
			c.ConfigState = ""
		}
		items = append(items, c)
	}
	if len(items) == 0 {
		return nil
	}
	return pPlatform(items)
}

var (
	reInvName = regexp.MustCompile(`(?i)NAME:\s*"([^"]*)",\s*DESCR:\s*"([^"]*)"`)
	reInvPID  = regexp.MustCompile(`(?i)PID:\s*(\S+)\s*,?\s*VID:\s*(\S+)\s*,?\s*SN:\s*(\S+)`)
)

func inventoryCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	var items []*diagpb.InventoryItem
	names := reInvName.FindAllStringSubmatch(raw, -1)
	pids := reInvPID.FindAllStringSubmatch(raw, -1)
	for i := range names {
		it := &diagpb.InventoryItem{Name: names[i][1], Description: names[i][2]}
		if i < len(pids) {
			it.Pid = pids[i][1]
			it.Vid = pids[i][2]
			it.Serial = pids[i][3]
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		return nil
	}
	return pInventory(items)
}

// ---- Cisco CPU / memory ---------------------------------------------------

var (
	reCPU5s = regexp.MustCompile(`(?i)five seconds:\s*(\d+)%`)
	reCPU1m = regexp.MustCompile(`(?i)one minute:\s*(\d+)%`)
	reCPU5m = regexp.MustCompile(`(?i)five minutes:\s*(\d+)%`)
)

func cpuCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	r := &diagpb.ResourceUsage{}
	if m := reCPU5s.FindStringSubmatch(raw); m != nil {
		r.Cpu_5S = atof(m[1])
	}
	if m := reCPU1m.FindStringSubmatch(raw); m != nil {
		r.Cpu_1M = atof(m[1])
	}
	if m := reCPU5m.FindStringSubmatch(raw); m != nil {
		r.Cpu_5M = atof(m[1])
	}
	// Top processes: rows "... 5Sec 1Min 5Min TTY Process". Heuristic: a line with
	// a trailing "N.NN%"/"N%" percent columns and a process name at the end.
	reProc := regexp.MustCompile(`^\s*(\d+)\s+.*?\s(\d+\.?\d*)%\s+\d+\.?\d*%\s+\d+\.?\d*%\s+\d+\s+(.+?)\s*$`)
	sc := scanLines(raw)
	for sc.Scan() {
		if m := reProc.FindStringSubmatch(sc.Text()); m != nil {
			r.TopProcesses = append(r.TopProcesses, &diagpb.ProcessCpu{
				Pid: atoi64(m[1]), CpuPct: atof(m[2]), Name: strings.TrimSpace(m[3]),
			})
		}
	}
	if r.Cpu_5S == 0 && r.Cpu_1M == 0 && r.Cpu_5M == 0 && len(r.TopProcesses) == 0 {
		return nil
	}
	return pResources(r)
}

var (
	rePhysMem = regexp.MustCompile(`(?i)Physical Memory:\s*(\d+)M\s*total\s*\((\d+)M\s*available\)`)
	reMemTUF  = regexp.MustCompile(`(?i)Total:\s*(\d+),\s*Used:\s*(\d+),\s*Free:\s*(\d+)`)
)

func memoryCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	r := &diagpb.ResourceUsage{}
	if m := rePhysMem.FindStringSubmatch(raw); m != nil {
		r.MemTotalBytes = atoi64(m[1]) * 1024 * 1024
		r.MemFreeBytes = atoi64(m[2]) * 1024 * 1024
		r.MemUsedBytes = r.MemTotalBytes - r.MemFreeBytes
	} else if m := reMemTUF.FindStringSubmatch(raw); m != nil {
		r.MemTotalBytes = atoi64(m[1])
		r.MemUsedBytes = atoi64(m[2])
		r.MemFreeBytes = atoi64(m[3])
	} else {
		return nil
	}
	return pResources(r)
}

// ---- Cisco environment (best-effort sensor extraction) --------------------

func envCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	var items []*diagpb.EnvSensor
	reTemp := regexp.MustCompile(`(?i)(temperature|temp).*?(\d+)\s*(?:C|Celsius|degrees)`)
	reFan := regexp.MustCompile(`(?i)(fan\s*\w*).*?(\d+)\s*RPM`)
	rePower := regexp.MustCompile(`(?i)(power|psu|supply)\b.*?\b(ok|good|normal|fail|failed|critical|warning|off)\b`)
	sc := scanLines(raw)
	for sc.Scan() {
		line := sc.Text()
		if m := reTemp.FindStringSubmatch(line); m != nil {
			items = append(items, &diagpb.EnvSensor{Name: strings.TrimSpace(m[1]), Type: "temperature", Value: atof(m[2]), Unit: "C"})
		}
		if m := reFan.FindStringSubmatch(line); m != nil {
			items = append(items, &diagpb.EnvSensor{Name: strings.TrimSpace(m[1]), Type: "fan", Value: atof(m[2]), Unit: "RPM"})
		}
		if m := rePower.FindStringSubmatch(line); m != nil {
			items = append(items, &diagpb.EnvSensor{Name: strings.TrimSpace(m[1]), Type: "psu", ValueText: strings.ToLower(m[2]), Status: normSensorStatus(m[2])})
		}
	}
	if len(items) == 0 {
		return nil
	}
	return pEnv(items)
}

func normSensorStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ok", "good", "normal":
		return "ok"
	case "warning", "warn":
		return "warning"
	case "fail", "failed", "critical", "off":
		return "critical"
	default:
		return strings.ToLower(s)
	}
}

// ---- Cisco redundancy (HSRP/VRRP) -----------------------------------------

func redundancyCisco(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	info := &diagpb.RedundancyInfo{}
	sc := scanLines(raw)
	proto := "hsrp"
	if strings.Contains(strings.ToLower(raw), "vrrp") {
		proto = "vrrp"
	}
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" || strings.HasPrefix(tl, "Interface") || strings.HasPrefix(tl, "---") {
			continue
		}
		f := fields(tl)
		if len(f) < 3 {
			continue
		}
		// show standby brief: Iface Grp Pri P State Active Standby Virtual
		g := &diagpb.RedundancyGroup{Protocol: proto, Interface: f[0], GroupId: f[1]}
		low := strings.ToLower(tl)
		for _, st := range []string{"master", "backup", "active", "standby", "init", "listen", "speak"} {
			if strings.Contains(low, st) {
				g.State = st
				break
			}
		}
		if ip := reIPv4.FindAllString(tl, -1); len(ip) > 0 {
			g.VirtualIp = ip[len(ip)-1]
		}
		info.Groups = append(info.Groups, g)
	}
	if len(info.Groups) == 0 {
		return nil
	}
	return pRedundancy(info)
}

// ---- RouterOS resource / health / vrrp ------------------------------------

var (
	reRosCPULoad = regexp.MustCompile(`(?i)cpu-load:\s*(\d+)%?`)
	reRosFreeMem = regexp.MustCompile(`(?i)free-memory:\s*([\d.]+)([KMG])iB`)
	reRosTotMem  = regexp.MustCompile(`(?i)total-memory:\s*([\d.]+)([KMG])iB`)
	reRosVer     = regexp.MustCompile(`(?i)version:\s*(\S+)`)
	reRosBoard   = regexp.MustCompile(`(?i)board-name:\s*(.+)`)
	reRosArch    = regexp.MustCompile(`(?i)architecture-name:\s*(\S+)`)
	reRosUptime  = regexp.MustCompile(`(?i)uptime:\s*(\S+)`)
)

func iecToBytes(num float64, unit string) int64 {
	switch strings.ToUpper(unit) {
	case "K":
		return int64(num * 1024)
	case "M":
		return int64(num * 1024 * 1024)
	case "G":
		return int64(num * 1024 * 1024 * 1024)
	}
	return int64(num)
}

func resourceRouterOS(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	r := &diagpb.ResourceUsage{}
	if m := reRosCPULoad.FindStringSubmatch(raw); m != nil {
		r.CpuLoadPct = atof(m[1])
	}
	if m := reRosFreeMem.FindStringSubmatch(raw); m != nil {
		r.MemFreeBytes = iecToBytes(atof(m[1]), m[2])
		r.MemFreeText = m[1] + m[2] + "iB"
	}
	if m := reRosTotMem.FindStringSubmatch(raw); m != nil {
		r.MemTotalBytes = iecToBytes(atof(m[1]), m[2])
		r.MemTotalText = m[1] + m[2] + "iB"
	}
	if r.MemTotalBytes > 0 && r.MemFreeBytes > 0 {
		r.MemUsedBytes = r.MemTotalBytes - r.MemFreeBytes
	}
	if r.CpuLoadPct == 0 && r.MemTotalBytes == 0 {
		return nil
	}
	return pResources(r)
}

// healthRouterOS parses "/system health print" columns: # NAME VALUE TYPE
func healthRouterOS(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	var items []*diagpb.EnvSensor
	sc := scanLines(raw)
	for sc.Scan() {
		tl := strings.TrimSpace(sc.Text())
		if tl == "" || strings.HasPrefix(tl, "Columns:") || strings.HasPrefix(tl, "#") {
			continue
		}
		f := fields(tl)
		if len(f) < 3 {
			continue
		}
		// f: idx name value [unit]
		name := f[1]
		val := f[2]
		unit := ""
		if len(f) >= 4 {
			unit = f[3]
		}
		sensor := &diagpb.EnvSensor{Name: name, Unit: unit}
		if regexp.MustCompile(`^-?\d+(\.\d+)?$`).MatchString(val) {
			sensor.Value = atof(val)
		} else {
			sensor.ValueText = val
			sensor.Status = normSensorStatus(val)
		}
		lname := strings.ToLower(name)
		switch {
		case strings.Contains(lname, "temp"):
			sensor.Type = "temperature"
		case strings.Contains(lname, "fan"):
			sensor.Type = "fan"
		case strings.Contains(lname, "volt"):
			sensor.Type = "voltage"
		case strings.Contains(lname, "consumption") || strings.Contains(lname, "power") || unit == "W":
			sensor.Type = "power"
		case strings.Contains(lname, "current") || unit == "A":
			sensor.Type = "current"
		}
		items = append(items, sensor)
	}
	if len(items) == 0 {
		return nil
	}
	return pEnv(items)
}

func redundancyRouterOS(raw string) *diagpb.Parsed {
	raw = stripHeader(raw)
	info := &diagpb.RedundancyInfo{}
	reRec := regexp.MustCompile(`(?m)^\s*\d+\s`)
	locs := reRec.FindAllStringIndex(raw, -1)
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
		g := &diagpb.RedundancyGroup{
			Protocol:  "vrrp",
			Interface: kv(block, "interface"),
			GroupId:   kv(block, "vrid"),
			State:     firstNonEmpty(kv(block, "master"), kv(block, "state")),
			Priority:  int32(atoi(kv(block, "priority"))),
		}
		if strings.Contains(block, "master") {
			g.State = "master"
		} else if strings.Contains(block, "backup") {
			g.State = "backup"
		}
		info.Groups = append(info.Groups, g)
	}
	if len(info.Groups) == 0 {
		return nil
	}
	return pRedundancy(info)
}
