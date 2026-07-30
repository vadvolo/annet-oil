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
