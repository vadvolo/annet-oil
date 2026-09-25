package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"annet-oil/internal/diag"
	"annet-oil/internal/diag/diagpb"
	"annet-oil/internal/gnetcli"
	"annet-oil/internal/inventory"
)

// DiagConfig tunes the diagnostic handler. Zero values fall back to catalog
// defaults (two MikroTik probes, two core anchors, a 90s step budget).
type DiagConfig struct {
	ProbeHosts  []string // MikroTik probe IPs to ping FROM (availability)
	CoreAnchors []string // anchor IPs to ping TO (availability)
	StepBudget  time.Duration
	PingCount   int
}

// DiagHandler serves read-only diagnostic playbooks with structured output.
type DiagHandler struct {
	client *gnetcli.Client
	engine *diag.Engine
	cfg    DiagConfig
}

func NewDiagHandler(client *gnetcli.Client, cfg DiagConfig) chi.Router {
	if len(cfg.ProbeHosts) == 0 {
		cfg.ProbeHosts = []string{"192.168.224.5", "95.140.112.230"}
	}
	if len(cfg.CoreAnchors) == 0 {
		cfg.CoreAnchors = []string{"172.22.2.4", "172.22.2.16"}
	}
	if cfg.StepBudget == 0 {
		cfg.StepBudget = 90 * time.Second
	}
	if cfg.PingCount == 0 {
		cfg.PingCount = 4
	}
	h := &DiagHandler{
		client: client,
		cfg:    cfg,
		engine: diag.NewEngine(client, diag.Config{
			StepBudget: cfg.StepBudget,
			PingCount:  cfg.PingCount,
		}),
	}
	r := chi.NewRouter()
	r.Post("/run", h.handleRun)
	r.Get("/playbooks", h.handlePlaybooks)
	r.Get("/schema", h.handleSchema)
	return r
}

// DiagRunRequest is the request body for POST /diag/run.
type DiagRunRequest struct {
	Host     string  `json:"host"`             // device hostname or IP (required)
	Alarm    string  `json:"alarm"`            // alarm type name (required)
	Port     string  `json:"port,omitempty"`   // <PORT>
	Peer     string  `json:"peer,omitempty"`   // <PEER>
	Target   string  `json:"target,omitempty"` // <CILJ>
	Location string  `json:"location,omitempty"`
	LastHop  string  `json:"last_hop,omitempty"`  // availability: last hop before device
	Vendor   string  `json:"vendor,omitempty"`    // optional inventory override
	Platform string  `json:"platform,omitempty"`  // optional inventory override
	NoProbes bool    `json:"no_probes,omitempty"` // availability: skip probe pings
	TimeoutS float64 `json:"timeout_s,omitempty"` // per-command timeout override

	Extra map[string]string `json:"extra,omitempty"`
}

func (h *DiagHandler) handleRun(w http.ResponseWriter, r *http.Request) {
	var req DiagRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Host == "" {
		http.Error(w, "host field is required", http.StatusBadRequest)
		return
	}
	alarm, err := diag.ParseAlarm(req.Alarm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dev := resolveTarget(req.Host, req.Vendor, req.Platform)

	run := diag.RunRequest{
		Device:   dev,
		Alarm:    alarm,
		Port:     req.Port,
		Peer:     req.Peer,
		CILJ:     req.Target,
		Location: req.Location,
		LastHop:  req.LastHop,
		Extra:    req.Extra,
	}
	// Availability: attach the MikroTik probes + core anchors unless suppressed.
	if alarm == diagpb.AlarmType_ALARM_AVAILABILITY && !req.NoProbes {
		for _, p := range h.cfg.ProbeHosts {
			run.Probes = append(run.Probes, resolveTarget(p, "ros", "routeros"))
		}
		run.CoreAnchors = h.cfg.CoreAnchors
	}
	// Per-request timeout override flows to the engine via a fresh engine copy.
	eng := h.engine
	if req.TimeoutS > 0 {
		eng = diag.NewEngine(h.client, diag.Config{
			StepBudget:        h.cfg.StepBudget,
			PingCount:         h.cfg.PingCount,
			DefaultTimeoutSec: req.TimeoutS,
		})
	}

	result := eng.Run(r.Context(), run)
	writeProto(w, r, result)
}

// resolveTarget builds an engine Target from the inventory (with env-credential
// fallback), mirroring the /execute handler.
func resolveTarget(host, vendorOverride, platformOverride string) diag.Target {
	device, err := inventory.GetDevice(host)
	if err != nil {
		device = &inventory.Device{
			Hostname: host,
			IP:       host,
			Vendor:   "cisco",
			Credentials: inventory.DeviceCredentials{
				Login:    os.Getenv("DEVICE_USERNAME"),
				Password: os.Getenv("DEVICE_PASSWORD"),
			},
		}
	}
	if vendorOverride != "" {
		device.Vendor = strings.ToLower(vendorOverride)
	}
	if platformOverride != "" {
		device.Platform = strings.ToLower(platformOverride)
	}
	creds := inventory.PrimaryCredentials(device)
	return diag.Target{
		Hostname: device.Hostname,
		IP:       device.IP,
		Vendor:   device.Vendor,
		Platform: device.Platform,
		Login:    creds.Login,
		Password: creds.Password,
		Port:     device.GetPort(),
	}
}

// writeProto serializes msg as binary protobuf when the client asks for it
// (Accept: application/protobuf or ?format=pb), else protojson.
func writeProto(w http.ResponseWriter, r *http.Request, msg proto.Message) {
	wantPB := r.URL.Query().Get("format") == "pb" ||
		strings.Contains(r.Header.Get("Accept"), "application/protobuf") ||
		strings.Contains(r.Header.Get("Accept"), "application/x-protobuf")
	if wantPB {
		b, err := proto.Marshal(msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/protobuf")
		w.Write(b)
		return
	}
	b, err := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: false}.Marshal(msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

// handlePlaybooks returns a descriptor of families, alarms, and command sets.
func (h *DiagHandler) handlePlaybooks(w http.ResponseWriter, r *http.Request) {
	type stepDesc struct {
		Command  string `json:"command"`
		Category string `json:"category"`
	}
	type cell struct {
		Steps []stepDesc `json:"steps,omitempty"`
		Gap   string     `json:"gap,omitempty"`
	}
	out := map[string]any{}
	families := []string{}
	for _, f := range diag.AllFamilies {
		families = append(families, diag.FamilyName(f))
	}
	byAlarm := map[string]map[string]cell{}
	for _, a := range diag.AllAlarms {
		byFam := map[string]cell{}
		for _, f := range diag.AllFamilies {
			steps, gap := diag.Playbook(a, f)
			if len(steps) == 0 && gap == "" {
				continue
			}
			c := cell{Gap: gap}
			for _, st := range steps {
				c.Steps = append(c.Steps, stepDesc{Command: st.Command, Category: st.Category})
			}
			byFam[diag.FamilyName(f)] = c
		}
		byAlarm[diag.AlarmName(a)] = byFam
	}
	out["families"] = families
	out["alarms"] = byAlarm
	out["placeholders"] = []string{"<PORT>", "<PEER>", "<CILJ>", "<lok>"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// handleSchema serves the protobuf schema text.
func (h *DiagHandler) handleSchema(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(diag.SchemaProto))
}
