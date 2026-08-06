package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/gnetcli"
	"annet-oil/internal/inventory"
	"annet-oil/internal/opstate"
)

// StateHandler serves normalized operational state collected from devices.
// The cache is shared across requests; each request builds a collector bound to
// its device's credentials over that shared cache.
type StateHandler struct {
	client *gnetcli.Client
	cache  *opstate.Cache
}

// StateRequest is the POST body / query for /state.
type StateRequest struct {
	Host   string   `json:"host"`             // hostname or IP (required)
	Vendor string   `json:"vendor,omitempty"` // override inventory vendor
	States []string `json:"states,omitempty"` // sections; default facts,interfaces,lldp; "all" for everything
	Force  bool     `json:"force,omitempty"`  // bypass cache
}

// NewStateHandler builds the handler. cacheTTL <= 0 disables caching.
func NewStateHandler(client *gnetcli.Client, cacheTTL time.Duration) http.Handler {
	h := &StateHandler{
		client: client,
		cache:  opstate.NewCache(cacheTTL),
	}
	r := chi.NewRouter()
	r.Get("/", h.handleGet)
	r.Post("/", h.handlePost)
	return r
}

func (h *StateHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := StateRequest{
		Host:   q.Get("host"),
		Vendor: q.Get("vendor"),
	}
	if v := q.Get("states"); v != "" {
		req.States = splitCSV(v)
	}
	if v := q.Get("force"); v != "" {
		req.Force = v == "1" || strings.EqualFold(v, "true")
	}
	h.run(w, r, req)
}

func (h *StateHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	var req StateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.run(w, r, req)
}

func (h *StateHandler) run(w http.ResponseWriter, r *http.Request, req StateRequest) {
	if req.Host == "" {
		http.Error(w, "host field is required", http.StatusBadRequest)
		return
	}

	states, err := opstate.ParseStateTypes(req.States)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Resolve the device from inventory (falling back to a bare target using
	// env credentials, mirroring the execute handler).
	device, derr := inventory.GetDevice(req.Host)
	if derr != nil {
		log.Printf("[state] host %s not in inventory, using env credentials: %v", req.Host, derr)
		device = &inventory.Device{
			Hostname: req.Host,
			IP:       req.Host,
			Vendor:   req.Vendor,
			Credentials: inventory.DeviceCredentials{
				Login:    os.Getenv("DEVICE_USERNAME"),
				Password: os.Getenv("DEVICE_PASSWORD"),
			},
		}
	}
	if req.Vendor != "" {
		device.Vendor = strings.ToLower(req.Vendor)
	}

	// Build a collector bound to this device's credentials over the shared cache.
	collector := opstate.NewCollector(newGnetcliExecutorForDevice(h.client, device), h.cache)

	result, cerr := collector.Collect(r.Context(), device.Hostname, device.Vendor, device.Platform, opstate.CollectOptions{
		States: states,
		Force:  req.Force,
	})
	if cerr != nil {
		http.Error(w, cerr.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func newGnetcliExecutorForDevice(client *gnetcli.Client, device *inventory.Device) opstate.Executor {
	return opstate.ExecFunc(func(ctx context.Context, host, cmd string) (string, error) {
		return execViaGnetcli(ctx, client, device, cmd)
	})
}

func execViaGnetcli(ctx context.Context, client *gnetcli.Client, device *inventory.Device, cmd string) (string, error) {
	target := device.IP
	if target == "" {
		target = device.Hostname
	}
	creds := inventory.PrimaryCredentials(device)

	res, err := client.ExecWithDevice(ctx, target, cmd, device.Vendor, creds.Login, creds.Password, device.GetPort(), device.UsesTelnet())
	if err != nil {
		return "", err
	}
	if res.Output == "" && res.Error != "" {
		return "", fmt.Errorf("%s", res.Error)
	}
	return res.Output, nil
}

func splitCSV(s string) []string {
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
