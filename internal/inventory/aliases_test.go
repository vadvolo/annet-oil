package inventory

import "testing"

func TestAliasesParsedAndResolvable(t *testing.T) {
	loadInv(t, `
devices:
  - hostname: rb750
    ip: 10.0.0.5
    vendor: ros
    platform: routeros
    aliases:
      - "Client: Pijaca-Karaburma-RB750"
      - "pijaca"
`)

	d, err := GetDevice("rb750")
	if err != nil {
		t.Fatalf("get by hostname: %v", err)
	}
	if len(d.Aliases) != 2 || d.Aliases[0] != "Client: Pijaca-Karaburma-RB750" {
		t.Fatalf("aliases not parsed: %v", d.Aliases)
	}

	// Exact alias, case-insensitive.
	if got, err := GetDevice("client: pijaca-karaburma-rb750"); err != nil || got.Hostname != "rb750" {
		t.Errorf("resolve by alias failed: dev=%v err=%v", got, err)
	}
	if got, err := GetDevice("pijaca"); err != nil || got.Hostname != "rb750" {
		t.Errorf("resolve by short alias failed: dev=%v err=%v", got, err)
	}
}

func TestFilterDevicesByAlias(t *testing.T) {
	loadInv(t, `
devices:
  - hostname: rb750
    ip: 10.0.0.5
    vendor: ros
    platform: routeros
    aliases:
      - "Client: Pijaca-Karaburma-RB750"
  - hostname: r2
    ip: 10.0.0.2
    vendor: cisco
    platform: ios
`)

	// Substring pattern matching an alias resolves the device by alias.
	got := FilterDevices("", "", "Pijaca")
	if len(got) != 1 || got[0].Hostname != "rb750" {
		t.Fatalf("filter by alias substring=%v", got)
	}

	// Wildcard pattern against an alias.
	if got := FilterDevices("", "", "Client:*"); len(got) != 1 || got[0].Hostname != "rb750" {
		t.Errorf("filter by alias wildcard=%v", got)
	}

	// Hostname matching still works and does not spuriously match the alias device.
	if got := FilterDevices("", "", "r2"); len(got) != 1 || got[0].Hostname != "r2" {
		t.Errorf("filter by hostname=%v", got)
	}

	// Vendor filter still composes with the alias match.
	if got := FilterDevices("cisco", "", "Pijaca"); len(got) != 0 {
		t.Errorf("vendor filter should exclude alias match=%v", got)
	}
}
