package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Inventory struct {
	Devices            []Device          `yaml:"devices"`
	DefaultCredentials DeviceCredentials `yaml:"default_credentials"`
}

type Device struct {
	Hostname    string            `yaml:"hostname"`
	IP          string            `yaml:"ip"`
	Port        int               `yaml:"port,omitempty"`
	Vendor      string            `yaml:"vendor"`
	Platform    string            `yaml:"platform"`
	Credentials DeviceCredentials `yaml:"credentials"`
	Description string            `yaml:"description,omitempty"`
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

	// Apply default credentials where needed
	for i := range inv.Devices {
		if inv.Devices[i].Credentials.Login == "" {
			inv.Devices[i].Credentials = inv.DefaultCredentials
		}
		// Normalize vendor names
		inv.Devices[i].Vendor = strings.ToLower(inv.Devices[i].Vendor)
	}

	inventory = &inv
	return &inv, nil
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
		return &Device{
			Hostname:    hostname,
			IP:          hostname,
			Port:        DefaultSSHPort,
			Vendor:      "cisco",
			Platform:    "ios",
			Credentials: inventory.DefaultCredentials,
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
		if pattern != "" && !matchPattern(pattern, device.Hostname) {
			continue
		}
		result = append(result, device)
	}
	return result
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
