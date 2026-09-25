package diag

import (
	"strings"

	"annet-oil/internal/diag/diagpb"
)

// Family is the diagnostic device family used to select a playbook. It is a thin
// alias over the generated proto enum with resolution + naming helpers.
type Family = diagpb.Family

// ResolveFamily maps an inventory (vendor, platform) pair to a diagnostic family.
// vendor is the gnetcli device profile ("cisco", "eltex", "ros", "fsos", ...);
// platform disambiguates cisco IOS vs IOS-XR ("iosxr" vs "ios"/"iosxe"/"nxos").
func ResolveFamily(vendor, platform string) diagpb.Family {
	v := strings.ToLower(strings.TrimSpace(vendor))
	p := strings.ToLower(strings.TrimSpace(platform))

	switch v {
	case "cisco":
		if strings.Contains(p, "xr") {
			return diagpb.Family_FAMILY_IOSXR
		}
		return diagpb.Family_FAMILY_IOS
	case "eltex":
		return diagpb.Family_FAMILY_ELTEX
	case "ros", "routeros", "mikrotik":
		return diagpb.Family_FAMILY_ROUTEROS
	case "fsos", "fs", "ruijie":
		return diagpb.Family_FAMILY_FS
	case "apc", "ups":
		return diagpb.Family_FAMILY_UPS
	}

	// Fall back to platform hints when the vendor string is unusual.
	switch {
	case strings.Contains(p, "xr"):
		return diagpb.Family_FAMILY_IOSXR
	case p == "ios" || p == "iosxe" || p == "nxos":
		return diagpb.Family_FAMILY_IOS
	case p == "mes":
		return diagpb.Family_FAMILY_ELTEX
	case p == "routeros":
		return diagpb.Family_FAMILY_ROUTEROS
	case p == "fsos":
		return diagpb.Family_FAMILY_FS
	}

	return diagpb.Family_FAMILY_GENERIC_CORE
}

// FamilyName returns the short lowercase name for a family.
func FamilyName(f diagpb.Family) string {
	switch f {
	case diagpb.Family_FAMILY_IOSXR:
		return "iosxr"
	case diagpb.Family_FAMILY_IOS:
		return "ios"
	case diagpb.Family_FAMILY_ELTEX:
		return "eltex"
	case diagpb.Family_FAMILY_ROUTEROS:
		return "routeros"
	case diagpb.Family_FAMILY_FS:
		return "fs"
	case diagpb.Family_FAMILY_UPS:
		return "ups"
	case diagpb.Family_FAMILY_GENERIC_CORE:
		return "generic_core"
	default:
		return "unspecified"
	}
}
