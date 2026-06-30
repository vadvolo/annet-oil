package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"annet-oil/internal/gnetcli"
	"annet-oil/internal/inventory"
)

type ExecuteHandler struct {
	client    *gnetcli.Client
	whitelist []*regexp.Regexp
}

type ExecuteRequest struct {
	Host    string `json:"host"`
	Command string `json:"command"`
	Device  string `json:"device,omitempty"` // Optional device/vendor type (cisco, juniper, etc)
}

type ExecuteResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
	Status int32  `json:"status"`
}

var CommandWhitelist = []*regexp.Regexp{
	// Show commands - safe read-only operations
	regexp.MustCompile(`(?i)^show\s+version$`),
	regexp.MustCompile(`(?i)^show\s+inventory$`),
	regexp.MustCompile(`(?i)^show\s+interfaces?(\s+status)?$`),
	regexp.MustCompile(`(?i)^show\s+interfaces?\s+brief$`),
	regexp.MustCompile(`(?i)^show\s+interfaces?\s+description$`),
	regexp.MustCompile(`(?i)^show\s+interfaces?\s+\S+$`),
	regexp.MustCompile(`(?i)^show\s+ip\s+interfaces?(\s+brief)?$`),
	regexp.MustCompile(`(?i)^show\s+ipv6\s+interfaces?(\s+brief)?$`),

	// Configuration display
	regexp.MustCompile(`(?i)^show\s+running-config$`),
	regexp.MustCompile(`(?i)^show\s+startup-config$`),
	regexp.MustCompile(`(?i)^show\s+config$`),
	regexp.MustCompile(`(?i)^show\s+configuration$`),
	regexp.MustCompile(`(?i)^show\s+running-config\s+interface\s+\S+$`),
	regexp.MustCompile(`(?i)^show\s+running-config\s+\|\s+section\s+\S+$`),

	// Routing protocols
	regexp.MustCompile(`(?i)^show\s+ip\s+route$`),
	regexp.MustCompile(`(?i)^show\s+ipv6\s+route$`),
	regexp.MustCompile(`(?i)^show\s+ip\s+route\s+\S+$`),
	regexp.MustCompile(`(?i)^show\s+ipv6\s+route\s+\S+$`),
	regexp.MustCompile(`(?i)^show\s+ip\s+bgp$`),
	regexp.MustCompile(`(?i)^show\s+ip\s+bgp\s+summary$`),
	regexp.MustCompile(`(?i)^show\s+ip\s+bgp\s+neighbors?$`),
	regexp.MustCompile(`(?i)^show\s+ip\s+ospf$`),
	regexp.MustCompile(`(?i)^show\s+ip\s+ospf\s+neighbors?$`),
	regexp.MustCompile(`(?i)^show\s+ip\s+eigrp\s+neighbors?$`),

	// Layer 2 and switching
	regexp.MustCompile(`(?i)^show\s+vlan$`),
	regexp.MustCompile(`(?i)^show\s+vlan\s+brief$`),
	regexp.MustCompile(`(?i)^show\s+vlan\s+id\s+\d+$`),
	regexp.MustCompile(`(?i)^show\s+spanning-tree$`),
	regexp.MustCompile(`(?i)^show\s+spanning-tree\s+brief$`),
	regexp.MustCompile(`(?i)^show\s+spanning-tree\s+vlan\s+\d+$`),
	regexp.MustCompile(`(?i)^show\s+vpc$`),
	regexp.MustCompile(`(?i)^show\s+vpc\s+brief$`),
	regexp.MustCompile(`(?i)^show\s+port-channel\s+summary$`),

	// MAC and ARP tables
	regexp.MustCompile(`(?i)^show\s+mac\s+address-table$`),
	regexp.MustCompile(`(?i)^show\s+mac\s+address-table\s+dynamic$`),
	regexp.MustCompile(`(?i)^show\s+mac\s+address-table\s+static$`),
	regexp.MustCompile(`(?i)^show\s+mac\s+address-table\s+vlan\s+\d+$`),
	regexp.MustCompile(`(?i)^show\s+arp$`),
	regexp.MustCompile(`(?i)^show\s+ip\s+arp$`),
	regexp.MustCompile(`(?i)^show\s+ipv6\s+neighbors?$`),

	// Neighbor discovery
	regexp.MustCompile(`(?i)^show\s+cdp\s+neighbors?$`),
	regexp.MustCompile(`(?i)^show\s+cdp\s+neighbors?\s+detail$`),
	regexp.MustCompile(`(?i)^show\s+lldp\s+neighbors?$`),
	regexp.MustCompile(`(?i)^show\s+lldp\s+neighbors?\s+detail$`),

	// System monitoring
	regexp.MustCompile(`(?i)^show\s+logging$`),
	regexp.MustCompile(`(?i)^show\s+log$`),
	regexp.MustCompile(`(?i)^show\s+logging\s+last\s+\d+$`),
	regexp.MustCompile(`(?i)^show\s+tech-support$`),
	regexp.MustCompile(`(?i)^show\s+processes\s+cpu$`),
	regexp.MustCompile(`(?i)^show\s+processes\s+memory$`),
	regexp.MustCompile(`(?i)^show\s+memory$`),
	regexp.MustCompile(`(?i)^show\s+environment$`),
	regexp.MustCompile(`(?i)^show\s+environment\s+temperature$`),
	regexp.MustCompile(`(?i)^show\s+environment\s+power$`),
	regexp.MustCompile(`(?i)^show\s+environment\s+fan$`),

	// Management
	regexp.MustCompile(`(?i)^show\s+ntp\s+status$`),
	regexp.MustCompile(`(?i)^show\s+ntp\s+associations?$`),
	regexp.MustCompile(`(?i)^show\s+clock$`),
	regexp.MustCompile(`(?i)^show\s+snmp$`),
	regexp.MustCompile(`(?i)^show\s+snmp\s+community$`),
	regexp.MustCompile(`(?i)^show\s+users?$`),
	regexp.MustCompile(`(?i)^show\s+tacacs$`),
	regexp.MustCompile(`(?i)^show\s+radius$`),
	regexp.MustCompile(`(?i)^show\s+aaa$`),

	// Security
	regexp.MustCompile(`(?i)^show\s+access-lists?$`),
	regexp.MustCompile(`(?i)^show\s+ip\s+access-lists?$`),
	regexp.MustCompile(`(?i)^show\s+access-lists?\s+\S+$`),
	regexp.MustCompile(`(?i)^show\s+firewall$`),
	regexp.MustCompile(`(?i)^show\s+crypto$`),
	regexp.MustCompile(`(?i)^show\s+crypto\s+ipsec$`),
	regexp.MustCompile(`(?i)^show\s+crypto\s+isakmp$`),

	// Diagnostics
	regexp.MustCompile(`(?i)^ping\s+[\d\.]+$`),
	regexp.MustCompile(`(?i)^ping\s+[a-fA-F0-9:]+$`),
	regexp.MustCompile(`(?i)^ping\s+\S+$`),
	regexp.MustCompile(`(?i)^traceroute\s+[\d\.]+$`),
	regexp.MustCompile(`(?i)^traceroute\s+[a-fA-F0-9:]+$`),
	regexp.MustCompile(`(?i)^traceroute\s+\S+$`),
}

