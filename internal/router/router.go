package router

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"annet-oil/internal/config"
)

type Router struct {
	config      *config.Config
	routing     map[string]string
	routingFile string
	mu          sync.RWMutex
}

type RoutingData struct {
	Routes map[string]string `json:"routes"`
}

func New(cfg *config.Config) *Router {
	return &Router{
		config:      cfg,
		routing:     make(map[string]string),
		routingFile: cfg.Storage.RoutingFile,
	}
}

func (r *Router) LoadRoutes() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ensureRoutingFile(); err != nil {
		return fmt.Errorf("failed to ensure routing file: %w", err)
	}

	data, err := os.ReadFile(r.routingFile)
	if err != nil {
		return fmt.Errorf("failed to read routing file: %w", err)
	}

	var routingData RoutingData
	if err := json.Unmarshal(data, &routingData); err != nil {
		return fmt.Errorf("failed to unmarshal routing data: %w", err)
	}

	r.routing = routingData.Routes
	return nil
}

func (r *Router) SaveRoutes() error {
	r.mu.RLock()
	routingData := RoutingData{Routes: r.routing}
	r.mu.RUnlock()

	data, err := json.MarshalIndent(routingData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal routing data: %w", err)
	}

	if err := os.WriteFile(r.routingFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write routing file: %w", err)
	}

	return nil
}

func (r *Router) GetContainerForHost(hostname string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hostname = strings.ToLower(strings.TrimSpace(hostname))

	if containerName, exists := r.routing[hostname]; exists {
		return containerName
	}

	defaultContainer := r.config.GetDefaultContainer()
	if defaultContainer != nil {
		return defaultContainer.Name
	}

	return ""
}

func (r *Router) GetContainerForHosts(hostnames []string) map[string]string {
	result := make(map[string]string)

	for _, hostname := range hostnames {
		containerName := r.GetContainerForHost(hostname)
		if containerName != "" {
			result[hostname] = containerName
		}
	}

	return result
}

func (r *Router) AddRoute(hostname, containerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	hostname = strings.ToLower(strings.TrimSpace(hostname))

	if hostname == "" {
		return fmt.Errorf("hostname cannot be empty")
	}

	if r.config.GetContainer(containerName) == nil {
		return fmt.Errorf("container %s not found in configuration", containerName)
	}

	r.routing[hostname] = containerName
	return r.SaveRoutes()
}

func (r *Router) RemoveRoute(hostname string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	hostname = strings.ToLower(strings.TrimSpace(hostname))

	if _, exists := r.routing[hostname]; !exists {
		return fmt.Errorf("route for hostname %s not found", hostname)
	}

	delete(r.routing, hostname)
	return r.SaveRoutes()
}

func (r *Router) GetAllRoutes() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range r.routing {
		result[k] = v
	}
	return result
}

func (r *Router) GetRoutesForContainer(containerName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var hostnames []string
	for hostname, container := range r.routing {
		if container == containerName {
			hostnames = append(hostnames, hostname)
		}
	}
	return hostnames
}

func (r *Router) ensureRoutingFile() error {
	if _, err := os.Stat(r.routingFile); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(r.routingFile), 0755); err != nil {
			return err
		}

		defaultRouting := RoutingData{
			Routes: map[string]string{
				"router1.example.com":      "annet",
				"switch1.example.com":      "annet",
				"old-router.example.com":   "annet-telnet",
				"legacy-switch.example.com": "annet-telnet",
			},
		}

		data, err := json.MarshalIndent(defaultRouting, "", "  ")
		if err != nil {
			return err
		}

		return os.WriteFile(r.routingFile, data, 0644)
	}
	return nil
}