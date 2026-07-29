package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/check"
	"annet-oil/internal/inventory"
)

// CheckRequest is the POST body for /check.
type CheckRequest struct {
	Host    string `json:"host"`              // hostname or IP (required)
	Ports   []int  `json:"ports,omitempty"`   // extra ports to probe
	Login   *bool  `json:"login,omitempty"`   // attempt SSH login (default true)
	Timeout int    `json:"timeout,omitempty"` // per-port dial timeout, seconds
}

func NewCheckHandler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", handleCheckGet)
	r.Post("/", handleCheckPost)
	return r
}

// handleCheckGet handles GET /check?host=...&ports=22,23&login=true&timeout=3
func handleCheckGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := CheckRequest{Host: q.Get("host")}

	if v := q.Get("ports"); v != "" {
		req.Ports = parsePorts(v)
	}
	if v := q.Get("login"); v != "" {
		b := v == "1" || strings.EqualFold(v, "true")
		req.Login = &b
	}
	if v := q.Get("timeout"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			req.Timeout = n
		}
	}

	runCheck(w, r, req)
}

func handleCheckPost(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	runCheck(w, r, req)
}

func runCheck(w http.ResponseWriter, r *http.Request, req CheckRequest) {
	if req.Host == "" {
		http.Error(w, "host field is required", http.StatusBadRequest)
		return
	}

	// Resolve device from inventory, falling back to a bare target so hosts
	// not present in inventory can still be probed.
	device, err := inventory.GetDevice(req.Host)
	if err != nil {
		log.Printf("[check] host %s not in inventory, probing directly: %v", req.Host, err)
		device = &inventory.Device{
			Hostname: req.Host,
			IP:       req.Host,
			Credentials: inventory.DeviceCredentials{
				Login:    os.Getenv("DEVICE_USERNAME"),
				Password: os.Getenv("DEVICE_PASSWORD"),
			},
		}
	}

	checkLogin := true
	if req.Login != nil {
		checkLogin = *req.Login
	}

	opts := check.Options{
		Ports:      req.Ports,
		CheckLogin: checkLogin,
	}
	if req.Timeout > 0 {
		opts.DialTimeout = time.Duration(req.Timeout) * time.Second
	}

	result := check.Device(r.Context(), device, opts)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func parsePorts(s string) []int {
	var ports []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if n, err := strconv.Atoi(part); err == nil {
			ports = append(ports, n)
		}
	}
	return ports
}
