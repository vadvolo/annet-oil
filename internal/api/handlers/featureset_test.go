package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"annet-oil/internal/featureset"
)

func loadTestKB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fs.yaml")
	body := `
vendors:
  juniper:
    models:
      - match: "EX4100-*"
        family: ex4100
        features:
          - name: ptp
            category: timing
            since: "21.4R1"
            modes:
              - name: transparent_clock
                support: supported
              - name: boundary_clock
                support: unsupported
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write kb: %v", err)
	}
	if _, err := featureset.Load(path); err != nil {
		t.Fatalf("load kb: %v", err)
	}
}

func TestFeatureSetHandler_GET(t *testing.T) {
	loadTestKB(t)
	h := NewFeatureSetHandler()

	req := httptest.NewRequest(http.MethodGet,
		"/?vendor=juniper&model=EX4100-48MP&version=24.4R2.23&feature=ptp", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var fs featureset.FeatureSet
	if err := json.Unmarshal(rec.Body.Bytes(), &fs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !fs.Matched || len(fs.Features) != 1 {
		t.Fatalf("expected one matched feature, got %+v", fs)
	}
	modes := map[string]featureset.Support{}
	for _, m := range fs.Features[0].Modes {
		modes[m.Name] = m.Support
	}
	if modes["boundary_clock"] != featureset.SupportUnsupported {
		t.Errorf("boundary_clock=%q want unsupported", modes["boundary_clock"])
	}
}

func TestFeatureSetHandler_MissingFields(t *testing.T) {
	loadTestKB(t)
	h := NewFeatureSetHandler()

	// Missing model → 400.
	req := httptest.NewRequest(http.MethodGet, "/?vendor=juniper", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when model missing, got %d", rec.Code)
	}
}
