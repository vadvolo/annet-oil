package inventory

import (
	"fmt"
	"testing"
)

// loadLargeInventory populates the process inventory with n devices directly
// (bypassing YAML) and builds the lookup indexes as Load does.
func loadLargeInventory(n int) {
	inv := &Inventory{
		Devices:    make([]Device, n),
		byHostname: make(map[string]*Device, n),
		byIP:       make(map[string]*Device, n),
	}
	for i := range n {
		inv.Devices[i] = Device{
			Hostname: fmt.Sprintf("device-%05d", i),
			IP:       fmt.Sprintf("10.%d.%d.%d", i>>16&0xff, i>>8&0xff, i&0xff),
			Vendor:   "cisco",
		}
		d := &inv.Devices[i]
		inv.byHostname[d.Hostname] = d
		inv.byIP[d.IP] = d
	}
	inventory = inv
}

// BenchmarkGetDeviceWorstCase looks up the last device (worst case for the old
// triple linear scan) in a 5000-device inventory.
func BenchmarkGetDeviceWorstCase(b *testing.B) {
	loadLargeInventory(5000)
	target := "device-04999"
	b.ReportAllocs()
	for b.Loop() {
		if _, err := GetDevice(target); err != nil {
			b.Fatal(err)
		}
	}
}
