package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type Inventory struct {
	Devices []Device `yaml:"devices"`
	// Credentials holds named credential groups. The "default" group is used as
	// the final fallback. Other groups (e.g. "infra-router") are referenced by a
	// device's role field.
	Credentials map[string]DeviceCredentials `yaml:"credentials"`
	// DefaultCredentials is the legacy single default. Deprecated: use the
	// "default" entry of Credentials. Kept for backward compatibility and
	// merged into Credentials["default"] on load.
	DefaultCredentials DeviceCredentials `yaml:"default_credentials"`
}

type Device struct {
	Hostname string `yaml:"hostname"`
	IP       string `yaml:"ip"`
	Port     int    `yaml:"port,omitempty"`
	Vendor   string `yaml:"vendor"`
	Platform string `yaml:"platform"`
	// Role names a credential group (see Inventory.Credentials) tried before the
	// default group at login time.
	Role string `yaml:"role,omitempty"`
	// Credentials, when set on a device, are the most specific credentials and
	// are tried first (an explicit per-device override).
	Credentials DeviceCredentials `yaml:"credentials,omitempty"`
	Description string            `yaml:"description,omitempty"`
	// Aliases are alternative human-friendly names for the device (e.g. a
	// customer or site label). They are informational and also matched by
	// GetDevice when resolving a device by name.
	Aliases []string `yaml:"aliases,omitempty"`
}

const DefaultSSHPort = 22

func (d *Device) GetPort() int {
	if d.Port == 0 {
		return DefaultSSHPort
	}
	return d.Port
}

type DeviceCredentials struct {
	Login    string `yaml:"login"`
	Password string `yaml:"password"`
}

var inventory *Inventory

func Load(path string) (*Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read inventory file: %w", err)
	}

	// Replace environment variables
	content := string(data)
	content = os.ExpandEnv(content)

	var inv Inventory
	if err := yaml.Unmarshal([]byte(content), &inv); err != nil {
		return nil, fmt.Errorf("failed to parse inventory: %w", err)
	}

	if inv.Credentials == nil {
		inv.Credentials = map[string]DeviceCredentials{}
	}
	// Backward compatibility: fold the legacy default_credentials into the
	// "default" credential group when the group isn't explicitly set.
	if _, ok := inv.Credentials["default"]; !ok && inv.DefaultCredentials.Login != "" {
		inv.Credentials["default"] = inv.DefaultCredentials
	}

	// Credentials are resolved lazily per device (see CredentialsFor); we only
	// normalize vendor names here.
	for i := range inv.Devices {
		inv.Devices[i].Vendor = strings.ToLower(inv.Devices[i].Vendor)
	}

	inventory = &inv
	return &inv, nil
}

// CredentialsFor returns the ordered list of credential candidates to try for a
// device: the device's explicit credentials first (if any), then its role
// group, then the "default" group. Entries with an empty login and duplicates
// are removed. Login attempts should try them in order.
func (inv *Inventory) CredentialsFor(d *Device) []DeviceCredentials {
	var out []DeviceCredentials
	add := func(c DeviceCredentials) {
		if c.Login == "" || slices.Contains(out, c) {
			return
		}
		out = append(out, c)
	}

	if d != nil {
		add(d.Credentials)
		if d.Role != "" {
			if c, ok := inv.Credentials[d.Role]; ok {
				add(c)
			}
		}
	}
	add(inv.Credentials["default"])
	return out
}

// CredentialsFor resolves credential candidates for a device against the
// currently loaded inventory. When no inventory is loaded it falls back to the
// device's own explicit credentials.
func CredentialsFor(d *Device) []DeviceCredentials {
	if inventory == nil {
		if d != nil && d.Credentials.Login != "" {
			return []DeviceCredentials{d.Credentials}
		}
		return nil
	}
	return inventory.CredentialsFor(d)
}

// PrimaryCredentials returns the single most-specific credential for a device
// (the first candidate), or an empty credential when none resolve. Used by
// callers that perform a single login rather than a fallback chain.
func PrimaryCredentials(d *Device) DeviceCredentials {
	if cands := CredentialsFor(d); len(cands) > 0 {
		return cands[0]
	}
	return DeviceCredentials{}
}

func GetDevice(hostname string) (*Device, error) {
	if inventory == nil {
		return nil, fmt.Errorf("inventory not loaded")
	}

	// Try exact match first
	for _, device := range inventory.Devices {
		if device.Hostname == hostname {
			return &device, nil
		}
	}

	// Try IP match
	for _, device := range inventory.Devices {
		if device.IP == hostname {
			return &device, nil
		}
	}

	// Try alias match (aliases are human-friendly labels; match case-insensitively).
	for _, device := range inventory.Devices {
		if slices.ContainsFunc(device.Aliases, func(a string) bool {
			return strings.EqualFold(a, hostname)
		}) {
			return &device, nil
		}
	}

	// Try partial match
	for _, device := range inventory.Devices {
		if strings.Contains(device.Hostname, hostname) || strings.Contains(hostname, device.Hostname) {
			return &device, nil
		}
	}

	return nil, fmt.Errorf("device %s not found in inventory", hostname)
}

func GetDeviceOrDefault(hostname string) *Device {
	device, err := GetDevice(hostname)
	if err != nil {
		var def DeviceCredentials
		if inventory != nil {
			def = inventory.Credentials["default"]
		}
		return &Device{
			Hostname:    hostname,
			IP:          hostname,
			Port:        DefaultSSHPort,
			Vendor:      "cisco",
			Platform:    "ios",
			Credentials: def,
		}
	}
	return device
}

func GetInventory() *Inventory {
	return inventory
}

func FilterDevices(vendor, platform, pattern string) []Device {
	if inventory == nil {
		return nil
	}

	var result []Device
	for _, device := range inventory.Devices {
		if vendor != "" && device.Vendor != strings.ToLower(vendor) {
			continue
		}
		if platform != "" && device.Platform != strings.ToLower(platform) {
			continue
		}
		if pattern != "" && !deviceMatchesPattern(pattern, &device) {
			continue
		}
		result = append(result, device)
	}
	return result
}

// deviceMatchesPattern reports whether pattern matches the device's hostname or
// any of its aliases, using the same wildcard/substring rules as matchPattern.
func deviceMatchesPattern(pattern string, d *Device) bool {
	if matchPattern(pattern, d.Hostname) {
		return true
	}
	for _, a := range d.Aliases {
		if matchPattern(pattern, a) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, hostname string) bool {
	if pattern == "" {
		return true
	}
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, hostname)
		return matched
	}
	return strings.Contains(hostname, pattern)
}
