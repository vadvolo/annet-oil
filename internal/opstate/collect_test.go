package opstate

import (
	"context"
	"strings"
	"testing"
	"time"
)

// fakeExec returns canned output per command and counts invocations.
type fakeExec struct {
	responses map[string]string
	calls     int
}

func (f *fakeExec) Exec(_ context.Context, _ string, cmd string) (string, error) {
	f.calls++
	for substr, out := range f.responses {
		if strings.Contains(cmd, substr) {
			return out, nil
		}
	}
	return "", nil
}

func TestCollectCiscoDefault(t *testing.T) {
	fx := &fakeExec{responses: map[string]string{
		"show version": "cisco IOS Software, Version 15.2(4)E, RELEASE\nrtr1 uptime is 5 days\ncisco WS-C2960 (PowerPC) processor\n",
		"interface":    "Interface  IP-Address  OK? Method Status  Protocol\nGi0/1  10.0.0.1  YES manual up  up\n",
		"lldp":         "Local Intf: Gi0/1\nSystem Name: peer1\nPort id: Gi0/2\n",
	}}
	c := NewCollector(fx, nil)

	st, err := c.Collect(context.Background(), "rtr1", "cisco", "ios", CollectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Vendor != "cisco" {
		t.Errorf("vendor=%q", st.Vendor)
	}
	if st.Facts == nil || st.Facts.Hostname != "rtr1" {
		t.Errorf("facts=%+v", st.Facts)
	}
	if len(st.Interfaces) != 1 || st.Interfaces[0].Name != "Gi0/1" {
		t.Errorf("interfaces=%+v", st.Interfaces)
	}
	if len(st.LLDP) != 1 || st.LLDP[0].RemoteSystem != "peer1" {
		t.Errorf("lldp=%+v", st.LLDP)
	}
	// Default set is facts, interfaces, lldp — three device calls.
	if fx.calls != 3 {
		t.Errorf("exec calls=%d want 3", fx.calls)
	}
}

func TestCollectCacheAndForce(t *testing.T) {
	fx := &fakeExec{responses: map[string]string{
		"show version": "cisco IOS Software, Version 15.2(4)E\nrtr1 uptime is 5 days\n",
	}}
	c := NewCollector(fx, NewCache(time.Minute))

	// First call: miss -> one exec.
	st1, _ := c.Collect(context.Background(), "rtr1", "cisco", "", CollectOptions{States: []StateType{Facts}})
	if fx.calls != 1 || st1.Sections[0].Cached {
		t.Fatalf("first call: calls=%d cached=%v", fx.calls, st1.Sections[0].Cached)
	}

	// Second call: served from cache, no new exec.
	st2, _ := c.Collect(context.Background(), "rtr1", "cisco", "", CollectOptions{States: []StateType{Facts}})
	if fx.calls != 1 {
		t.Errorf("second call should hit cache, calls=%d", fx.calls)
	}
	if len(st2.Sections) != 1 || !st2.Sections[0].Cached {
		t.Errorf("second call should be cached: %+v", st2.Sections)
	}
	if st2.Facts == nil || st2.Facts.OSVersion != "15.2(4)E" {
		t.Errorf("cached facts=%+v", st2.Facts)
	}

	// Force: bypasses cache -> new exec.
	c.Collect(context.Background(), "rtr1", "cisco", "", CollectOptions{States: []StateType{Facts}, Force: true})
	if fx.calls != 2 {
		t.Errorf("force should re-exec, calls=%d want 2", fx.calls)
	}
}

func TestCollectMikrotikVersionReusesFacts(t *testing.T) {
	fx := &fakeExec{responses: map[string]string{
		"resource": "  version: 7.15.3 (stable)\n  board-name: CCR2004\n  uptime: 1d\n",
		"route":    " 0 Dac dst-address=10.0.0.0/24 gateway=ether1 immediate-gw=ether1\n",
	}}
	c := NewCollector(fx, NewCache(time.Minute))

	st, err := c.Collect(context.Background(), "mk1", "ros", "", CollectOptions{States: []StateType{Facts, Routes}})
	if err != nil {
		t.Fatal(err)
	}
	if st.Vendor != "mikrotik" {
		t.Fatalf("vendor=%q", st.Vendor)
	}
	// v7 encodes connect as lowercase 'c'; a version-unaware parse would miss it.
	if len(st.Routes) != 1 || st.Routes[0].Protocol != "connected" || st.Routes[0].Interface != "ether1" {
		t.Fatalf("routes=%+v want v7 connected via ether1", st.Routes)
	}
	// Version is resolved from the cached facts section, so facts is fetched once
	// and reused: resource print + route print = 2 execs, not 3.
	if fx.calls != 2 {
		t.Errorf("exec calls=%d want 2 (facts reused for version)", fx.calls)
	}
}

func TestCollectMikrotikVersionProbeWhenFactsUnrequested(t *testing.T) {
	fx := &fakeExec{responses: map[string]string{
		"resource": "  version: 7.15.3 (stable)\n",
		"route":    " 0 As dst-address=0.0.0.0/0 gateway=10.0.0.254 immediate-gw=10.0.0.254%ether1\n",
	}}
	c := NewCollector(fx, nil) // caching disabled

	st, err := c.Collect(context.Background(), "mk1", "ros", "", CollectOptions{States: []StateType{Routes}})
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Routes) != 1 || st.Routes[0].Protocol != "static" || st.Routes[0].NextHop != "10.0.0.254" {
		t.Fatalf("routes=%+v want v7 static via 10.0.0.254", st.Routes)
	}
	// Facts was probed only to resolve the version; it must not appear in output.
	if st.Facts != nil {
		t.Errorf("facts should not be reported when unrequested: %+v", st.Facts)
	}
	if len(st.Sections) != 1 || st.Sections[0].Type != Routes {
		t.Errorf("sections=%+v want only routes", st.Sections)
	}
}

func TestCollectUnsupportedVendor(t *testing.T) {
	c := NewCollector(&fakeExec{}, nil)
	if _, err := c.Collect(context.Background(), "h", "nokia", "sros", CollectOptions{}); err == nil {
		t.Error("expected error for unsupported vendor")
	}
}

func TestParseStateTypes(t *testing.T) {
	ts, err := ParseStateTypes([]string{"facts", "MAC", "all"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != len(AllStates) {
		t.Errorf("all should expand to %d types, got %d", len(AllStates), len(ts))
	}
	if _, err := ParseStateTypes([]string{"bogus"}); err == nil {
		t.Error("expected error for bogus state type")
	}
}
