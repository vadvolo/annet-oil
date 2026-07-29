package featureset

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"24.4R2.23", "24.4R2.23", 0},
		{"24.4R2.23", "24.4R2", 1},  // longer prefix is greater
		{"24.4R2", "24.4R1", 1},     // R2 > R1
		{"21.4R1", "24.4R2.23", -1}, // 21 < 24
		{"24.4R2", "24.4R2.23", -1}, // shorter prefix is lower
		{"4.31.2F", "4.31.2F", 0},   // Arista EOS
		{"4.31.2F", "4.30.5M", 1},   // 31 > 30
		{"17.9.4a", "17.9.4", 1},    // trailing alpha token
		{"", "24.4R2", -1},          // empty is lowest
		{"", "", 0},
		{"10.3", "10.3(4a)", -1}, // NX-OS style parens
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestVersionInRange(t *testing.T) {
	cases := []struct {
		v, since, until string
		want            bool
	}{
		{"24.4R2.23", "21.4R1", "", true},  // above lower bound
		{"20.1R1", "21.4R1", "", false},    // below lower bound
		{"22.2R1", "21.4R1", "23.0", true}, // inside window
		{"24.0", "21.4R1", "23.0", false},  // above upper bound
		{"", "21.4R1", "23.0", true},       // unknown version passes
	}
	for _, c := range cases {
		if got := versionInRange(c.v, c.since, c.until); got != c.want {
			t.Errorf("versionInRange(%q,%q,%q)=%v want %v", c.v, c.since, c.until, got, c.want)
		}
	}
}

// testKB mirrors the canonical EX4100 PTP case used across the codebase.
func testKB(t *testing.T) *KnowledgeBase {
	t.Helper()
	data := []byte(`
vendors:
  juniper:
    models:
      - match: "EX4100-*"
        family: ex4100
        platform: junos
        features:
          - name: ptp
            category: timing
            since: "21.4R1"
            modes:
              - name: transparent_clock
                support: supported
              - name: boundary_clock
                support: unsupported
`)
	kb, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return kb
}

func TestResolvePTPBoundaryClockUnsupported(t *testing.T) {
	kb := testKB(t)
	fs := kb.Resolve(Query{Vendor: "Juniper", Model: "EX4100-48MP", Version: "24.4R2.23"})

	if !fs.Matched {
		t.Fatalf("expected a match for EX4100-48MP, got warnings: %v", fs.Warnings)
	}
	if fs.Family != "ex4100" {
		t.Errorf("family=%q want ex4100", fs.Family)
	}
	if len(fs.Features) != 1 {
		t.Fatalf("features=%d want 1", len(fs.Features))
	}
	ptp := fs.Features[0]
	if ptp.Support != SupportPartial {
		t.Errorf("ptp support=%q want partial (one mode supported, one not)", ptp.Support)
	}

	modes := map[string]Support{}
	for _, m := range ptp.Modes {
		modes[m.Name] = m.Support
	}
	if modes["transparent_clock"] != SupportSupported {
		t.Errorf("transparent_clock=%q want supported", modes["transparent_clock"])
	}
	if modes["boundary_clock"] != SupportUnsupported {
		t.Errorf("boundary_clock=%q want unsupported", modes["boundary_clock"])
	}
}

func TestResolveFeatureGatedByVersion(t *testing.T) {
	kb := testKB(t)
	// A version below the feature's `since` gates the whole feature out.
	fs := kb.Resolve(Query{Vendor: "juniper", Model: "EX4100-48MP", Version: "20.1R1"})
	if len(fs.Features) != 1 {
		t.Fatalf("features=%d want 1", len(fs.Features))
	}
	if fs.Features[0].Support != SupportUnsupported {
		t.Errorf("ptp on 20.1R1 support=%q want unsupported", fs.Features[0].Support)
	}
	if fs.Features[0].Notes == "" {
		t.Errorf("expected a version-gate note explaining unavailability")
	}
}

func TestResolveUnknownVendorAndModel(t *testing.T) {
	kb := testKB(t)

	fs := kb.Resolve(Query{Vendor: "nokia", Model: "7750"})
	if fs.Matched || len(fs.Warnings) == 0 {
		t.Errorf("unknown vendor should not match and should warn, got %+v", fs)
	}

	fs = kb.Resolve(Query{Vendor: "juniper", Model: "MX960"})
	if fs.Matched || len(fs.Warnings) == 0 {
		t.Errorf("unknown model should not match and should warn, got %+v", fs)
	}
}

func TestResolveFeatureFilter(t *testing.T) {
	kb := testKB(t)
	fs := kb.Resolve(Query{Vendor: "juniper", Model: "EX4100-48MP", Version: "24.4R2.23", Feature: "PTP"})
	if len(fs.Features) != 1 || fs.Features[0].Name != "ptp" {
		t.Fatalf("feature filter should return only ptp, got %+v", fs.Features)
	}

	fs = kb.Resolve(Query{Vendor: "juniper", Model: "EX4100-48MP", Feature: "nonexistent"})
	if len(fs.Features) != 0 || len(fs.Warnings) == 0 {
		t.Errorf("unknown feature filter should return nothing and warn, got %+v", fs)
	}
}
