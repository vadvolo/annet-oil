package opstate

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// buildCiscoBrief builds a "show ip interface brief" with n interfaces.
func buildCiscoBrief(n int) string {
	var b strings.Builder
	b.WriteString("Interface              IP-Address      OK? Method Status                Protocol\n")
	for i := range n {
		fmt.Fprintf(&b, "GigabitEthernet0/%d     10.0.%d.1        YES manual up                    up\n", i, i)
	}
	return b.String()
}

// buildJunosInterfacesJSON builds a Junos terse interfaces JSON with n physical
// interfaces, each with one logical unit carrying an address.
func buildJunosInterfacesJSON(n int) string {
	var phys []string
	for i := range n {
		phys = append(phys, fmt.Sprintf(`{
      "name":[{"data":"ge-0/0/%d"}],
      "admin-status":[{"data":"up"}],
      "oper-status":[{"data":"up"}],
      "logical-interface":[
        {"name":[{"data":"ge-0/0/%d.0"}],
         "admin-status":[{"data":"up"}],"oper-status":[{"data":"up"}],
         "address-family":[{"address-family-name":[{"data":"inet"}],
           "interface-address":[{"ifa-local":[{"data":"10.0.%d.1/24"}]}]}]}
      ]}`, i, i, i))
	}
	return `{"interface-information":[{"physical-interface":[` + strings.Join(phys, ",") + `]}]}`
}

func BenchmarkCiscoInterfaces(b *testing.B) {
	raw := buildCiscoBrief(48)
	b.ReportAllocs()
	for b.Loop() {
		_ = parseCiscoIPIntBrief(raw)
	}
}

func BenchmarkJunosInterfaces(b *testing.B) {
	raw := buildJunosInterfacesJSON(48)
	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseJunosInterfacesTerse(raw)
	}
}

func BenchmarkCiscoVersion(b *testing.B) {
	raw := `Cisco IOS Software, C3560 Software (C3560-IPSERVICESK9-M), Version 12.2(55)SE9, RELEASE SOFTWARE (fc1)
switch1 uptime is 1 year, 20 weeks, 3 days
cisco WS-C3560-48TS (PowerPC) processor (revision H0) with 131072K bytes of memory.
Processor board ID CAT1234ABCD
`
	b.ReportAllocs()
	for b.Loop() {
		_ = parseCiscoVersion(raw)
	}
}

// BenchmarkCollectCacheHit measures the cached path (JSON round-trip on get).
func BenchmarkCollectCacheHit(b *testing.B) {
	fx := &fakeExec{responses: map[string]string{
		"interface": buildCiscoBrief(48),
	}}
	c := NewCollector(fx, NewCache(time.Hour))
	ctx := context.Background()
	// Prime the cache.
	_, _ = c.Collect(ctx, "r1", "cisco", "", CollectOptions{States: []StateType{Interfaces}})
	b.ReportAllocs()
	for b.Loop() {
		_, _ = c.Collect(ctx, "r1", "cisco", "", CollectOptions{States: []StateType{Interfaces}})
	}
}
