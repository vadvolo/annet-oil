package opstate

import (
	"bufio"
	"fmt"
	"sort"
	"strings"
)

// maxLineBuffer bounds a single scanned line. Device tables are line-oriented
// with short lines, but the default bufio.Scanner limit (64 KB) would silently
// truncate the rest of the output on a single pathological long line; 4 MB is a
// safe ceiling.
const maxLineBuffer = 4 * 1024 * 1024

// lineScanner returns a line scanner over raw with an enlarged token buffer.
func lineScanner(raw string) *bufio.Scanner {
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBuffer)
	return sc
}

// Parser turns raw device command output into normalized operational state for
// one vendor. Implementations are pure (no I/O): the collector runs Command(t)
// via an Executor and hands the raw output to Parse(t, raw, dst). This keeps
// parsers fully unit-testable from fixtures.
type Parser interface {
	// Vendor returns the canonical vendor name this parser handles.
	Vendor() string
	// Command returns the CLI command that collects state type t, and false when
	// the vendor/parser does not support that state type.
	Command(t StateType) (string, bool)
	// Parse fills the section of dst for state type t from raw command output.
	Parse(t StateType, raw string, dst *State) error
}

// registry maps canonical vendor -> parser factory.
var registry = map[string]func() Parser{
	"juniper": func() Parser { return &juniperParser{} },
	"cisco":   func() Parser { return &ciscoParser{} },
}

// vendorAliases maps common vendor/platform spellings to a canonical vendor.
var vendorAliases = map[string]string{
	"juniper": "juniper",
	"junos":   "juniper",
	"cisco":   "cisco",
	"ios":     "cisco",
	"ios-xe":  "cisco",
	"iosxe":   "cisco",
	"ios-xr":  "cisco",
	"iosxr":   "cisco",
}

// CanonicalVendor maps a vendor or platform string to the canonical vendor key,
// or "" when unknown.
func CanonicalVendor(vendor string) string {
	return vendorAliases[strings.ToLower(strings.TrimSpace(vendor))]
}

// ParserFor returns a parser for the given vendor or platform string. It accepts
// aliases (e.g. "junos" -> juniper, "ios-xe" -> cisco).
func ParserFor(vendor string) (Parser, error) {
	canon := CanonicalVendor(vendor)
	if canon == "" {
		return nil, fmt.Errorf("no operational-state parser for vendor %q", vendor)
	}
	factory, ok := registry[canon]
	if !ok {
		return nil, fmt.Errorf("no operational-state parser for vendor %q", vendor)
	}
	return factory(), nil
}

// SupportedVendors lists the canonical vendors with a parser, sorted.
func SupportedVendors() []string {
	out := make([]string, 0, len(registry))
	for v := range registry {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// ParseStateTypes converts strings to StateTypes, validating each against the
// known set. "all" expands to AllStates. Returns the deduped list in a stable
// order, or an error naming the first unknown value.
func ParseStateTypes(names []string) ([]StateType, error) {
	if len(names) == 0 {
		return nil, nil
	}
	valid := map[StateType]bool{}
	for _, t := range AllStates {
		valid[t] = true
	}
	seen := map[StateType]bool{}
	var out []StateType
	add := func(t StateType) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, n := range names {
		s := StateType(strings.ToLower(strings.TrimSpace(n)))
		if s == "all" {
			for _, t := range AllStates {
				add(t)
			}
			continue
		}
		if !valid[s] {
			return nil, fmt.Errorf("unknown state type %q (valid: %s, all)", n, joinStates(AllStates))
		}
		add(s)
	}
	return out, nil
}

func joinStates(ts []StateType) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = string(t)
	}
	return strings.Join(parts, ", ")
}

// normStatus normalizes admin/oper status text to "up"/"down"/"" .
func normStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case s == "":
		return ""
	case strings.HasPrefix(s, "up") || s == "connected" || s == "active":
		return "up"
	case strings.HasPrefix(s, "down") || s == "notconnect" || s == "admin-down" || s == "disabled":
		return "down"
	default:
		return s
	}
}
