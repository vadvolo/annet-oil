package featureset

import (
	"path/filepath"
	"testing"
)

// kbPath is the shipped knowledge base, relative to this package directory.
const kbPath = "../../resources/featuresets.yaml"

// TestShippedKnowledgeBaseLoads guards the committed resources/featuresets.yaml:
// it must parse, and the canonical EX4100 PTP case (Transparent supported,
// Boundary Clock not) must hold — this is the example the whole feature exists
// to serve.
func TestShippedKnowledgeBaseLoads(t *testing.T) {
	kb, err := Load(filepath.Clean(kbPath))
	if err != nil {
		t.Fatalf("shipped knowledge base failed to load: %v", err)
	}
	if len(kb.Vendors) == 0 {
		t.Fatal("shipped knowledge base has no vendors")
	}

	fs := kb.Resolve(Query{Vendor: "juniper", Model: "EX4100-48MP", Version: "24.4R2.23", Feature: "ptp"})
	if len(fs.Features) != 1 {
		t.Fatalf("EX4100 ptp: features=%d want 1 (warnings: %v)", len(fs.Features), fs.Warnings)
	}
	modes := map[string]Support{}
	for _, m := range fs.Features[0].Modes {
		modes[m.Name] = m.Support
	}
	if modes["transparent_clock"] != SupportSupported {
		t.Errorf("EX4100 transparent_clock=%q want supported", modes["transparent_clock"])
	}
	if modes["boundary_clock"] != SupportUnsupported {
		t.Errorf("EX4100 boundary_clock=%q want unsupported", modes["boundary_clock"])
	}

	// A DC switch that does support Boundary Clock, for contrast.
	fs = kb.Resolve(Query{Vendor: "juniper", Model: "QFX5120-48Y", Version: "23.2R1", Feature: "ptp"})
	if len(fs.Features) != 1 {
		t.Fatalf("QFX5120 ptp: features=%d want 1 (warnings: %v)", len(fs.Features), fs.Warnings)
	}
	bc := SupportUnknown
	for _, m := range fs.Features[0].Modes {
		if m.Name == "boundary_clock" {
			bc = m.Support
		}
	}
	if bc != SupportSupported {
		t.Errorf("QFX5120 boundary_clock=%q want supported", bc)
	}
}
