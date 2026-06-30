package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type ExtendedHealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Service   string                 `json:"service"`
	Gnetcli   *GnetcliStatus         `json:"gnetcli,omitempty"`
	Checks    map[string]CheckStatus `json:"checks"`
}

type GnetcliStatus struct {
	Enabled     bool   `json:"enabled"`
	Running     bool   `json:"running"`
	GrpcPort    string `json:"grpc_port"`
	HttpPort    string `json:"http_port"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	ServiceInfo string `json:"service_info,omitempty"`
}

type CheckStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ExtendedHealthHandler provides comprehensive health status including gnetcli
func ExtendedHealthHandler(w http.ResponseWriter, r *http.Request) {
	response := ExtendedHealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Service:   "annet-oil",
		Checks:    make(map[string]CheckStatus),
	}

	// Check gnetcli service status
	gnetcliStatus := checkGnetcliStatus()
	response.Gnetcli = gnetcliStatus

	// Overall health status
	if gnetcliStatus != nil && !gnetcliStatus.Running {
		response.Status = "degraded"
		response.Checks["gnetcli"] = CheckStatus{
			Status:  "unhealthy",
			Message: gnetcliStatus.Error,
		}
	} else if gnetcliStatus != nil {
		response.Checks["gnetcli"] = CheckStatus{
			Status:  "healthy",
			Message: "gnetcli service is running",
		}
	}

	// Check API connectivity
	response.Checks["api"] = CheckStatus{
		Status:  "healthy",
		Message: "API is responsive",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func checkGnetcliStatus() *GnetcliStatus {
	status := &GnetcliStatus{
		GrpcPort: "443",
		HttpPort: "50052",
	}

	// Check if systemd service exists and is enabled
	enabledCmd := exec.Command("systemctl", "is-enabled", "gnetcli")
	enabledOut, _ := enabledCmd.Output()
	status.Enabled = strings.TrimSpace(string(enabledOut)) == "enabled"

	// Check if systemd service is running
	runningCmd := exec.Command("systemctl", "is-active", "gnetcli")
	runningOut, _ := runningCmd.Output()
	status.Running = strings.TrimSpace(string(runningOut)) == "active"

	// Get detailed service status
	statusCmd := exec.Command("systemctl", "status", "gnetcli", "--no-pager", "-n", "0")
	statusOut, err := statusCmd.Output()
	if err == nil {
		lines := strings.Split(string(statusOut), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Active:") {
				status.ServiceInfo = strings.TrimSpace(line)
				break
			}
		}
	}

	if status.Running {
		// Try to connect to gRPC health check endpoint
		if err := checkGrpcHealth("localhost:" + status.GrpcPort); err != nil {
			status.Status = "running_with_issues"
			status.Error = fmt.Sprintf("gRPC health check failed: %v", err)
		} else {
			status.Status = "healthy"
		}
	} else {
		status.Status = "stopped"
		if !status.Enabled {
			status.Error = "Service is not enabled"
		} else {
			status.Error = "Service is not running"
		}
	}

	return status
}

func checkGrpcHealth(address string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("service not serving: %s", resp.Status)
	}

	return nil
}

// SimpleGnetcliCheck performs a basic check without gRPC dependencies
func SimpleGnetcliCheck() *GnetcliStatus {
	status := &GnetcliStatus{
		GrpcPort: "443",
		HttpPort: "50052",
	}

	// Check if systemd service is running
	runningCmd := exec.Command("systemctl", "is-active", "gnetcli")
	runningOut, _ := runningCmd.Output()
	status.Running = strings.TrimSpace(string(runningOut)) == "active"

	// Check if systemd service is enabled
	enabledCmd := exec.Command("systemctl", "is-enabled", "gnetcli")
	enabledOut, _ := enabledCmd.Output()
	status.Enabled = strings.TrimSpace(string(enabledOut)) == "enabled"

	// Try basic TCP connection to ports
	if status.Running {
		// Check gRPC port
		conn, err := net.DialTimeout("tcp", "localhost:"+status.GrpcPort, 2*time.Second)
		if err == nil {
			conn.Close()
			status.Status = "healthy"
		} else {
			status.Status = "running_with_issues"
			status.Error = fmt.Sprintf("Cannot connect to gRPC port %s", status.GrpcPort)
		}
	} else {
		status.Status = "stopped"
		if !status.Enabled {
			status.Error = "Service is not enabled"
		} else {
			status.Error = "Service is not running"
		}
	}

	return status
}
