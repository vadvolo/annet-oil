package featureset

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// KnowledgeBase is the curated capability database, keyed by lower-cased vendor.
type KnowledgeBase struct {
	Vendors map[string]*VendorKB `yaml:"vendors"`
}

// VendorKB holds the model entries for one vendor.
type VendorKB struct {
	// VersionScheme names the versioning convention (e.g. "junos", "eos") for
	// documentation; comparison itself is scheme-agnostic (see version.go).
	VersionScheme string        `yaml:"version_scheme,omitempty"`
	Models        []*ModelEntry `yaml:"models"`
}

// ModelEntry maps a set of models (by glob) to the features they support.
type ModelEntry struct {
	// Match is a case-insensitive shell glob against the model string, e.g.
	// "EX4100-*". "*" (or empty) is a vendor-wide baseline that any model-
	// specific entry can override.
	Match    string         `yaml:"match"`
	Family   string         `yaml:"family,omitempty"`
	Platform string         `yaml:"platform,omitempty"`
	Features []*FeatureSpec `yaml:"features"`
}

// FeatureSpec is the knowledge-base definition of one feature.
type FeatureSpec struct {
	Name     string      `yaml:"name"`
	Category string      `yaml:"category,omitempty"`
	Title    string      `yaml:"title,omitempty"`
	Support  Support     `yaml:"support,omitempty"`
	Since    string      `yaml:"since,omitempty"` // available from this version (inclusive)
	Until    string      `yaml:"until,omitempty"` // available up to this version (inclusive)
	Notes    string      `yaml:"notes,omitempty"`
	Modes    []*ModeSpec `yaml:"modes,omitempty"`
	Refs     []string    `yaml:"refs,omitempty"`
}

// ModeSpec is the knowledge-base definition of one feature mode.
type ModeSpec struct {
	Name    string  `yaml:"name"`
	Support Support `yaml:"support,omitempty"`
	Since   string  `yaml:"since,omitempty"`
	Until   string  `yaml:"until,omitempty"`
	Notes   string  `yaml:"notes,omitempty"`
}

// db is the process-wide knowledge base, populated by Load.
var db *KnowledgeBase

// Load reads and parses the feature-set knowledge base from path, sets it as
// the process-wide database and returns it. Environment variables in the file
// are expanded, mirroring the inventory loader.
func Load(path string) (*KnowledgeBase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read feature-set file: %w", err)
	}

	kb, err := Parse(data)
	if err != nil {
		return nil, err
	}
	db = kb
	return kb, nil
}

// Parse unmarshals a knowledge base from YAML bytes and normalizes vendor keys
// to lower case. It does not touch the process-wide database.
func Parse(data []byte) (*KnowledgeBase, error) {
	content := os.ExpandEnv(string(data))

	var kb KnowledgeBase
	if err := yaml.Unmarshal([]byte(content), &kb); err != nil {
		return nil, fmt.Errorf("failed to parse feature-set file: %w", err)
	}
	if kb.Vendors == nil {
		kb.Vendors = map[string]*VendorKB{}
	}

	// Normalize vendor keys to lower case so lookups are case-insensitive.
	normalized := make(map[string]*VendorKB, len(kb.Vendors))
	for name, v := range kb.Vendors {
		normalized[strings.ToLower(name)] = v
	}
	kb.Vendors = normalized
	return &kb, nil
}

// Get returns the process-wide knowledge base, or nil if none is loaded.
func Get() *KnowledgeBase {
	return db
}

// Resolve answers a query against the process-wide knowledge base. When no
// knowledge base is loaded it returns a FeatureSet carrying a warning rather
// than nil, so callers can always render a result.
func Resolve(q Query) *FeatureSet {
	if db == nil {
		vendor := strings.ToLower(strings.TrimSpace(q.Vendor))
		return &FeatureSet{
			Vendor:   vendor,
			Model:    q.Model,
			Version:  q.Version,
			Features: []Feature{},
			Warnings: []string{"feature-set knowledge base not loaded"},
		}
	}
	return db.Resolve(q)
}
