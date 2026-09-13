package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/audit"
)

// AuditHandler exposes the audit trail for reading. The frontend (annet-web)
// queries this endpoint rather than the database directly.
type AuditHandler struct {
	recorder audit.Recorder
}

type auditListResponse struct {
	Events []audit.Event `json:"events"`
	Total  int           `json:"total"`
}

func NewAuditHandler(recorder audit.Recorder) chi.Router {
	h := &AuditHandler{recorder: recorder}

	r := chi.NewRouter()
	r.Get("/", h.list)

	return r
}

func (h *AuditHandler) list(w http.ResponseWriter, r *http.Request) {
	if h.recorder == nil {
		http.Error(w, "audit is not available", http.StatusServiceUnavailable)
		return
	}

	f, err := parseAuditFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	events, total, err := h.recorder.List(r.Context(), f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if events == nil {
		events = []audit.Event{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(auditListResponse{Events: events, Total: total}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func parseAuditFilter(r *http.Request) (audit.Filter, error) {
	q := r.URL.Query()
	f := audit.Filter{
		Actor:  q.Get("actor"),
		Device: q.Get("device"),
		Action: q.Get("action"),
		Source: q.Get("source"),
	}

	if v := q.Get("from"); v != "" {
		t, err := parseTime(v)
		if err != nil {
			return f, err
		}
		f.From = t
	}
	if v := q.Get("to"); v != "" {
		t, err := parseTime(v)
		if err != nil {
			return f, err
		}
		f.To = t
	}
	if v := q.Get("success"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return f, err
		}
		f.Success = &b
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Offset = n
		}
	}

	return f, nil
}

// parseTime accepts RFC3339 or a plain date (YYYY-MM-DD).
func parseTime(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", v)
}
