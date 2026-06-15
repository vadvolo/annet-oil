package container

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"annet-oil/internal/config"
)

type Manager struct {
	client *client.Client
	config *config.Config
}

type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

func New(cfg *config.Config) (*Manager, error) {
	var clientOpts []client.Opt

	// Добавляем базовые опции
	clientOpts = append(clientOpts, client.FromEnv, client.WithAPIVersionNegotiation())

	// Если указан кастомный host в конфигурации
	if cfg.Docker.Host != "" {
		clientOpts = append(clientOpts, client.WithHost(cfg.Docker.Host))
	}

	// Если указана API версия
	if cfg.Docker.APIVersion != "" {
		clientOpts = append(clientOpts, client.WithVersion(cfg.Docker.APIVersion))
	}

	// TLS настройки
	if cfg.Docker.TLSVerify {
		clientOpts = append(clientOpts, client.WithTLSClientConfig(cfg.Docker.CertPath, cfg.Docker.CertPath, cfg.Docker.CertPath))
	}

	cli, err := client.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Manager{
		client: cli,
		config: cfg,
	}, nil
}

func (m *Manager) Close() error {
	return m.client.Close()
}

func (m *Manager) IsContainerRunning(ctx context.Context, containerName string) (bool, error) {
	containers, err := m.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range containers {
		for _, name := range c.Names {
			if strings.TrimPrefix(name, "/") == containerName && c.State == "running" {
				return true, nil
			}
		}
	}

	return false, nil
}

func (m *Manager) GetContainerInfo(ctx context.Context, containerName string) (*types.Container, error) {
	containers, err := m.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range containers {
		for _, name := range c.Names {
			if strings.TrimPrefix(name, "/") == containerName {
				return &c, nil
			}
		}
	}

	return nil, fmt.Errorf("container %s not found", containerName)
}

func (m *Manager) ExecuteAnnetCommand(ctx context.Context, containerName string, command []string) (*ExecResult, error) {
	containerInfo, err := m.GetContainerInfo(ctx, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get container info: %w", err)
	}

	if containerInfo.State != "running" {
		return nil, fmt.Errorf("container %s is not running (state: %s)", containerName, containerInfo.State)
	}

	annetCommand := append([]string{"annet"}, command...)

	execConfig := container.ExecOptions{
		Cmd:          annetCommand,
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := m.client.ContainerExecCreate(ctx, containerInfo.ID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := m.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	stdout, stderr, err := m.readExecOutput(attachResp.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read exec output: %w", err)
	}

	inspectResp, err := m.client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect exec: %w", err)
	}

	return &ExecResult{
		ExitCode: inspectResp.ExitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}, nil
}

func (m *Manager) ValidateContainerAccess(ctx context.Context, containerName string) error {
	// Используем --help вместо --version, так как annet не поддерживает --version
	result, err := m.ExecuteAnnetCommand(ctx, containerName, []string{"--help"})
	if err != nil {
		return fmt.Errorf("failed to validate container %s: %w", containerName, err)
	}

	// --help возвращает exit code 0 и показывает справку
	if result.ExitCode != 0 {
		return fmt.Errorf("annet command failed in container %s: %s", containerName, result.Stderr)
	}

	// Проверяем что в выводе есть ключевые слова annet
	if !strings.Contains(result.Stdout, "annet") && !strings.Contains(result.Stderr, "annet") {
		return fmt.Errorf("annet command output doesn't contain expected content in container %s", containerName)
	}

	return nil
}

func (m *Manager) GetAllConfiguredContainers(ctx context.Context) (map[string]*ContainerStatus, error) {
	result := make(map[string]*ContainerStatus)

	for _, annetContainer := range m.config.AnnetContainers {
		status := &ContainerStatus{
			Name:          annetContainer.Name,
			ContainerName: annetContainer.ContainerName,
			Description:   annetContainer.Description,
			Default:       annetContainer.Default,
		}

		isRunning, err := m.IsContainerRunning(ctx, annetContainer.ContainerName)
		if err != nil {
			status.Status = "error"
			status.Error = err.Error()
		} else if isRunning {
			status.Status = "running"
			if err := m.ValidateContainerAccess(ctx, annetContainer.ContainerName); err != nil {
				status.Status = "unhealthy"
				status.Error = err.Error()
			} else {
				status.Status = "healthy"
			}
		} else {
			status.Status = "stopped"
		}

		result[annetContainer.Name] = status
	}

	return result, nil
}

func (m *Manager) readExecOutput(reader io.Reader) (string, string, error) {
	var stdout, stderr strings.Builder

	buf := make([]byte, 8)
	for {
		n, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", "", err
		}

		if n < 8 {
			continue
		}

		streamType := buf[0]
		payloadSize := int(buf[4])<<24 | int(buf[5])<<16 | int(buf[6])<<8 | int(buf[7])

		if payloadSize > 0 {
			payload := make([]byte, payloadSize)
			_, err = io.ReadFull(reader, payload)
			if err != nil {
				return "", "", err
			}

			switch streamType {
			case 1:
				stdout.Write(payload)
			case 2:
				stderr.Write(payload)
			}
		}
	}

	return stdout.String(), stderr.String(), nil
}

type ContainerStatus struct {
	Name          string `json:"name"`
	ContainerName string `json:"container_name"`
	Description   string `json:"description,omitempty"`
	Default       bool   `json:"default"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
}