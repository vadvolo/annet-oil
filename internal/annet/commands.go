package annet

import (
	"context"
	"fmt"

	"annet-oil/internal/config"
	"annet-oil/internal/container"
	"annet-oil/internal/router"
)

type Service struct {
	config           *config.Config
	containerManager *container.Manager
	router           *router.Router
}

type CommandRequest struct {
	Command           string            `json:"command"`
	Filters           []string          `json:"filters,omitempty"`            // Hostnames for routing
	Generators        []string          `json:"generators,omitempty"`         // Generator filters (-g)
	ExcludeGenerators []string          `json:"exclude_generators,omitempty"` // Exclude generators (-G)
	Container         string            `json:"container,omitempty"`
	DryRun            bool              `json:"dry_run,omitempty"`
	Parallel          bool              `json:"parallel,omitempty"`
	Timeout           int               `json:"timeout,omitempty"`
	Quiet             bool              `json:"quiet,omitempty"` // Suppress stderr warnings
	ExtraArgs         []string          `json:"extra_args,omitempty"`
	Environment       map[string]string `json:"environment,omitempty"`
}

type CommandResponse struct {
	Success      bool                      `json:"success"`
	Results      map[string]*CommandResult `json:"results,omitempty"`
	Error        string                    `json:"error,omitempty"`
	TotalHosts   int                       `json:"total_hosts"`
	SuccessHosts int                       `json:"success_hosts"`
	FailedHosts  int                       `json:"failed_hosts"`
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
		config:           cfg,
		containerManager: containerMgr,
		router:           router,
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

			// Suppress stderr if quiet flag is set
			if req.Quiet {
				result.Stderr = ""
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
		Success:      failedHosts == 0,
		Results:      results,
		TotalHosts:   totalHosts,
		SuccessHosts: successHosts,
		FailedHosts:  failedHosts,
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
		// Если контейнер указан явно, используем его
		containerRoutes[req.Container] = []string{"all"}
		return containerRoutes
	}

	if len(req.Filters) == 0 {
		// Если фильтры не указаны, используем default контейнер
		defaultContainer := s.config.GetDefaultContainer()
		if defaultContainer != nil {
			containerRoutes[defaultContainer.Name] = []string{"all"}
		}
		return containerRoutes
	}

	// Фильтры - это имена устройств для маршрутизации
	hostContainerMap := s.router.GetContainerForHosts(req.Filters)

	for hostname, containerName := range hostContainerMap {
		if containerRoutes[containerName] == nil {
			containerRoutes[containerName] = make([]string, 0)
		}
		containerRoutes[containerName] = append(containerRoutes[containerName], hostname)
	}

	// Для hostname без маршрутов используем default контейнер
	for _, filter := range req.Filters {
		// Проверяем, есть ли маршрут для этого hostname
		if _, found := hostContainerMap[filter]; !found {
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

	// Добавляем generator фильтры если указаны
	if len(req.Generators) > 0 {
		for _, generator := range req.Generators {
			args = append(args, "-g", generator)
		}
	}

	// Добавляем exclude generator фильтры если указаны
	if len(req.ExcludeGenerators) > 0 {
		for _, excludeGen := range req.ExcludeGenerators {
			args = append(args, "-G", excludeGen)
		}
	}

	if req.DryRun && (req.Command == "patch" || req.Command == "deploy") {
		args = append(args, "--dry-run")
	}

	// Для команды deploy всегда добавляем --no-ask-deploy
	if req.Command == "deploy" {
		args = append(args, "--no-ask-deploy")
	}

	if req.Parallel {
		args = append(args, "--parallel")
	}

	if req.Timeout > 0 {
		args = append(args, "--timeout", fmt.Sprintf("%d", req.Timeout))
	}

	args = append(args, req.ExtraArgs...)

	// Добавляем query (hostnames) в конец - annet требует его
	if len(hosts) > 0 && hosts[0] != "all" {
		// Используем имена устройств как query
		args = append(args, hosts...)
	} else {
		// Если не указаны конкретные устройства, используем "*" (все)
		args = append(args, "*")
	}

	return args
}

func (s *Service) GetContainerStatus(ctx context.Context) (map[string]*container.ContainerStatus, error) {
	return s.containerManager.GetAllConfiguredContainers(ctx)
}
