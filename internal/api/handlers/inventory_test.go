package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"annet-oil/internal/inventory"
)

func writeInventoryFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write inventory: %v", err)
	}
	return path
}

func TestReloadInventory(t *testing.T) {
	dir := t.TempDir()

	// Initial inventory: one device.
	path := writeInventoryFile(t, dir, "inv.yaml", `
devices:
  - hostname: r1
    ip: 10.0.0.1
    vendor: cisco
    platform: ios
`)
	if _, err := inventory.Load(path); err != nil {
		t.Fatalf("initial load: %v", err)
	}

	h := NewInventoryHandler(path)

	// Overwrite the file with two devices, then reload via the endpoint.
	writeInventoryFile(t, dir, "inv.yaml", `
devices:
  - hostname: r1
    ip: 10.0.0.1
    vendor: cisco
    platform: ios
  - hostname: r2
    ip: 10.0.0.2
    vendor: juniper
    platform: junos
`)

	req := httptest.NewRequest(http.MethodPost, "/reload", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ReloadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %q", resp.Status)
	}
	if resp.Devices != 2 {
		t.Errorf("expected 2 devices after reload, got %d", resp.Devices)
	}

	// The in-memory inventory should now resolve the newly added device.
	if _, err := inventory.GetDevice("r2"); err != nil {
		t.Errorf("expected r2 to be resolvable after reload: %v", err)
	}
}

func TestListInventoryIncludesAliases(t *testing.T) {
	dir := t.TempDir()
	path := writeInventoryFile(t, dir, "inv.yaml", `
devices:
  - hostname: rb750
    ip: 10.0.0.5
    vendor: ros
    platform: routeros
    aliases:
      - "Client: Pijaca-Karaburma-RB750"
      - "pijaca"
  - hostname: r2
    ip: 10.0.0.2
    vendor: cisco
    platform: ios
`)
	if _, err := inventory.Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	listInventory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp InventoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	byHost := map[string]DeviceInfo{}
	for _, d := range resp.Devices {
		byHost[d.Hostname] = d
	}
	if got := byHost["rb750"].Aliases; len(got) != 2 || got[0] != "Client: Pijaca-Karaburma-RB750" || got[1] != "pijaca" {
		t.Errorf("rb750 aliases=%v", got)
	}
	// A device without aliases should omit the field entirely in JSON.
	if len(byHost["r2"].Aliases) != 0 {
		t.Errorf("r2 should have no aliases, got %v", byHost["r2"].Aliases)
	}
	if strings.Contains(rec.Body.String(), `"aliases":[]`) {
		t.Errorf("empty aliases should be omitted, body=%s", rec.Body.String())
	}
}

func TestReloadInventory_NoFileConfigured(t *testing.T) {
	h := NewInventoryHandler("")
	req := httptest.NewRequest(http.MethodPost, "/reload", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when no inventory file configured, got %d", rec.Code)
	}
}
