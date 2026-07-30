package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/inventory"
)

// InventoryHandler serves inventory listing and on-the-fly reload.
type InventoryHandler struct {
	inventoryFile string
}

type InventoryResponse struct {
	Devices []DeviceInfo `json:"devices"`
	Total   int          `json:"total"`
}

type DeviceInfo struct {
	Hostname    string   `json:"hostname"`
	IP          string   `json:"ip"`
	Port        int      `json:"port"`
	Vendor      string   `json:"vendor"`
	Platform    string   `json:"platform"`
	Description string   `json:"description,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
}

// ReloadResponse is returned by the inventory reload endpoint.
type ReloadResponse struct {
	Status     string    `json:"status"`
	Path       string    `json:"path"`
	Devices    int       `json:"devices"`
	ReloadedAt time.Time `json:"reloaded_at"`
}

func NewInventoryHandler(inventoryFile string) http.Handler {
	h := &InventoryHandler{inventoryFile: inventoryFile}

	r := chi.NewRouter()
	r.Get("/", listInventory)
	r.Post("/reload", h.reloadInventory)
	return r
}

// reloadInventory re-reads the inventory file into memory, replacing the
// current in-memory inventory without restarting the server.
func (h *InventoryHandler) reloadInventory(w http.ResponseWriter, r *http.Request) {
	if h.inventoryFile == "" {
		http.Error(w, "no inventory file configured (storage.inventory_file)", http.StatusBadRequest)
		return
	}

	inv, err := inventory.Load(h.inventoryFile)
	if err != nil {
		log.Printf("[inventory] reload failed: %v", err)
		http.Error(w, "failed to reload inventory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("[inventory] reloaded %d device(s) from %s", len(inv.Devices), h.inventoryFile)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ReloadResponse{
		Status:     "ok",
		Path:       h.inventoryFile,
		Devices:    len(inv.Devices),
		ReloadedAt: time.Now(),
	})
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
			Aliases:     d.Aliases,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
