package inventory

import (
	"os"
	"path/filepath"
	"testing"
)

func loadTestInventory(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "inv.yaml")
	body := `
devices:
  - hostname: leaf-01
    ip: 10.0.0.1
    vendor: Juniper
    platform: junos
  - hostname: rtr-core
    ip: 10.0.0.2
    vendor: cisco
    platform: ios
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}
}

func TestGetDeviceIndex(t *testing.T) {
	loadTestInventory(t)

	// Exact hostname.
	if d, err := GetDevice("leaf-01"); err != nil || d.IP != "10.0.0.1" {
		t.Errorf("hostname lookup: %+v err=%v", d, err)
	}
	// Exact IP.
	if d, err := GetDevice("10.0.0.2"); err != nil || d.Hostname != "rtr-core" {
		t.Errorf("ip lookup: %+v err=%v", d, err)
	}
	// Substring fallback.
	if d, err := GetDevice("leaf"); err != nil || d.Hostname != "leaf-01" {
		t.Errorf("substring lookup: %+v err=%v", d, err)
	}
	// Unknown.
	if _, err := GetDevice("nope-99"); err == nil {
		t.Error("expected error for unknown device")
	}
}

func TestGetDeviceReturnsCopy(t *testing.T) {
	loadTestInventory(t)

	d, err := GetDevice("leaf-01")
	if err != nil {
		t.Fatal(err)
	}
	// Mutating the returned device must not affect the shared inventory (several
	// handlers override device.Vendor).
	d.Vendor = "mutated"

	again, _ := GetDevice("leaf-01")
	if again.Vendor != "juniper" {
		t.Errorf("shared inventory was mutated: vendor=%q", again.Vendor)
	}
}
