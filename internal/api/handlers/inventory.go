package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/inventory"
)

type InventoryResponse struct {
	Devices []DeviceInfo `json:"devices"`
	Total   int          `json:"total"`
}

type DeviceInfo struct {
	Hostname    string `json:"hostname"`
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	Vendor      string `json:"vendor"`
	Platform    string `json:"platform"`
	Description string `json:"description,omitempty"`
}

func NewInventoryHandler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", listInventory)
	return r
}

func listInventory(w http.ResponseWriter, r *http.Request) {
	vendor := r.URL.Query().Get("vendor")
	platform := r.URL.Query().Get("platform")
	pattern := r.URL.Query().Get("pattern")

	devices := inventory.FilterDevices(vendor, platform, pattern)

	response := InventoryResponse{
		Devices: make([]DeviceInfo, 0, len(devices)),
		Total:   len(devices),
	}

	for _, d := range devices {
		response.Devices = append(response.Devices, DeviceInfo{
			Hostname:    d.Hostname,
			IP:          d.IP,
			Port:        d.GetPort(),
			Vendor:      d.Vendor,
			Platform:    d.Platform,
			Description: d.Description,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
