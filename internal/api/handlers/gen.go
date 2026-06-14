package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/annet"
)

type GenHandler struct {
	service *annet.Service
}

func NewGenHandler(service *annet.Service) chi.Router {
	h := &GenHandler{service: service}

	r := chi.NewRouter()
	r.Get("/", h.HandleGen)
	r.Post("/", h.HandleGen)

	return r
}

func (h *GenHandler) HandleGen(w http.ResponseWriter, r *http.Request) {
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

func (h *GenHandler) parseRequest(r *http.Request) (*annet.CommandRequest, error) {
	req := &annet.CommandRequest{
		Command: "gen",
	}

	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			return nil, err
		}
		req.Command = "gen"
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

	return req, nil
}