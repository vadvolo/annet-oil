package opstate

import "testing"

func TestEltexFacts(t *testing.T) {
	raw := `Active-image: flash://system/images/mes5324.ros
  SW version    4.0.15 (date 12-Feb-2021 time 10:00:00)
  Boot version  1.1.1.6
System type:   Eltex MES5324
`
	f := parseEltexVersion(raw)
	if f.OSVersion != "4.0.15" {
		t.Errorf("os_version=%q", f.OSVersion)
	}
	if f.Model != "MES5324" {
		t.Errorf("model=%q", f.Model)
	}
}

func TestEltexInterfaces(t *testing.T) {
	raw := `                                          Flow    Link        Back   Mdix
Port     Type         Duplex  Speed  Neg    control State       Pressure Mode
-------- ------------ ------  -----  ------ ------- ----------- -------- -------
gi1/0/1  1G-Copper    Full    1000   Enabled Off    Up          Disabled On
gi1/0/2  1G-Copper    --      --     --      --      Down        --       --
`
	ifs := parseEltexIfStatus(raw)
	if len(ifs) != 2 {
		t.Fatalf("interfaces=%d want 2", len(ifs))
	}
	if ifs[0].Name != "gi1/0/1" || ifs[0].OperStatus != "up" || ifs[0].SpeedMbps != 1000 {
		t.Errorf("if0=%+v", ifs[0])
	}
	if ifs[1].Name != "gi1/0/2" || ifs[1].OperStatus != "down" {
		t.Errorf("if1=%+v", ifs[1])
	}
}

func TestEltexLLDP(t *testing.T) {
	raw := `Port       Device ID          Port ID   System Name   Capabilities  TTL
---------- ------------------ --------- ------------- ------------- ----
gi1/0/1    aa:bb:cc:dd:ee:ff  gi0/2     switch2       B             120
`
	n := parseEltexLLDP(raw)
	if len(n) != 1 {
		t.Fatalf("neighbors=%d want 1", len(n))
	}
	got := n[0]
	if got.LocalPort != "gi1/0/1" || got.RemoteChassis != "aa:bb:cc:dd:ee:ff" || got.RemotePort != "gi0/2" || got.RemoteSystem != "switch2" {
		t.Errorf("neighbor=%+v", got)
	}
}

func TestEltexMACAndARP(t *testing.T) {
	mac := parseEltexMACTable(`Vlan          Mac Address         Port          Type
------------ ------------------- ------------- -----------
1            00:11:22:33:44:55   gi1/0/1       dynamic
200          00:11:22:33:44:66   gi1/0/2       static
`)
	if len(mac) != 2 {
		t.Fatalf("mac=%d want 2", len(mac))
	}
	if mac[0].VLAN != 1 || mac[0].MAC != "00:11:22:33:44:55" || mac[0].Interface != "gi1/0/1" || mac[0].Type != "dynamic" {
		t.Errorf("mac0=%+v", mac[0])
	}

	arp := parseEltexARP(`VLAN       Interface     IP address        HW address           Status
---------- ------------- ----------------- -------------------- ----------
vlan1      gi1/0/1       10.0.0.2          00:11:22:33:44:55    dynamic
`)
	if len(arp) != 1 || arp[0].IP != "10.0.0.2" || arp[0].MAC != "00:11:22:33:44:55" || arp[0].Interface != "gi1/0/1" {
		t.Errorf("arp=%+v", arp)
	}
}

func TestEltexRoutesReuseCisco(t *testing.T) {
	routes := parseCiscoRoutes(`C   10.0.0.0/24 is directly connected, vlan1
S   0.0.0.0/0 [1/0] via 10.0.0.254
`)
	if len(routes) != 2 {
		t.Fatalf("routes=%d want 2: %+v", len(routes), routes)
	}
	if routes[0].Prefix != "10.0.0.0/24" || routes[0].Protocol != "connected" || routes[0].Interface != "vlan1" {
		t.Errorf("route0=%+v", routes[0])
	}
	if routes[1].Prefix != "0.0.0.0/0" || routes[1].Protocol != "static" || routes[1].NextHop != "10.0.0.254" {
		t.Errorf("route1=%+v", routes[1])
	}
}
