package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/annet"
	"annet-oil/internal/checkeast"
)

// CheckeastHandler runs a diff and archives it to S3. It sits next to /diff
// rather than being a flag on it, keeping an auditable record of every diff.
type CheckeastHandler struct {
	service *annet.Service
	store   *checkeast.Store
}

// checkeastResponse is the JSON envelope: the diff plus the archive report.
type checkeastResponse struct {
	Diff    *annet.CommandResponse `json:"diff"`
	Archive *checkeast.Report      `json:"archive"`
}

func NewCheckeastHandler(service *annet.Service, store *checkeast.Store) chi.Router {
	h := &CheckeastHandler{service: service, store: store}

	r := chi.NewRouter()
	r.Get("/", h.Handle)
	r.Post("/", h.Handle)
	// Read-back of previously archived diffs (for the web Diff Tracker).
	r.Get("/runs", h.ListRuns)
	r.Get("/object", h.GetObject)

	return r
}

// storedDiffsResponse lists archived per-host diffs. Enabled is false when the
// S3 store is not configured, letting the UI show an informative empty state.
type storedDiffsResponse struct {
	Enabled bool                   `json:"enabled"`
	Diffs   []checkeast.StoredDiff `json:"diffs"`
}

// ListRuns returns archived per-host diffs, newest first. Optional ?host= filters
// to a single device. Returns an empty (but successful) list when archival is off.
func (h *CheckeastHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	out := storedDiffsResponse{Enabled: h.store != nil, Diffs: []checkeast.StoredDiff{}}
	if h.store != nil {
		diffs, err := h.store.List(r.Context(), r.URL.Query().Get("host"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if diffs != nil {
			out.Diffs = diffs
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetObject streams one archived object (raw diff text, or the decoded combined
// JSON) by its S3 key. The key is validated against the archive prefix by the store.
func (h *CheckeastHandler) GetObject(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "checkeast store is disabled", http.StatusServiceUnavailable)
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	data, contentType, err := h.store.Get(r.Context(), key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(data)
}

func (h *CheckeastHandler) Handle(w http.ResponseWriter, r *http.Request) {
	req, err := h.parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Run the diff (reused as-is). ExecuteCommand returns a response even for
	// failures, so an archive of a failed run is still useful.
	resp, err := h.service.ExecuteCommand(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out := checkeastResponse{Diff: resp}
	if h.store == nil {
		// Archival being off is not an error — return the diff with a report
		// noting the store is disabled.
		out.Archive = &checkeast.Report{
			Success: false,
			Error:   &checkeast.Error{Type: checkeast.ErrStoreDisabled, Message: "checkeast store is disabled"},
		}
	} else {
		out.Archive, _ = h.store.Archive(r.Context(), req, resp)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// parseRequest mirrors DiffHandler.parseRequest — it forces Command:"diff" and
// reads the same fields from a JSON body (POST) or query params (GET).
func (h *CheckeastHandler) parseRequest(r *http.Request) (*annet.CommandRequest, error) {
	req := &annet.CommandRequest{Command: "diff"}

	if r.Method == http.MethodPost {
		if r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(req); err != nil {
				return nil, err
			}
		}
		req.Command = "diff"
		return req, nil
	}

	query := r.URL.Query()

	if filters := query.Get("filters"); filters != "" {
		req.Filters = strings.Split(filters, ",")
	}
	if generators := query.Get("generators"); generators != "" {
		req.Generators = strings.Split(generators, ",")
	}
	if container := query.Get("container"); container != "" {
		req.Container = container
	}
	if query.Get("parallel") == "true" {
		req.Parallel = true
	}
	if timeoutStr := query.Get("timeout"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			req.Timeout = timeout
		}
	}
	if query.Get("quiet") == "true" {
		req.Quiet = true
	}

	return req, nil
}
