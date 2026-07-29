package opstate

import "testing"

func TestCiscoFacts(t *testing.T) {
	raw := `Cisco IOS Software, C3560 Software (C3560-IPSERVICESK9-M), Version 12.2(55)SE9, RELEASE SOFTWARE (fc1)
ROM: Bootstrap program is C3560 boot loader
switch1 uptime is 1 year, 20 weeks, 3 days
cisco WS-C3560-48TS (PowerPC) processor (revision H0) with 131072K bytes of memory.
Processor board ID CAT1234ABCD
`
	f := parseCiscoVersion(raw)
	if f.OSVersion != "12.2(55)SE9" {
		t.Errorf("os_version=%q", f.OSVersion)
	}
	if f.Model != "WS-C3560-48TS" {
		t.Errorf("model=%q", f.Model)
	}
	if f.Serial != "CAT1234ABCD" {
		t.Errorf("serial=%q", f.Serial)
	}
	if f.Hostname != "switch1" {
		t.Errorf("hostname=%q", f.Hostname)
	}
}

func TestCiscoInterfaces(t *testing.T) {
	raw := `Interface              IP-Address      OK? Method Status                Protocol
GigabitEthernet0/0     10.0.0.1        YES manual up                    up
GigabitEthernet0/1     unassigned      YES unset  administratively down down
Vlan1                  192.168.1.1     YES manual up                    up
`
	ifs := parseCiscoIPIntBrief(raw)
	if len(ifs) != 3 {
		t.Fatalf("interfaces=%d want 3", len(ifs))
	}
	if ifs[0].Name != "GigabitEthernet0/0" || ifs[0].OperStatus != "up" || ifs[0].AdminStatus != "up" {
		t.Errorf("if0=%+v", ifs[0])
	}
	if len(ifs[0].IPv4) != 1 || ifs[0].IPv4[0] != "10.0.0.1" {
		t.Errorf("if0 ipv4=%v", ifs[0].IPv4)
	}
	if ifs[1].AdminStatus != "down" || ifs[1].OperStatus != "down" {
		t.Errorf("if1 status=%+v", ifs[1])
	}
	if len(ifs[1].IPv4) != 0 {
		t.Errorf("if1 should have no ip, got %v", ifs[1].IPv4)
	}
}

func TestCiscoLLDP(t *testing.T) {
	raw := `------------------------------------------------
Local Intf: Gi0/1
Chassis id: aabb.cc00.1122
Port id: Gi0/2
Port Description: Uplink
System Name: switch2

Management Addresses:
    IP: 10.0.0.2
`
	n := parseCiscoLLDPDetail(raw)
	if len(n) != 1 {
		t.Fatalf("neighbors=%d want 1", len(n))
	}
	got := n[0]
	if got.LocalPort != "Gi0/1" || got.RemotePort != "Gi0/2" || got.RemoteSystem != "switch2" {
		t.Errorf("neighbor=%+v", got)
	}
	if got.RemoteChassis != "aabb.cc00.1122" || got.RemoteMgmtIP != "10.0.0.2" {
		t.Errorf("chassis/mgmt=%+v", got)
	}
}

func TestCiscoMACAndARP(t *testing.T) {
	mac := parseCiscoMACTable(`Vlan    Mac Address       Type        Ports
----    -----------       --------    -----
  10    aabb.cc00.1100    DYNAMIC     Gi0/1
 200    aabb.cc00.2200    STATIC      Gi0/2
`)
	if len(mac) != 2 || mac[0].VLAN != 10 || mac[0].Type != "dynamic" || mac[0].Interface != "Gi0/1" {
		t.Errorf("mac=%+v", mac)
	}

	arp := parseCiscoARP(`Protocol  Address          Age (min)  Hardware Addr   Type   Interface
Internet  10.0.0.1                -   aabb.cc00.0100  ARPA   GigabitEthernet0/0
Internet  10.0.0.2               12   aabb.cc00.0200  ARPA   GigabitEthernet0/0
`)
	if len(arp) != 2 || arp[0].IP != "10.0.0.1" || arp[0].MAC != "aabb.cc00.0100" || arp[0].Interface != "GigabitEthernet0/0" {
		t.Errorf("arp=%+v", arp)
	}
}

func TestCiscoRoutes(t *testing.T) {
	routes := parseCiscoRoutes(`Gateway of last resort is 10.0.0.254 to network 0.0.0.0

C        10.0.0.0/24 is directly connected, GigabitEthernet0/0
O        10.1.0.0/24 [110/2] via 10.0.0.2, 00:10:00, GigabitEthernet0/0
`)
	if len(routes) != 2 {
		t.Fatalf("routes=%d want 2: %+v", len(routes), routes)
	}
	if routes[0].Prefix != "10.0.0.0/24" || routes[0].Protocol != "connected" || routes[0].Interface != "GigabitEthernet0/0" {
		t.Errorf("route0=%+v", routes[0])
	}
	if routes[1].Prefix != "10.1.0.0/24" || routes[1].Protocol != "ospf" || routes[1].NextHop != "10.0.0.2" {
		t.Errorf("route1=%+v", routes[1])
	}
}
