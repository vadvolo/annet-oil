package opstate

import "testing"

func TestJunosFacts(t *testing.T) {
	raw := `{
      "software-information": [
        {
          "host-name": [{"data": "leaf-01"}],
          "product-model": [{"data": "ex4100-48mp"}],
          "product-name": [{"data": "ex4100-48mp"}],
          "junos-version": [{"data": "24.4R2.23"}]
        }
      ]
    }`
	f, err := parseJunosVersion(raw)
	if err != nil {
		t.Fatal(err)
	}
	if f.Hostname != "leaf-01" || f.Model != "ex4100-48mp" || f.OSVersion != "24.4R2.23" {
		t.Errorf("facts=%+v", f)
	}
}

func TestJunosInterfaces(t *testing.T) {
	raw := `{
      "interface-information": [
        {
          "physical-interface": [
            {
              "name": [{"data": "ge-0/0/0"}],
              "admin-status": [{"data": "up"}],
              "oper-status": [{"data": "up"}],
              "logical-interface": [
                {
                  "name": [{"data": "ge-0/0/0.0"}],
                  "admin-status": [{"data": "up"}],
                  "oper-status": [{"data": "up"}],
                  "address-family": [
                    {
                      "address-family-name": [{"data": "inet"}],
                      "interface-address": [
                        {"ifa-local": [{"data": "10.0.0.1/24"}]}
                      ]
                    }
                  ]
                }
              ]
            },
            {
              "name": [{"data": "ge-0/0/1"}],
              "admin-status": [{"data": "down"}],
              "oper-status": [{"data": "down"}]
            }
          ]
        }
      ]
    }`
	ifs, err := parseJunosInterfacesTerse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ifs) != 3 {
		t.Fatalf("interfaces=%d want 3: %+v", len(ifs), ifs)
	}
	if ifs[0].Name != "ge-0/0/0" || ifs[0].OperStatus != "up" {
		t.Errorf("phys=%+v", ifs[0])
	}
	if ifs[1].Name != "ge-0/0/0.0" || len(ifs[1].IPv4) != 1 || ifs[1].IPv4[0] != "10.0.0.1/24" {
		t.Errorf("logical=%+v", ifs[1])
	}
	if ifs[2].Name != "ge-0/0/1" || ifs[2].AdminStatus != "down" {
		t.Errorf("down iface=%+v", ifs[2])
	}
}

func TestJunosLLDP(t *testing.T) {
	raw := `{
      "lldp-neighbors-information": [
        {
          "lldp-neighbor-information": [
            {
              "lldp-local-port-id": [{"data": "ge-0/0/0"}],
              "lldp-remote-chassis-id": [{"data": "00:11:22:33:44:55"}],
              "lldp-remote-port-description": [{"data": "ge-0/0/2"}],
              "lldp-remote-system-name": [{"data": "spine-01"}]
            }
          ]
        }
      ]
    }`
	n, err := parseJunosLLDP(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(n) != 1 {
		t.Fatalf("neighbors=%d want 1", len(n))
	}
	if n[0].LocalPort != "ge-0/0/0" || n[0].RemoteSystem != "spine-01" || n[0].RemotePort != "ge-0/0/2" {
		t.Errorf("neighbor=%+v", n[0])
	}
	if n[0].RemoteChassis != "00:11:22:33:44:55" {
		t.Errorf("chassis=%q", n[0].RemoteChassis)
	}
}

func TestJunosMACARPRoutes(t *testing.T) {
	mac := parseJunosMACTable(`Vlan    MAC address        Type       Age  Interfaces
v10     00:11:22:33:44:55  Dynamic    0    ge-0/0/1.0
`)
	if len(mac) != 1 || mac[0].VLAN != 10 || mac[0].MAC != "00:11:22:33:44:55" || mac[0].Interface != "ge-0/0/1.0" {
		t.Errorf("mac=%+v", mac)
	}

	arp := parseJunosARP(`MAC Address       Address         Interface     Flags
00:11:22:33:44:55 10.0.0.2        ge-0/0/0.0    none
`)
	if len(arp) != 1 || arp[0].IP != "10.0.0.2" || arp[0].MAC != "00:11:22:33:44:55" || arp[0].Interface != "ge-0/0/0.0" {
		t.Errorf("arp=%+v", arp)
	}

	routes := parseJunosRoutesTerse(`inet.0: 2 destinations, 2 routes (2 active, 0 holddown, 0 hidden)

A V Destination        P Prf   Metric 1   Metric 2  Next hop        AS path
* ? 10.0.0.0/24        D   0                          >ge-0/0/0.0
* ? 10.1.0.0/24        O  10        2                 >10.0.0.2
`)
	if len(routes) != 2 {
		t.Fatalf("routes=%d want 2: %+v", len(routes), routes)
	}
	if routes[0].Prefix != "10.0.0.0/24" || routes[0].Protocol != "direct" || routes[0].Interface != "ge-0/0/0.0" {
		t.Errorf("route0=%+v", routes[0])
	}
	if routes[1].Prefix != "10.1.0.0/24" || routes[1].Protocol != "ospf" || routes[1].NextHop != "10.0.0.2" {
		t.Errorf("route1=%+v", routes[1])
	}
}
