package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/gnetcli"
	"annet-oil/internal/inventory"
	"annet-oil/internal/opstate"
	"annet-oil/internal/topology"
)

// TopologyHandler builds/serves the network topology graph.
type TopologyHandler struct {
	client *gnetcli.Client
	store  *topology.Store
	cache  *opstate.Cache
}

// TopologyCollectRequest is the POST body for /topology/collect.
type TopologyCollectRequest struct {
	Hosts   []string `json:"hosts"`             // hostnames/IPs to poll (required)
	Sources []string `json:"sources,omitempty"` // lldp,cdp,ip (default lldp,ip)
	Vendor  string   `json:"vendor,omitempty"`  // override inventory vendor
	Force   bool     `json:"force,omitempty"`   // bypass opstate cache
}

// NewTopologyHandler builds the handler; storePath is the JSON store file.
func NewTopologyHandler(client *gnetcli.Client, storePath string) http.Handler {
	st := topology.NewStore(storePath)
	_ = st.Load()
	h := &TopologyHandler{client: client, store: st, cache: opstate.NewCache(0)}
	r := chi.NewRouter()
	r.Get("/", h.handleGraph)         // GET /topology?host=&depth=
	r.Post("/collect", h.handleCollect) // POST /topology/collect
	return r
}

// handleGraph returns the whole graph, or the subgraph within `depth` hops of `host`.
func (h *TopologyHandler) handleGraph(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	host := strings.TrimSpace(q.Get("host"))
	depth := 1
	if v := q.Get("depth"); v != "" {
		if d, err := strconv.Atoi(v); err == nil && d >= 0 {
			depth = d
		}
	}
	var g topology.Graph
	if host == "" {
		g = h.store.Graph()
	} else {
		g = h.store.Subgraph(host, depth)
	}
	writeJSONTopo(w, g)
}

// handleCollect polls the given hosts and records their status + links.
// Unreachable hosts are marked down (their topology is preserved), not deleted.
func (h *TopologyHandler) handleCollect(w http.ResponseWriter, r *http.Request) {
	var req TopologyCollectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Hosts) == 0 {
		http.Error(w, "hosts is required", http.StatusBadRequest)
		return
	}
	var sources []topology.Source
	for _, s := range req.Sources {
		sources = append(sources, topology.Source(strings.ToLower(strings.TrimSpace(s))))
	}
	// Map requested sources to opstate sections.
	states := []opstate.StateType{opstate.LLDP}
	wantIP := len(sources) == 0
	for _, s := range sources {
		if s == topology.SourceIP {
			wantIP = true
		}
	}
	if wantIP {
		states = append(states, opstate.ARP)
	}

	now := time.Now()
	type hostResult struct {
		Host      string `json:"host"`
		Reachable bool   `json:"reachable"`
		Edges     int    `json:"edges,omitempty"`
		Error     string `json:"error,omitempty"`
	}
	results := make([]hostResult, 0, len(req.Hosts))
	up, down := 0, 0

	for _, host := range req.Hosts {
		device, err := inventory.GetDevice(host)
		if err != nil {
			device = &inventory.Device{Hostname: host, IP: host, Vendor: req.Vendor}
		}
		if req.Vendor != "" {
			device.Vendor = strings.ToLower(req.Vendor)
		}
		collector := opstate.NewCollector(newGnetcliExecutorForDevice(h.client, device), h.cache)
		st, cerr := collector.Collect(r.Context(), device.Hostname, device.Vendor, device.Platform, opstate.CollectOptions{States: states, Force: req.Force})

		// Collect accumulates per-section failures in st.Errors and returns a nil
		// error even when the device is unreachable, so treat "no section parsed"
		// as unreachable rather than trusting cerr alone.
		res := hostResult{Host: host}
		reachable := cerr == nil && st != nil && len(st.Sections) > 0
		if !reachable {
			// Unreachable/collect error -> mark node+edges down, keep topology.
			res.Reachable = false
			switch {
			case cerr != nil:
				res.Error = cerr.Error()
			case st != nil && len(st.Errors) > 0:
				res.Error = st.Errors[0].Message
			default:
				res.Error = "no state collected"
			}
			down++
			_ = h.store.Apply(host, false, nil, now)
			results = append(results, res)
			continue
		}
		edges := topology.EdgesFromState(st, sources)
		if err := h.store.Apply(st.Host, true, edges, now); err != nil {
			res.Error = err.Error()
		}
		res.Reachable = true
		res.Edges = len(edges)
		up++
		results = append(results, res)
	}

	writeJSONTopo(w, map[string]any{
		"polled":      len(req.Hosts),
		"up":          up,
		"down":        down,
		"per_host":    results,
	})
}

func writeJSONTopo(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
