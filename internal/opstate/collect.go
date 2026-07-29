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

	for _, t := range states {
		cmd, ok := parser.Command(t)
		if !ok {
			dst.Errors = append(dst.Errors, SectionError{Type: t, Message: fmt.Sprintf("state %q not supported for vendor %s", t, parser.Vendor())})
			continue
		}

		key := host + "|" + parser.Vendor() + "|" + string(t)

		// Serve from cache unless forced.
		if !opts.Force {
			if cached, hit := c.cache.get(key); hit {
				setSection(dst, t, cached)
				dst.Sections = append(dst.Sections, SectionMeta{Type: t, Cached: true})
				continue
			}
		}

		raw, execErr := c.exec.Exec(ctx, host, cmd)
		if execErr != nil {
			dst.Errors = append(dst.Errors, SectionError{Type: t, Message: fmt.Sprintf("exec %q: %v", cmd, execErr)})
			continue
		}

		scratch := &State{}
		if perr := parser.Parse(t, raw, scratch); perr != nil {
			dst.Errors = append(dst.Errors, SectionError{Type: t, Message: fmt.Sprintf("parse %s: %v", t, perr)})
			continue
		}

		setSection(dst, t, scratch)
		dst.Sections = append(dst.Sections, SectionMeta{Type: t, Cached: false})

		// Cache just this parsed section for reuse by later requests.
		c.cache.set(key, scratch)
	}

	return dst, nil
}
