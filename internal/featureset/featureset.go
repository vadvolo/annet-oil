// Package featureset answers capability questions about network devices:
// given a vendor, model and software version it reports which features (and
// feature modes) that platform supports, so an operator — or an AI agent —
// can avoid proposing configuration the hardware cannot run (e.g. PTP Boundary
// Clock on a switch that only implements Transparent Clock).
//
// Data comes from a curated knowledge base (see resources/featuresets.yaml),
// not from the live device, so queries are deterministic and work offline.
package featureset

import (
	"path/filepath"
	"strings"
)

// Support is the level to which a feature or mode is available on a platform.
type Support string

const (
	SupportSupported   Support = "supported"   // fully available
	SupportUnsupported Support = "unsupported" // not available on this platform/version
	SupportPartial     Support = "partial"     // available with limitations (see modes/notes)
	SupportUnknown     Support = "unknown"     // no data
)

// Query identifies the platform to report on. Vendor and Model are required;
// Version is optional (when empty, version gating is skipped and all features
// are reported). Feature, when set, filters the result to a single feature.
type Query struct {
	Vendor  string
	Model   string
	Version string
	Feature string
}

// FeatureSet is the capability report for a queried platform.
type FeatureSet struct {
	Vendor   string    `json:"vendor"`
	Model    string    `json:"model"`
	Version  string    `json:"version,omitempty"`
	Family   string    `json:"family,omitempty"`
	Platform string    `json:"platform,omitempty"`
	Matched  bool      `json:"matched"` // a knowledge-base entry matched vendor+model
	Features []Feature `json:"features"`
	Warnings []string  `json:"warnings,omitempty"`
}

// Feature is a single capability and its per-mode breakdown.
type Feature struct {
	Name     string   `json:"name"`
	Category string   `json:"category,omitempty"`
	Title    string   `json:"title,omitempty"`
	Support  Support  `json:"support"`
	Notes    string   `json:"notes,omitempty"`
	Modes    []Mode   `json:"modes,omitempty"`
	Refs     []string `json:"refs,omitempty"`
}

// Mode is a variant of a feature (e.g. PTP transparent vs boundary clock).
type Mode struct {
	Name    string  `json:"name"`
	Support Support `json:"support"`
	Notes   string  `json:"notes,omitempty"`
}

// Resolve answers a query against the knowledge base kb. It never returns nil.
func (kb *KnowledgeBase) Resolve(q Query) *FeatureSet {
	vendor := strings.ToLower(strings.TrimSpace(q.Vendor))
	fs := &FeatureSet{
		Vendor:   vendor,
		Model:    q.Model,
		Version:  q.Version,
		Features: []Feature{},
	}

	vkb := kb.Vendors[vendor]
	if vkb == nil {
		fs.Warnings = append(fs.Warnings, "no feature data for vendor "+quoteOrEmpty(vendor))
		return fs
	}

	// Accumulate features from every matching model entry, most-specific last so
	// later entries override earlier ones by feature name. A vendor-wide "*"
	// baseline can thus be refined by a model-specific block.
	merged := map[string]*FeatureSpec{}
	var order []string
	for _, m := range vkb.Models {
		if !modelMatches(m.Match, q.Model) {
			continue
		}
		fs.Matched = true
		if m.Family != "" {
			fs.Family = m.Family
		}
		if m.Platform != "" {
			fs.Platform = m.Platform
		}
		for _, spec := range m.Features {
			key := strings.ToLower(spec.Name)
			if _, seen := merged[key]; !seen {
				order = append(order, key)
			}
			merged[key] = spec
		}
	}

	if !fs.Matched {
		fs.Warnings = append(fs.Warnings, "no feature data for model "+quoteOrEmpty(q.Model)+" (vendor "+vendor+")")
		return fs
	}

	filter := strings.ToLower(strings.TrimSpace(q.Feature))
	for _, key := range order {
		spec := merged[key]
		if filter != "" && !featureNameMatches(spec.Name, filter) {
			continue
		}
		fs.Features = append(fs.Features, resolveFeature(spec, q.Version))
	}

	if filter != "" && len(fs.Features) == 0 {
		fs.Warnings = append(fs.Warnings, "no feature named "+quoteOrEmpty(q.Feature)+" for this platform")
	}
	return fs
}

