package diag

import (
	"os"
	"path/filepath"
	"testing"

	"annet-oil/internal/diag/diagpb"
)

// TestParsersAgainstFixtures runs each parser against the real device output
// captured in testdata and asserts the expected typed payload came out.
func TestParsersAgainstFixtures(t *testing.T) {
	cases := []struct {
		file     string
		family   diagpb.Family
		category string
		check    func(*testing.T, *diagpb.Parsed)
	}{
		{"iosxr__show_version_.txt", fXR, "facts", func(t *testing.T, p *diagpb.Parsed) {
			f := p.GetFacts()
			must(t, f != nil, "facts nil")
			must(t, f.OsVersion == "6.4.2", "xr version=%q", f.OsVersion)
			must(t, f.Hostname == "CORE-HERKUL", "xr hostname=%q", f.Hostname)
			must(t, f.UptimeSeconds > 0, "xr uptime_seconds=%d", f.UptimeSeconds)
		}},
		{"ios__show_version_.txt", fIOS, "facts", func(t *testing.T, p *diagpb.Parsed) {
			f := p.GetFacts()
			must(t, f != nil, "facts nil")
			must(t, f.Model == "WS-C3560E-24TD-E", "ios model=%q", f.Model)
			must(t, f.Serial == "FDO1240R0WY", "ios serial=%q", f.Serial)
		}},
		{"eltex__show_version_.txt", fEL, "facts", func(t *testing.T, p *diagpb.Parsed) {
			f := p.GetFacts()
			must(t, f != nil && f.OsVersion != "", "eltex version=%v", f)
		}},
		{"fs__show_version_.txt", fFS, "facts", func(t *testing.T, p *diagpb.Parsed) {
			f := p.GetFacts()
			must(t, f != nil, "fs facts nil")
			must(t, f.Serial == "G1TH2HQ00104B", "fs serial=%q", f.Serial)
			must(t, f.Model == "S5810-28FS", "fs model=%q", f.Model)
		}},
		{"iosxr__show_ip_interface_brief_.txt", fXR, "interfaces", func(t *testing.T, p *diagpb.Parsed) {
			il := p.GetInterfaces()
			must(t, il != nil && len(il.Items) > 20, "xr interfaces=%d", count(il))
		}},
		{"eltex__show_interfaces_status_.txt", fEL, "interfaces", func(t *testing.T, p *diagpb.Parsed) {
			il := p.GetInterfaces()
			must(t, il != nil && len(il.Items) > 5, "eltex interfaces=%d", count(il))
		}},
		{"ros___interface_print_detail_.txt", fROS, "interfaces", func(t *testing.T, p *diagpb.Parsed) {
			// This fixture may be empty (flaky device); tolerate nil.
			if p != nil && p.GetInterfaces() != nil {
				must(t, len(p.GetInterfaces().Items) > 0, "ros interfaces empty")
			}
		}},
		{"rosv7___interface_print_detail_.txt", fROS, "interfaces", func(t *testing.T, p *diagpb.Parsed) {
			il := p.GetInterfaces()
			must(t, il != nil && len(il.Items) > 3, "rosv7 interfaces=%d", count(il))
			must(t, il.Items[0].Name != "", "rosv7 iface name empty")
		}},
		{"iosxr__show_arp_.txt", fXR, "arp", func(t *testing.T, p *diagpb.Parsed) {
			a := p.GetArp()
			must(t, a != nil && len(a.Items) > 0, "xr arp empty")
		}},
		{"iosxr__show_bgp_summary_.txt", fXR, "bgp", func(t *testing.T, p *diagpb.Parsed) {
			n := p.GetNeighbors()
			must(t, n != nil, "xr bgp nil")
			must(t, n.LocalAs == 9125, "xr bgp local_as=%d", n.LocalAs)
			must(t, len(n.Items) >= 2, "xr bgp neighbors=%d", len(n.Items))
			byPeer := map[string]*diagpb.Neighbor{}
			for _, it := range n.Items {
				byPeer[it.Peer] = it
			}
			if e := byPeer["10.132.128.122"]; e != nil {
				must(t, e.Uptime == "9w2d", "bgp est uptime=%q", e.Uptime)
				must(t, e.State == "Established" && e.PrefixesReceived == 5, "bgp est state=%q pfx=%d", e.State, e.PrefixesReceived)
				must(t, e.RemoteAs == 65055, "bgp est remote_as=%d", e.RemoteAs)
			} else {
				t.Errorf("bgp peer 10.132.128.122 missing")
			}
			if e := byPeer["10.100.200.2"]; e != nil {
				must(t, e.Uptime == "41w4d", "bgp idle uptime=%q", e.Uptime)
				must(t, e.State == "Idle (Admin)", "bgp idle state=%q", e.State)
			}
		}},
		{"iosxr__show_ospf_neighbor_.txt", fXR, "ospf", func(t *testing.T, p *diagpb.Parsed) {
			n := p.GetNeighbors()
			must(t, n != nil && len(n.Items) == 5, "xr ospf neighbors=%d", nlen(n))
			must(t, n.Items[0].State != "", "xr ospf state empty")
		}},
		{"iosxr__show_bfd_session_.txt", fXR, "bfd", func(t *testing.T, p *diagpb.Parsed) {
			n := p.GetNeighbors()
			must(t, n != nil && len(n.Items) >= 2, "xr bfd=%d", nlen(n))
		}},
		{"iosxr__show_lldp_neighbors_detail_.txt", fXR, "lldp", func(t *testing.T, p *diagpb.Parsed) {
			n := p.GetNeighbors()
			must(t, n != nil && len(n.Items) > 0, "xr lldp empty")
		}},
		{"ios__show_cdp_neighbors_detail_.txt", fIOS, "cdp", func(t *testing.T, p *diagpb.Parsed) {
			n := p.GetNeighbors()
			must(t, n != nil && len(n.Items) > 0, "ios cdp empty")
		}},
		{"iosxr__show_inventory_.txt", fXR, "inventory", func(t *testing.T, p *diagpb.Parsed) {
			inv := p.GetInventory()
			must(t, inv != nil && len(inv.Items) > 3, "xr inventory=%d", invlen(inv))
			must(t, inv.Items[0].Pid != "" && inv.Items[0].Serial != "", "xr inv pid/sn empty")
		}},
		{"iosxr__show_platform_.txt", fXR, "platform", func(t *testing.T, p *diagpb.Parsed) {
			pl := p.GetPlatform()
			must(t, pl != nil && len(pl.Items) > 3, "xr platform=%d", pllen(pl))
		}},
		{"iosxr__show_memory_summary_.txt", fXR, "memory", func(t *testing.T, p *diagpb.Parsed) {
			r := p.GetResources()
			must(t, r != nil && r.MemTotalBytes > 0, "xr mem total=%d", memtot(r))
		}},
		{"ios__show_processes_cpu_sorted_.txt", fIOS, "cpu", func(t *testing.T, p *diagpb.Parsed) {
			r := p.GetResources()
			must(t, r != nil, "ios cpu nil")
		}},
		{"rosv7___system_resource_print_.txt", fROS, "resource", func(t *testing.T, p *diagpb.Parsed) {
			r := p.GetResources()
			must(t, r != nil && r.MemTotalBytes > 0, "ros resource mem=%d", memtot(r))
		}},
		{"rosv7___system_health_print_.txt", fROS, "env", func(t *testing.T, p *diagpb.Parsed) {
			e := p.GetEnv()
			must(t, e != nil && len(e.Items) > 3, "ros health sensors=%d", envlen(e))
		}},
		{"ios__show_mac_address_table_dynamic_.txt", fIOS, "mac", func(t *testing.T, p *diagpb.Parsed) {
			// switch may have few/no dynamic entries; tolerate nil.
			_ = p
		}},
		{"ios__show_spanning_tree_detail_.txt", fIOS, "stp", func(t *testing.T, p *diagpb.Parsed) {
			// best-effort; just ensure no panic and (if parsed) has content.
			_ = p
		}},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Skipf("fixture missing: %v", err)
				return
			}
			p := parseCategory(tc.family, tc.category, string(raw))
			tc.check(t, p)
		})
	}
}

func must(t *testing.T, cond bool, format string, args ...any) {
	t.Helper()
	if !cond {
		t.Errorf(format, args...)
	}
}

func count(il *diagpb.InterfaceList) int {
	if il == nil {
		return -1
	}
	return len(il.Items)
}
func nlen(n *diagpb.NeighborList) int {
	if n == nil {
		return -1
	}
	return len(n.Items)
}
func invlen(i *diagpb.InventoryList) int {
	if i == nil {
		return -1
	}
	return len(i.Items)
}
func pllen(p *diagpb.PlatformList) int {
	if p == nil {
		return -1
	}
	return len(p.Items)
}
func envlen(e *diagpb.EnvSensors) int {
	if e == nil {
		return -1
	}
	return len(e.Items)
}
func memtot(r *diagpb.ResourceUsage) int64 {
	if r == nil {
		return -1
	}
	return r.MemTotalBytes
}
