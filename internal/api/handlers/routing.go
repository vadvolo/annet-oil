package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/router"
)

type RoutingHandler struct {
	router *router.Router
}

type AddRouteRequest struct {
	Hostname  string `json:"hostname"`
	Container string `json:"container"`
}

type RemoveRouteRequest struct {
	Hostname string `json:"hostname"`
}

func NewRoutingHandler(router *router.Router) chi.Router {
	h := &RoutingHandler{router: router}

	r := chi.NewRouter()
	r.Get("/", h.HandleGetRoutes)
	r.Post("/", h.HandleAddRoute)
	r.Delete("/", h.HandleRemoveRoute)

	return r
}

func (h *RoutingHandler) HandleGetRoutes(w http.ResponseWriter, r *http.Request) {
	routes := h.router.GetAllRoutes()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(routes); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *RoutingHandler) HandleAddRoute(w http.ResponseWriter, r *http.Request) {
	var req AddRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" || req.Container == "" {
		http.Error(w, "hostname and container are required", http.StatusBadRequest)
		return
	}

	if err := h.router.AddRoute(req.Hostname, req.Container); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Route added successfully",
	})
}

func (h *RoutingHandler) HandleRemoveRoute(w http.ResponseWriter, r *http.Request) {
	var req RemoveRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" {
		http.Error(w, "hostname is required", http.StatusBadRequest)
		return
	}

	if err := h.router.RemoveRoute(req.Hostname); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Route removed successfully",
	})
}