// resolveFeature applies version gating to a spec and produces a reported
// Feature. When the running version is outside the feature's [since,until]
// window the feature is reported unsupported with an explanatory note.
func resolveFeature(spec *FeatureSpec, version string) Feature {
	f := Feature{
		Name:     spec.Name,
		Category: spec.Category,
		Title:    spec.Title,
		Notes:    spec.Notes,
		Refs:     spec.Refs,
	}

	if !versionInRange(version, spec.Since, spec.Until) {
		f.Support = SupportUnsupported
		f.Notes = appendNote(f.Notes, versionGateNote(version, spec.Since, spec.Until))
		// Modes are moot once the whole feature is gated out; report them as
		// unsupported for completeness.
		for _, ms := range spec.Modes {
			f.Modes = append(f.Modes, Mode{Name: ms.Name, Support: SupportUnsupported, Notes: ms.Notes})
		}
		return f
	}

	for _, ms := range spec.Modes {
		mode := Mode{Name: ms.Name, Support: normalizeSupport(ms.Support), Notes: ms.Notes}
		if !versionInRange(version, ms.Since, ms.Until) {
			mode.Support = SupportUnsupported
			mode.Notes = appendNote(mode.Notes, versionGateNote(version, ms.Since, ms.Until))
		}
		f.Modes = append(f.Modes, mode)
	}

	f.Support = deriveSupport(spec.Support, f.Modes)
	return f
}

// deriveSupport resolves a feature's overall support: an explicit value wins;
// otherwise it is inferred from the mode breakdown.
func deriveSupport(explicit Support, modes []Mode) Support {
	if s := normalizeSupport(explicit); s != SupportUnknown {
		return s
	}
	if len(modes) == 0 {
		return SupportUnknown
	}
	var supported, unsupported int
	for _, m := range modes {
		switch m.Support {
		case SupportSupported:
			supported++
		case SupportUnsupported:
			unsupported++
		case SupportPartial:
			supported++
		}
	}
	switch {
	case supported == 0:
		return SupportUnsupported
	case unsupported == 0:
		return SupportSupported
	default:
		return SupportPartial
	}
}

func normalizeSupport(s Support) Support {
	switch Support(strings.ToLower(string(s))) {
	case SupportSupported, "yes", "true":
		return SupportSupported
	case SupportUnsupported, "no", "false":
		return SupportUnsupported
	case SupportPartial, "limited":
		return SupportPartial
	default:
		return SupportUnknown
	}
}

// modelMatches reports whether a model string satisfies a knowledge-base match
// pattern. Matching is case-insensitive; an empty pattern or "*" matches any
// model. The pattern uses shell-glob syntax (filepath.Match).
func modelMatches(pattern, model string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" {
		return true
	}
	p := strings.ToLower(pattern)
	m := strings.ToLower(strings.TrimSpace(model))
	if ok, err := filepath.Match(p, m); err == nil && ok {
		return true
	}
	// Fall back to substring so "EX4100" matches "EX4100-48MP" without a glob.
	return strings.Contains(m, p)
}

// featureNameMatches reports whether a feature name satisfies a lower-cased
// filter: exact match or substring.
func featureNameMatches(name, lowerFilter string) bool {
	n := strings.ToLower(name)
	return n == lowerFilter || strings.Contains(n, lowerFilter)
}

func versionGateNote(version, since, until string) string {
	switch {
	case since != "" && until != "":
		return "not available on " + version + " (requires " + since + " to " + until + ")"
	case since != "":
		return "not available on " + version + " (introduced in " + since + ")"
	case until != "":
		return "not available on " + version + " (removed after " + until + ")"
	default:
		return "not available on " + version
	}
}

func appendNote(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + "; " + add
}

func quoteOrEmpty(s string) string {
	if s == "" {
		return "(empty)"
	}
	return `"` + s + `"`
}
