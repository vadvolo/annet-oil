package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"annet-oil/internal/annet"
	"annet-oil/internal/audit"
	"annet-oil/internal/config"
)

// checkeast handler with a nil store returns the diff plus an archive report
// noting the store is disabled — HTTP 200, not an error.
func TestCheckeastHandler_StoreDisabled(t *testing.T) {
	svc := annet.New(&config.Config{}, nil, nil, nil)
	h := NewCheckeastHandler(svc, nil)

	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var out struct {
		Diff    *annet.CommandResponse `json:"diff"`
		Archive struct {
			Success bool `json:"success"`
			Error   *struct {
				Type string `json:"type"`
			} `json:"error"`
		} `json:"archive"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Diff == nil {
		t.Error("expected a diff in the response")
	}
	if out.Archive.Success {
		t.Error("archive should be unsuccessful when store is disabled")
	}
	if out.Archive.Error == nil || out.Archive.Error.Type != "store_disabled" {
		t.Errorf("archive error = %+v, want store_disabled", out.Archive.Error)
	}
}

// listRecorder is a recorder that returns a fixed event list from List.
type listRecorder struct {
	events []audit.Event
}

func (l *listRecorder) Record(context.Context, audit.Event) {}
func (l *listRecorder) List(_ context.Context, f audit.Filter) ([]audit.Event, int, error) {
	// echo the actor filter back so we can assert parsing wired through.
	if f.Actor != "" && f.Actor != "alice" {
		return nil, 0, nil
	}
	return l.events, len(l.events), nil
}
func (l *listRecorder) Close() error { return nil }

func TestAuditHandler_ListJSON(t *testing.T) {
	rec := &listRecorder{events: []audit.Event{
		{ID: 1, Timestamp: time.Now(), Actor: "alice", Source: "api", Action: "diff", Success: true, Devices: []string{"r1"}},
	}}
	h := NewAuditHandler(rec)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/?actor=alice&action=diff&limit=10")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var out struct {
		Events []audit.Event `json:"events"`
		Total  int           `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Total != 1 || len(out.Events) != 1 {
		t.Fatalf("expected 1 event, got total=%d len=%d", out.Total, len(out.Events))
	}
	if out.Events[0].Actor != "alice" || out.Events[0].Action != "diff" {
		t.Errorf("event = %+v", out.Events[0])
	}
}

// A nil recorder makes the audit endpoint report unavailable rather than crash.
func TestAuditHandler_NilRecorderUnavailable(t *testing.T) {
	h := NewAuditHandler(nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}
