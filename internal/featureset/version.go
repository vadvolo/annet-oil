package featureset

import (
	"strconv"
	"strings"
)

// Version comparison across network-OS versioning schemes (Junos "24.4R2.23",
// Arista EOS "4.31.2F", Cisco IOS-XE "17.9.4a", NX-OS "10.3(4a)", ...) is done
// with a best-effort tokenizer rather than a per-vendor parser.
//
// A version string is split into an ordered list of tokens, where each maximal
// run of digits becomes a numeric token and each maximal run of letters becomes
// an alphabetic token; every other character (".", "-", "_", "(", ")", space)
// is a separator and is discarded. Tokens are then compared position by
// position: numbers numerically, letters case-insensitively, and when the kinds
// differ a numeric token sorts before an alphabetic one. A shorter version that
// is a prefix of a longer one sorts lower (so 24.4R2 < 24.4R2.23).
//
// This is only meaningful within a single vendor's scheme; the caller compares
// a device version against bounds written in the same scheme (a feature's
// `since`/`until`), never across vendors.

type tokenKind int

const (
	tokNumeric tokenKind = iota
	tokAlpha
)

type versionToken struct {
	kind tokenKind
	num  int64  // valid when kind == tokNumeric
	str  string // valid when kind == tokAlpha (upper-cased)
}

// tokenizeVersion splits a version string into comparable tokens. Returns nil
// for an empty/whitespace-only string.
func tokenizeVersion(v string) []versionToken {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}

	var tokens []versionToken
	var buf strings.Builder
	var bufKind tokenKind
	haveBuf := false

	flush := func() {
		if !haveBuf {
			return
		}
		s := buf.String()
		if bufKind == tokNumeric {
			n, _ := strconv.ParseInt(s, 10, 64)
			tokens = append(tokens, versionToken{kind: tokNumeric, num: n})
		} else {
			tokens = append(tokens, versionToken{kind: tokAlpha, str: strings.ToUpper(s)})
		}
		buf.Reset()
		haveBuf = false
	}

	for _, r := range v {
		switch {
		case r >= '0' && r <= '9':
			if haveBuf && bufKind != tokNumeric {
				flush()
			}
			bufKind = tokNumeric
			haveBuf = true
			buf.WriteRune(r)
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			if haveBuf && bufKind != tokAlpha {
				flush()
			}
			bufKind = tokAlpha
			haveBuf = true
			buf.WriteRune(r)
		default:
			// separator
			flush()
		}
	}
	flush()
	return tokens
}

// CompareVersions reports whether a is less than (-1), equal to (0) or greater
// than (+1) b, using the tokenizer described above. Empty versions compare as
// lower than any non-empty version (two empties are equal).
func CompareVersions(a, b string) int {
	ta := tokenizeVersion(a)
	tb := tokenizeVersion(b)

	n := min(len(ta), len(tb))

	for i := range n {
		if c := compareToken(ta[i], tb[i]); c != 0 {
			return c
		}
	}

	switch {
	case len(ta) < len(tb):
		return -1
	case len(ta) > len(tb):
		return 1
	default:
		return 0
	}
}

func compareToken(a, b versionToken) int {
	if a.kind != b.kind {
		// Numeric sorts before alphabetic when kinds differ.
		if a.kind == tokNumeric {
			return -1
		}
		return 1
	}
	if a.kind == tokNumeric {
		switch {
		case a.num < b.num:
			return -1
		case a.num > b.num:
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(a.str, b.str)
}

// versionInRange reports whether version v satisfies the inclusive bounds
// [since, until]. An empty bound is unbounded on that side; an empty v (unknown
// device version) satisfies any range so features are not hidden when the
// version is unknown.
func versionInRange(v, since, until string) bool {
	if strings.TrimSpace(v) == "" {
		return true
	}
	if since != "" && CompareVersions(v, since) < 0 {
		return false
	}
	if until != "" && CompareVersions(v, until) > 0 {
		return false
	}
	return true
}
