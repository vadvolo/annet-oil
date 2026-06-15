package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/annet"
)

type DiffHandler struct {
	service *annet.Service
}

func NewDiffHandler(service *annet.Service) chi.Router {
	h := &DiffHandler{service: service}

	r := chi.NewRouter()
	r.Get("/", h.HandleDiff)
	r.Post("/", h.HandleDiff)

	return r
}

func (h *DiffHandler) HandleDiff(w http.ResponseWriter, r *http.Request) {
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

func (h *DiffHandler) parseRequest(r *http.Request) (*annet.CommandRequest, error) {
	req := &annet.CommandRequest{
		Command: "diff",
	}

	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			return nil, err
		}
		req.Command = "diff"
		return req, nil
	}

	query := r.URL.Query()

	if filters := query.Get("filters"); filters != "" {
		req.Filters = strings.Split(filters, ",")
	}

	if container := query.Get("container"); container != "" {
		req.Container = container
	}

	if parallel := query.Get("parallel"); parallel == "true" {
		req.Parallel = true
	}

	if timeoutStr := query.Get("timeout"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			req.Timeout = timeout
		}
	}

	if quiet := query.Get("quiet"); quiet == "true" {
		req.Quiet = true
	}

	return req, nil
}