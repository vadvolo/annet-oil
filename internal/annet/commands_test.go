package annet

import (
	"context"
	"sync"
	"testing"

	"annet-oil/internal/audit"
	"annet-oil/internal/config"
)

// memRecorder is an in-memory audit.Recorder for tests. It enriches events from
// context the same way the production PostgresRecorder does, so it exercises the
// actor/source propagation contract.
type memRecorder struct {
	mu     sync.Mutex
	events []audit.Event
}

func (m *memRecorder) Record(ctx context.Context, e audit.Event) {
	a := audit.ActorFrom(ctx)
	if e.Actor == "" {
		e.Actor = a.Name
	}
	if e.Source == "" {
		e.Source = a.Source
	}
	m.mu.Lock()
	m.events = append(m.events, e)
	m.mu.Unlock()
}

func (m *memRecorder) List(context.Context, audit.Filter) ([]audit.Event, int, error) {
	return nil, 0, nil
}
func (m *memRecorder) Close() error { return nil }

// A command that fails validation still records an audit event carrying the
// action, the failure, and the actor/source resolved from context — proving the
// choke-point hook fires for every path and honours the context identity.
func TestExecuteCommand_RecordsAuditEventWithActor(t *testing.T) {
	rec := &memRecorder{}
	svc := New(&config.Config{}, nil, nil, rec)

	ctx := audit.WithActor(context.Background(), "alice", "admin", audit.SourceAPI)
	resp, err := svc.ExecuteCommand(ctx, &CommandRequest{
		Command: "bogus", // fails validateCommand before any container work
		Filters: []string{"r1", "r2"},
	})
	if err != nil {
		t.Fatalf("ExecuteCommand returned error: %v", err)
	}
	if resp.Success {
		t.Fatal("expected validation failure")
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(rec.events))
	}
	e := rec.events[0]
	if e.Action != "bogus" {
		t.Errorf("action = %q, want bogus", e.Action)
	}
	if e.Success {
		t.Error("event should record failure")
	}
	if e.Error == nil || e.Error.Type != "command_failed" {
		t.Errorf("event error = %+v, want command_failed", e.Error)
	}
	if len(e.Devices) != 2 || e.Devices[0] != "r1" {
		t.Errorf("devices = %v, want [r1 r2]", e.Devices)
	}
	if e.Actor != "alice" || e.Source != audit.SourceAPI {
		t.Errorf("actor/source = %q/%q, want alice/api", e.Actor, e.Source)
	}
}

// With a nil recorder, ExecuteCommand must not panic (auditing simply off).
func TestExecuteCommand_NilRecorderNoPanic(t *testing.T) {
	svc := New(&config.Config{}, nil, nil, nil)
	if _, err := svc.ExecuteCommand(context.Background(), &CommandRequest{Command: "bogus"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
