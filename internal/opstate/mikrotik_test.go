package opstate

import "testing"

func TestMikrotikFacts(t *testing.T) {
	raw := `                   uptime: 1w2d3h4m5s
                  version: 6.48.6 (stable)
               build-time: Aug/19/2021 14:11:36
               board-name: CCR1009-7G-1C-1S+
                 platform: MikroTik
`
	f := parseROSResource(raw)
	if f.OSVersion != "6.48.6 (stable)" {
		t.Errorf("os_version=%q", f.OSVersion)
	}
	if f.Model != "CCR1009-7G-1C-1S+" {
		t.Errorf("model=%q", f.Model)
	}
	if f.Uptime != "1w2d3h4m5s" {
		t.Errorf("uptime=%q", f.Uptime)
	}
}

func TestMikrotikInterfaces(t *testing.T) {
	raw := `Flags: D - dynamic, X - disabled, R - running, S - slave
 0  R  name=ether1 default-name=ether1 type=ether mtu=1500 actual-mtu=1500 mac-address=E4:8D:8C:00:00:01 comment=uplink
 1  X  name=ether2 default-name=ether2 type=ether mtu=1500 actual-mtu=1500 mac-address=E4:8D:8C:00:00:02
`
	ifs := parseROSInterfaces(raw)
	if len(ifs) != 2 {
		t.Fatalf("interfaces=%d want 2", len(ifs))
	}
	if ifs[0].Name != "ether1" || ifs[0].OperStatus != "up" || ifs[0].AdminStatus != "up" {
		t.Errorf("if0=%+v", ifs[0])
	}
	if ifs[0].MTU != 1500 || ifs[0].MAC != "e4:8d:8c:00:00:01" || ifs[0].Description != "uplink" {
		t.Errorf("if0 detail=%+v", ifs[0])
	}
	// ether2 is disabled (X) and not running.
	if ifs[1].AdminStatus != "down" || ifs[1].OperStatus != "down" {
		t.Errorf("if1 status=%+v", ifs[1])
	}
}

func TestMikrotikNeighbors(t *testing.T) {
	raw := `Flags: X - disabled
 0 interface=ether1 address=10.0.0.2 mac-address=E4:8D:8C:00:00:99 identity=switch2 platform=MikroTik board=CRS326 interface-name=ether5
`
	n := parseROSNeighbors(raw)
	if len(n) != 1 {
		t.Fatalf("neighbors=%d want 1", len(n))
	}
	got := n[0]
	if got.LocalPort != "ether1" || got.RemoteSystem != "switch2" || got.RemotePort != "ether5" {
		t.Errorf("neighbor=%+v", got)
	}
	if got.RemoteChassis != "e4:8d:8c:00:00:99" || got.RemoteMgmtIP != "10.0.0.2" {
		t.Errorf("chassis/mgmt=%+v", got)
	}
}

func TestMikrotikBridgeHostsAndARP(t *testing.T) {
	mac := parseROSBridgeHosts(`Flags: D - dynamic, L - local
 0 D mac-address=AA:BB:CC:00:11:00 on-interface=ether1 bridge=bridge1 vid=10 dynamic=yes
 1   mac-address=AA:BB:CC:00:22:00 on-interface=ether2 bridge=bridge1 vid=20 dynamic=no
`)
	if len(mac) != 2 {
		t.Fatalf("mac=%d want 2", len(mac))
	}
	if mac[0].MAC != "aa:bb:cc:00:11:00" || mac[0].VLAN != 10 || mac[0].Interface != "ether1" || mac[0].Type != "dynamic" {
		t.Errorf("mac0=%+v", mac[0])
	}
	if mac[1].Type != "static" {
		t.Errorf("mac1 type=%q", mac[1].Type)
	}

	arp := parseROSARP(`Flags: D - dynamic, C - complete
 0 DC address=10.0.0.2 mac-address=AA:BB:CC:00:11:00 interface=ether1
 1 DC address=10.0.0.3 mac-address=AA:BB:CC:00:22:00 interface=ether2
`)
	if len(arp) != 2 || arp[0].IP != "10.0.0.2" || arp[0].MAC != "aa:bb:cc:00:11:00" || arp[0].Interface != "ether1" {
		t.Errorf("arp=%+v", arp)
	}
}

func TestMikrotikRoutesV6(t *testing.T) {
	// v6: connect/static are uppercase C/S in the flag column.
	p := &mikrotikParser{major: 6}
	routes := p.parseROSRoutes(`Flags: X - disabled, A - active, D - dynamic, C - connect, S - static, o - ospf, b - bgp
 0 A S  dst-address=0.0.0.0/0 gateway=10.0.0.254 gateway-status=10.0.0.254 reachable via ether1 distance=1
 1 ADC  dst-address=10.0.0.0/24 pref-src=10.0.0.1 gateway=ether1 gateway-status=ether1 reachable distance=0
`)
	if len(routes) != 2 {
		t.Fatalf("routes=%d want 2: %+v", len(routes), routes)
	}
	if routes[0].Prefix != "0.0.0.0/0" || routes[0].Protocol != "static" || routes[0].NextHop != "10.0.0.254" {
		t.Errorf("route0=%+v", routes[0])
	}
	if routes[1].Prefix != "10.0.0.0/24" || routes[1].Protocol != "connected" || routes[1].Interface != "ether1" {
		t.Errorf("route1=%+v", routes[1])
	}
}

func TestMikrotikRoutesV7(t *testing.T) {
	// v7: route origin is lowercase (c/s); uppercase letters are state flags.
	// v7 also emits routing-table and immediate-gw fields.
	p := &mikrotikParser{major: 7}
	routes := p.parseROSRoutes(`Flags: X, F, U, A - ACTIVE; c - CONNECT; s - STATIC; d - DHCP
 0 As  dst-address=0.0.0.0/0 routing-table=main gateway=10.0.0.254 immediate-gw=10.0.0.254%ether1 distance=1
 1 Dac dst-address=10.0.0.0/24 routing-table=main gateway=ether1 immediate-gw=ether1 pref-src=10.0.0.1
`)
	if len(routes) != 2 {
		t.Fatalf("routes=%d want 2: %+v", len(routes), routes)
	}
	if routes[0].Prefix != "0.0.0.0/0" || routes[0].Protocol != "static" || routes[0].NextHop != "10.0.0.254" {
		t.Errorf("route0=%+v", routes[0])
	}
	if routes[1].Prefix != "10.0.0.0/24" || routes[1].Protocol != "connected" || routes[1].Interface != "ether1" {
		t.Errorf("route1=%+v", routes[1])
	}
}

func TestMikrotikMajorVersionFromFacts(t *testing.T) {
	p := &mikrotikParser{}
	if got := p.MajorVersionFromFacts(&DeviceFacts{OSVersion: "7.15.3 (stable)"}); got != 7 {
		t.Errorf("v7 major=%d want 7", got)
	}
	if got := p.MajorVersionFromFacts(&DeviceFacts{OSVersion: "6.49.17"}); got != 6 {
		t.Errorf("v6 major=%d want 6", got)
	}
	if got := p.MajorVersionFromFacts(nil); got != 0 {
		t.Errorf("nil major=%d want 0", got)
	}
}
