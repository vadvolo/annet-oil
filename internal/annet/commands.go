package annet

import (
	"context"
	"fmt"
	"strings"

	"annet-oil/internal/config"
	"annet-oil/internal/container"
	"annet-oil/internal/router"
)

type Service struct {
	config          *config.Config
	containerManager *container.Manager
	router          *router.Router
}

type CommandRequest struct {
	Command     string            `json:"command"`
	Filters     []string          `json:"filters,omitempty"`
	Container   string            `json:"container,omitempty"`
	DryRun      bool              `json:"dry_run,omitempty"`
	Parallel    bool              `json:"parallel,omitempty"`
	Timeout     int               `json:"timeout,omitempty"`
	ExtraArgs   []string          `json:"extra_args,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

type CommandResponse struct {
	Success     bool                      `json:"success"`
	Results     map[string]*CommandResult `json:"results,omitempty"`
	Error       string                    `json:"error,omitempty"`
	TotalHosts  int                       `json:"total_hosts"`
	SuccessHosts int                      `json:"success_hosts"`
	FailedHosts int                       `json:"failed_hosts"`
}

type CommandResult struct {
	Container string `json:"container"`
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Error     string `json:"error,omitempty"`
	Duration  string `json:"duration,omitempty"`
}

func New(cfg *config.Config, containerMgr *container.Manager, router *router.Router) *Service {
	return &Service{
		config:          cfg,
		containerManager: containerMgr,
		router:          router,
	}
}

func (s *Service) ExecuteCommand(ctx context.Context, req *CommandRequest) (*CommandResponse, error) {
	if err := s.validateCommand(req); err != nil {
		return &CommandResponse{
			Success: false,
			Error:   fmt.Sprintf("command validation failed: %v", err),
		}, nil
	}

	containerRoutes := s.determineContainerRoutes(req)
	if len(containerRoutes) == 0 {
		return &CommandResponse{
			Success: false,
			Error:   "no valid containers found for the specified filters",
		}, nil
	}

	results := make(map[string]*CommandResult)
	totalHosts := 0
	successHosts := 0
	failedHosts := 0

	for containerName, hosts := range containerRoutes {
		totalHosts += len(hosts)

		cmdArgs := s.buildCommandArgs(req, hosts)

		execResult, err := s.containerManager.ExecuteAnnetCommand(ctx, containerName, cmdArgs)
		if err != nil {
			for _, host := range hosts {
				results[host] = &CommandResult{
					Container: containerName,
					ExitCode:  -1,
					Error:     fmt.Sprintf("container execution failed: %v", err),
				}
				failedHosts++
			}
			continue
		}

		for _, host := range hosts {
			result := &CommandResult{
				Container: containerName,
				ExitCode:  execResult.ExitCode,
				Stdout:    execResult.Stdout,
				Stderr:    execResult.Stderr,
			}

			if execResult.ExitCode == 0 {
				successHosts++
			} else {
				failedHosts++
				if result.Error == "" && result.Stderr != "" {
					result.Error = result.Stderr
				}
			}

			results[host] = result
		}
	}

	return &CommandResponse{
		Success:     failedHosts == 0,
		Results:     results,
		TotalHosts:  totalHosts,
		SuccessHosts: successHosts,
		FailedHosts: failedHosts,
	}, nil
}

func (s *Service) validateCommand(req *CommandRequest) error {
	validCommands := map[string]bool{
		"gen":    true,
		"diff":   true,
		"patch":  true,
		"deploy": true,
	}

	if !validCommands[req.Command] {
		return fmt.Errorf("invalid command: %s. Valid commands: gen, diff, patch, deploy", req.Command)
	}

	if req.Container != "" {
		if container := s.config.GetContainer(req.Container); container == nil {
			return fmt.Errorf("container %s not found in configuration", req.Container)
		}
	}

	return nil
}

func (s *Service) determineContainerRoutes(req *CommandRequest) map[string][]string {
	containerRoutes := make(map[string][]string)

	if req.Container != "" {
		if len(req.Filters) > 0 {
			containerRoutes[req.Container] = req.Filters
		} else {
			containerRoutes[req.Container] = []string{"all"}
		}
		return containerRoutes
	}

	if len(req.Filters) == 0 {
		defaultContainer := s.config.GetDefaultContainer()
		if defaultContainer != nil {
			containerRoutes[defaultContainer.Name] = []string{"all"}
		}
		return containerRoutes
	}

	hostContainerMap := s.router.GetContainerForHosts(req.Filters)

	for hostname, containerName := range hostContainerMap {
		if containerRoutes[containerName] == nil {
			containerRoutes[containerName] = make([]string, 0)
		}
		containerRoutes[containerName] = append(containerRoutes[containerName], hostname)
	}

	for _, filter := range req.Filters {
		found := false
		for _, containerName := range hostContainerMap {
			if containerName != "" {
				found = true
				break
			}
		}

		if !found {
			defaultContainer := s.config.GetDefaultContainer()
			if defaultContainer != nil {
				if containerRoutes[defaultContainer.Name] == nil {
					containerRoutes[defaultContainer.Name] = make([]string, 0)
				}
				containerRoutes[defaultContainer.Name] = append(containerRoutes[defaultContainer.Name], filter)
			}
		}
	}

	return containerRoutes
}

func (s *Service) buildCommandArgs(req *CommandRequest, hosts []string) []string {
	args := []string{req.Command}

	if len(hosts) > 0 && hosts[0] != "all" {
		args = append(args, "-g", strings.Join(hosts, ","))
	}

	if req.DryRun && (req.Command == "patch" || req.Command == "deploy") {
		args = append(args, "--dry-run")
	}

	if req.Parallel {
		args = append(args, "--parallel")
	}

	if req.Timeout > 0 {
		args = append(args, "--timeout", fmt.Sprintf("%d", req.Timeout))
	}

	args = append(args, req.ExtraArgs...)

	return args
}

func (s *Service) GetContainerStatus(ctx context.Context) (map[string]*container.ContainerStatus, error) {
	return s.containerManager.GetAllConfiguredContainers(ctx)
}