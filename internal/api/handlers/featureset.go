package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/featureset"
	"annet-oil/internal/inventory"
)

// FeatureSetRequest is the POST body / query for /featureset.
//
// Vendor+Model (and optionally Version) identify the platform directly. As a
// convenience, Host resolves Vendor from the inventory when Vendor is omitted;
// Model and Version still have to be supplied since the inventory does not
// track them.
type FeatureSetRequest struct {
	Vendor  string `json:"vendor,omitempty"`
	Model   string `json:"model,omitempty"`
	Version string `json:"version,omitempty"`
	Feature string `json:"feature,omitempty"` // filter to a single feature
	Host    string `json:"host,omitempty"`    // resolve vendor from inventory
}

func NewFeatureSetHandler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", handleFeatureSetGet)
	r.Post("/", handleFeatureSetPost)
	return r
}

// handleFeatureSetGet handles
// GET /featureset?vendor=juniper&model=EX4100-48MP&version=24.4R2.23&feature=ptp
func handleFeatureSetGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := FeatureSetRequest{
		Vendor:  q.Get("vendor"),
		Model:   q.Get("model"),
		Version: q.Get("version"),
		Feature: q.Get("feature"),
		Host:    q.Get("host"),
	}
	runFeatureSet(w, req)
}

func handleFeatureSetPost(w http.ResponseWriter, r *http.Request) {
	var req FeatureSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	runFeatureSet(w, req)
}

func runFeatureSet(w http.ResponseWriter, req FeatureSetRequest) {
	// Fill vendor from inventory when a host is given and vendor is not.
	if req.Vendor == "" && req.Host != "" {
		if dev, err := inventory.GetDevice(req.Host); err == nil {
			req.Vendor = dev.Vendor
		}
	}

	if strings.TrimSpace(req.Vendor) == "" || strings.TrimSpace(req.Model) == "" {
		http.Error(w, "vendor and model are required (or a host resolvable to a vendor plus a model)", http.StatusBadRequest)
		return
	}

	result := featureset.Resolve(featureset.Query{
		Vendor:  req.Vendor,
		Model:   req.Model,
		Version: req.Version,
		Feature: req.Feature,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
