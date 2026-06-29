package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AnnetContainers []AnnetContainer `yaml:"annet_containers"`
	SSHKeys         []SSHKey         `yaml:"ssh_keys"`
	Server          ServerConfig     `yaml:"server"`
	Storage         StorageConfig    `yaml:"storage"`
	Docker          DockerConfig     `yaml:"docker"`
}

type AnnetContainer struct {
	Name          string `yaml:"name"`
	ContainerName string `yaml:"container_name"`
	Default       bool   `yaml:"default,omitempty"`
	Description   string `yaml:"description,omitempty"`
}

type SSHKey struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
	User string `yaml:"user"`
}

type ServerConfig struct {
	SSH SSHConfig `yaml:"ssh"`
	API APIConfig `yaml:"api"`
}

type SSHConfig struct {
	Port int    `yaml:"port"`
	Bind string `yaml:"bind"`
}

type APIConfig struct {
	Port      int    `yaml:"port"`
	Bind      string `yaml:"bind"`
	AuthToken string `yaml:"auth_token"`
}

type StorageConfig struct {
	RoutingFile string `yaml:"routing_file"`
}

type DockerConfig struct {
	Host    string `yaml:"host,omitempty"`
	APIVersion string `yaml:"api_version,omitempty"`
	CertPath   string `yaml:"cert_path,omitempty"`
	TLSVerify  bool   `yaml:"tls_verify,omitempty"`
}

func Load() (*Config, error) {
	configPath := getConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return createDefaultConfig(configPath)
		}
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func (c *Config) validate() error {
	if len(c.AnnetContainers) == 0 {
		return fmt.Errorf("at least one annet container must be configured")
	}

	defaultCount := 0
	for _, container := range c.AnnetContainers {
		if container.Name == "" {
			return fmt.Errorf("container name cannot be empty")
		}
		if container.ContainerName == "" {
			return fmt.Errorf("container_name cannot be empty for container %s", container.Name)
		}
		if container.Default {
			defaultCount++
		}
	}

	if defaultCount != 1 {
		return fmt.Errorf("exactly one container must be marked as default, found %d", defaultCount)
	}

	if c.Server.SSH.Port <= 0 {
		return fmt.Errorf("SSH port must be positive")
	}

	if c.Server.API.Port <= 0 {
		return fmt.Errorf("API port must be positive")
	}

	return nil
}

func (c *Config) GetDefaultContainer() *AnnetContainer {
	for _, container := range c.AnnetContainers {
		if container.Default {
			return &container
		}
	}
	return nil
}

func (c *Config) GetContainer(name string) *AnnetContainer {
	for _, container := range c.AnnetContainers {
		if container.Name == name {
			return &container
		}
	}
	return nil
}

func getConfigPath() string {
	if path := os.Getenv("ANNET_OIL_CONFIG"); path != "" {
		return path
	}

	if home, err := os.UserHomeDir(); err == nil {
		if path := filepath.Join(home, ".config", "annet-oil", "config.yaml"); fileExists(path) {
			return path
		}
	}

	return "./configs/config.yaml"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func createDefaultConfig(configPath string) (*Config, error) {
	defaultConfig := &Config{
		AnnetContainers: []AnnetContainer{
			{
				Name:          "annet",
				ContainerName: "annet-default",
				Default:       true,
				Description:   "Default annet container",
			},
			{
				Name:          "annet-telnet",
				ContainerName: "annet-telnet",
				Description:   "Telnet devices container",
			},
		},
		SSHKeys: []SSHKey{
			{
				Name: "default",
				Path: "/keys/id_rsa",
				User: "admin",
			},
		},
		Server: ServerConfig{
			SSH: SSHConfig{
				Port: 22,
				Bind: "0.0.0.0",
			},
			API: APIConfig{
				Port:      8080,
				Bind:      "0.0.0.0",
				AuthToken: "change-me-in-production",
			},
		},
		Storage: StorageConfig{
			RoutingFile: "./storage/routing.json",
		},
		Docker: DockerConfig{
			Host: "", // Empty value = auto-detect
		},
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, fmt.Errorf("error creating config directory: %w", err)
	}

	data, err := yaml.Marshal(defaultConfig)
	if err != nil {
		return nil, fmt.Errorf("error marshaling default config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("error writing default config: %w", err)
	}

	return defaultConfig, nil
}