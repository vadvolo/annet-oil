package audit

import (
	"context"
	"testing"
	"time"

	"annet-oil/internal/config"
)

func TestNewRecorder_DisabledReturnsNop(t *testing.T) {
	rec, err := NewRecorder(config.AuditConfig{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := rec.(NopRecorder); !ok {
		t.Fatalf("expected NopRecorder when disabled, got %T", rec)
	}
	// Record must be a safe no-op (no DB, no panic).
	rec.Record(context.Background(), Event{Action: ActionDiff, Actor: "tester"})
	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNopRecorder_ListReportsDisabled(t *testing.T) {
	var rec Recorder = NopRecorder{}
	if _, _, err := rec.List(context.Background(), Filter{}); err == nil {
		t.Fatal("expected List on disabled recorder to error")
	}
}

func TestActorFrom_FallbackAndOverride(t *testing.T) {
	// No actor in context → legacy fallback.
	a := ActorFrom(context.Background())
	if a.Name != "legacy" || a.Source != "unknown" {
		t.Fatalf("fallback actor = %+v", a)
	}

	ctx := WithActor(context.Background(), "alice", "admin", SourceAPI)
	a = ActorFrom(ctx)
	if a.Name != "alice" || a.Role != "admin" || a.Source != SourceAPI {
		t.Fatalf("actor from context = %+v", a)
	}
}

func TestBuildWhere(t *testing.T) {
	yes := true
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	where, args := buildWhere(Filter{
		Actor:   "alice",
		Action:  ActionDeploy,
		Source:  SourceCLI,
		Device:  "r1",
		From:    from,
		Success: &yes,
	})

	if want := "actor = $1"; !contains(where, want) {
		t.Errorf("where %q missing %q", where, want)
	}
	if want := "$4 = ANY(devices)"; !contains(where, want) {
		t.Errorf("where %q missing device predicate %q", where, want)
	}
	if want := "success = $6"; !contains(where, want) {
		t.Errorf("where %q missing %q", where, want)
	}
	if len(args) != 6 {
		t.Fatalf("expected 6 args, got %d: %v", len(args), args)
	}
	if args[0] != "alice" || args[3] != "r1" || args[5] != true {
		t.Errorf("args order wrong: %v", args)
	}
}

func TestBuildWhere_Empty(t *testing.T) {
	where, args := buildWhere(Filter{})
	if where != "" {
		t.Errorf("empty filter should yield no WHERE, got %q", where)
	}
	if len(args) != 0 {
		t.Errorf("empty filter should yield no args, got %v", args)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(needle) == 0 ||
		indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
