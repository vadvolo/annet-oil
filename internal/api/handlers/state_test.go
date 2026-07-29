package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// These exercise the HTTP glue paths that return before any device I/O, so a
// nil gnetcli client is never dereferenced.
func TestStateHandler_ValidationErrors(t *testing.T) {
	h := NewStateHandler(nil, time.Minute)

	cases := []struct {
		name string
		url  string
		want int
	}{
		{"missing host", "/", http.StatusBadRequest},
		{"bad state type", "/?host=198.51.100.7&states=bogus", http.StatusBadRequest},
		// TEST-NET-2 host is not in inventory, so no platform rescues the unknown
		// vendor and collection fails before any device I/O.
		{"unsupported vendor", "/?host=198.51.100.7&vendor=nokia", http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, c.url, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Errorf("%s: code=%d want %d (body: %s)", c.name, rec.Code, c.want, rec.Body.String())
			}
		})
	}
}
