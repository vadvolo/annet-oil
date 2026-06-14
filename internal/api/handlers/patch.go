package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/annet"
)

type PatchHandler struct {
	service *annet.Service
}

func NewPatchHandler(service *annet.Service) chi.Router {
	h := &PatchHandler{service: service}

	r := chi.NewRouter()
	r.Post("/", h.HandlePatch)

	return r
}

func (h *PatchHandler) HandlePatch(w http.ResponseWriter, r *http.Request) {
	req, err := h.parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.service.ExecuteCommand(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *PatchHandler) parseRequest(r *http.Request) (*annet.CommandRequest, error) {
	req := &annet.CommandRequest{
		Command: "patch",
	}

	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			return nil, err
		}
		req.Command = "patch"
		return req, nil
	}

	query := r.URL.Query()

	if filters := query.Get("filters"); filters != "" {
		req.Filters = strings.Split(filters, ",")
	}

	if container := query.Get("container"); container != "" {
		req.Container = container
	}

	if dryRun := query.Get("dry_run"); dryRun == "true" {
		req.DryRun = true
	}

	if parallel := query.Get("parallel"); parallel == "true" {
		req.Parallel = true
	}

	if timeoutStr := query.Get("timeout"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			req.Timeout = timeout
		}
	}

	return req, nil
}