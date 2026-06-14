package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/annet"
)

type ContainersHandler struct {
	service *annet.Service
}

func NewContainersHandler(service *annet.Service) chi.Router {
	h := &ContainersHandler{service: service}

	r := chi.NewRouter()
	r.Get("/", h.HandleGetContainers)

	return r
}

func (h *ContainersHandler) HandleGetContainers(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.GetContainerStatus(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}