func NewExecuteHandler(client *gnetcli.Client) chi.Router {
	h := &ExecuteHandler{
		client:    client,
		whitelist: CommandWhitelist,
	}

	r := chi.NewRouter()
	r.Post("/", h.HandleExecute)

	return r
}

func (h *ExecuteHandler) HandleExecute(w http.ResponseWriter, r *http.Request) {
	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Host == "" {
		http.Error(w, "host field is required", http.StatusBadRequest)
		return
	}
	if req.Command == "" {
		http.Error(w, "command field is required", http.StatusBadRequest)
		return
	}

	if !h.isCommandAllowed(req.Command) {
		http.Error(w, fmt.Sprintf("command not allowed: %s", req.Command), http.StatusForbidden)
		return
	}

	// Default vendor/device type
	vendor := "cisco"
	if req.Device != "" {
		vendor = strings.ToLower(req.Device)
	}

	// Try to get device from inventory
	device, err := inventory.GetDevice(req.Host)
	if err != nil {
		log.Printf("[execute] Device %s not found in inventory, using defaults: %v", req.Host, err)
		// Use default values if device not in inventory
		device = &inventory.Device{
			Hostname: req.Host,
			IP:       req.Host,
			Vendor:   vendor, // Use vendor from request or default
			Credentials: inventory.DeviceCredentials{
				Login:    os.Getenv("DEVICE_USERNAME"),
				Password: os.Getenv("DEVICE_PASSWORD"),
			},
		}
	} else {
		// If device from request is provided, override inventory vendor
		if req.Device != "" {
			device.Vendor = vendor
		}
	}

	// Use IP if available, otherwise use hostname
	targetHost := device.IP
	if targetHost == "" {
		targetHost = device.Hostname
	}

	log.Printf("[execute] Executing command on device: host=%s, ip=%s, vendor=%s",
		device.Hostname, targetHost, device.Vendor)

	// Execute command with device parameters
	result, err := h.client.ExecWithDevice(
		r.Context(),
		targetHost,
		req.Command,
		device.Vendor,
		device.Credentials.Login,
		device.Credentials.Password,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ExecuteResponse{
		Output: result.Output,
		Error:  result.Error,
		Status: result.Status,
	})
}

func (h *ExecuteHandler) isCommandAllowed(command string) bool {
	trimmed := strings.TrimSpace(command)
	for _, pattern := range h.whitelist {
		if pattern.MatchString(trimmed) {
			return true
		}
	}
	return false
}

func GetAllowedCommands() []string {
	return []string{
		"Interface information (show interfaces, show ip interface)",
		"Configuration display (show running-config, show startup-config)",
		"Routing protocols (show ip route, show ip bgp, show ip ospf)",
		"Layer 2 switching (show vlan, show spanning-tree, show vpc)",
		"MAC and ARP tables (show mac address-table, show arp)",
		"Neighbor discovery (show cdp neighbors, show lldp neighbors)",
		"System monitoring (show logging, show processes, show memory)",
		"Management protocols (show ntp, show snmp, show users)",
		"Security (show access-lists, show crypto)",
		"Diagnostics (ping, traceroute)",
	}
}
