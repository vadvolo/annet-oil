package opstate

import (
	"context"
	"fmt"
	"time"
)

// Executor runs a read-only command on a device and returns its stdout. It
// decouples the collector from gnetcli so parsers can be exercised with a fake
// executor over fixtures.
type Executor interface {
	Exec(ctx context.Context, host, cmd string) (string, error)
}

// ExecFunc adapts a function to the Executor interface.
type ExecFunc func(ctx context.Context, host, cmd string) (string, error)

func (f ExecFunc) Exec(ctx context.Context, host, cmd string) (string, error) {
	return f(ctx, host, cmd)
}

// Collector gathers normalized operational state from a device, using an
// Executor to run commands and an optional Cache to avoid re-hitting the device.
type Collector struct {
	exec  Executor
	cache *Cache
}

// NewCollector builds a collector. cache may be nil to disable caching.
func NewCollector(exec Executor, cache *Cache) *Collector {
	return &Collector{exec: exec, cache: cache}
}

// CollectOptions controls a collection run.
type CollectOptions struct {
	// States to collect. Empty means DefaultStates (facts, interfaces, lldp).
	States []StateType
	// Force bypasses the cache (always re-queries the device and refreshes the
	// cache entry).
	Force bool
}

// Collect gathers the requested operational-state sections for one device.
// Per-section failures are recorded in State.Errors; the call only returns an
// error for problems that prevent any collection (e.g. no parser for the
// vendor).
func (c *Collector) Collect(ctx context.Context, host, vendor, platform string, opts CollectOptions) (*State, error) {
	parser, err := ParserFor(vendor)
	if err != nil && platform != "" {
		parser, err = ParserFor(platform)
	}
	if err != nil {
		return nil, err
	}

	states := opts.States
	if len(states) == 0 {
		states = DefaultStates
	}

	dst := &State{
		Host:        host,
		Vendor:      parser.Vendor(),
		Platform:    platform,
		CollectedAt: time.Now(),
	}

	// Version-aware parsers (e.g. RouterOS 6 vs 7) must know the device OS major
	// version before parsing version-sensitive sections. Resolve it up front from
	// the facts output; that output is cached, so when facts is also a requested
	// section the loop below reuses it rather than re-hitting the device.
	if va, ok := parser.(versionAware); ok {
		if facts, _, err := c.section(ctx, host, parser, Facts, opts.Force); err == nil {
			va.SetMajorVersion(va.MajorVersionFromFacts(facts.Facts))
		}
	}

	for _, t := range states {
		section, cached, err := c.section(ctx, host, parser, t, opts.Force)
		if err != nil {
			dst.Errors = append(dst.Errors, SectionError{Type: t, Message: err.Error()})
			continue
		}
		setSection(dst, t, section)
		dst.Sections = append(dst.Sections, SectionMeta{Type: t, Cached: cached})
	}

	return dst, nil
}

// section produces one parsed state section, serving it from the cache unless
// forced and caching fresh results. It reports whether the result came from the
// cache. It does not mutate a destination State or record section metadata, so
// it can also be used to resolve the OS version before the main collection loop.
func (c *Collector) section(ctx context.Context, host string, parser Parser, t StateType, force bool) (*State, bool, error) {
	cmd, ok := parser.Command(t)
	if !ok {
		return nil, false, fmt.Errorf("state %q not supported for vendor %s", t, parser.Vendor())
	}

	key := host + "|" + parser.Vendor() + "|" + string(t)
	if !force {
		if cached, hit := c.cache.get(key); hit {
			return cached, true, nil
		}
	}

	raw, execErr := c.exec.Exec(ctx, host, cmd)
	if execErr != nil {
		return nil, false, fmt.Errorf("exec %q: %v", cmd, execErr)
	}

	scratch := &State{}
	if perr := parser.Parse(t, raw, scratch); perr != nil {
		return nil, false, fmt.Errorf("parse %s: %v", t, perr)
	}

	c.cache.set(key, scratch)
	return scratch, false, nil
}
