package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AnnetContainers []AnnetContainer   `yaml:"annet_containers"`
	SSHKeys         []SSHKey           `yaml:"ssh_keys"`
	Server          ServerConfig       `yaml:"server"`
	Storage         StorageConfig      `yaml:"storage"`
	Docker          DockerConfig       `yaml:"docker"`
	Gnetcli         GnetcliConfig      `yaml:"gnetcli"`
	Logging         LoggingConfig      `yaml:"logging,omitempty"`
	Cache           CacheConfig        `yaml:"cache,omitempty"`
	Auth            AuthConfig         `yaml:"auth,omitempty"`
	Integrations    IntegrationsConfig `yaml:"integrations,omitempty"`
	Checkeast       CheckeastConfig    `yaml:"checkeast,omitempty"`
	Audit           AuditConfig        `yaml:"audit,omitempty"`
}

// S3StoreConfig configures an S3-compatible object store for archiving diffs.
// Credentials come from the standard AWS environment (AWS_ACCESS_KEY_ID / …),
// not from YAML. Set Endpoint to target MinIO or another S3-compatible service.
type S3StoreConfig struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	Bucket   string `yaml:"bucket,omitempty"`
	Prefix   string `yaml:"prefix,omitempty"` // e.g. "diffs/annet-oil/"
	Region   string `yaml:"region,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"` // optional: MinIO / S3-compatible
	// RetentionDays is how long archived diffs are kept before expiring, applied
	// as a bucket lifecycle rule scoped to Prefix. 0 uses the default (3 days);
	// a negative value disables lifecycle management entirely.
	RetentionDays int `yaml:"retention_days,omitempty"`
}

// CheckeastConfig configures the checkeast feature (run a diff, archive it to S3).
type CheckeastConfig struct {
	S3       S3StoreConfig  `yaml:"s3,omitempty"`
	Schedule ScheduleConfig `yaml:"schedule,omitempty"`
}

// ScheduleConfig drives cron-based checkeast runs over a scope of devices.
type ScheduleConfig struct {
	Enabled bool          `yaml:"enabled,omitempty"`
	Jobs    []ScheduleJob `yaml:"jobs,omitempty"`
}

// ScheduleJob is one cron entry: a schedule plus an inventory scope to diff.
type ScheduleJob struct {
	Name        string   `yaml:"name"`
	Cron        string   `yaml:"cron"`               // standard 5-field cron or @descriptor (e.g. "0 2 * * *", "@daily")
	Vendor      string   `yaml:"vendor,omitempty"`   // inventory filter (e.g. "cisco")
	Platform    string   `yaml:"platform,omitempty"` // inventory filter (e.g. "ios")
	Pattern     string   `yaml:"pattern,omitempty"`  // hostname/alias wildcard or substring
	ByAlias     bool     `yaml:"by_alias,omitempty"` // diff against the device IP/alias instead of hostname
	Concurrency int      `yaml:"concurrency,omitempty"`
	Timeout     int      `yaml:"timeout,omitempty"` // per-device diff timeout, seconds
	Generators  []string `yaml:"generators,omitempty"`
}

// AuditConfig configures the audit trail. When Enabled is false (the default)
// a no-op recorder is used and no database is required.
type AuditConfig struct {
	Enabled    bool   `yaml:"enabled,omitempty"`
	DSN        string `yaml:"dsn,omitempty"` // postgres://user:pass@host:port/db?sslmode=…
	Host       string `yaml:"host,omitempty"`
	Port       int    `yaml:"port,omitempty"`
	User       string `yaml:"user,omitempty"`
	Password   string `yaml:"password,omitempty"`
	Database   string `yaml:"database,omitempty"`
	SSLMode    string `yaml:"ssl_mode,omitempty"`
	BufferSize int    `yaml:"buffer_size,omitempty"` // async queue size (default 1024)
}

// DefaultRetentionDays is the retention applied to archived diffs when
// S3StoreConfig.RetentionDays is left at its zero value.
const DefaultRetentionDays = 3

// EffectiveRetentionDays resolves the configured retention: 0 → default (3),
// negative → 0 (lifecycle disabled), positive → as-is.
func (c S3StoreConfig) EffectiveRetentionDays() int {
	if c.RetentionDays == 0 {
		return DefaultRetentionDays
	}
	if c.RetentionDays < 0 {
		return 0
	}
	return c.RetentionDays
}

type IntegrationsConfig struct {
	Jira   JiraConfig   `yaml:"jira,omitempty"`
	GitHub GitHubConfig `yaml:"github,omitempty"`
}

type JiraConfig struct {
	Enabled    bool   `yaml:"enabled,omitempty"`
	URL        string `yaml:"url,omitempty"`
	Email      string `yaml:"email,omitempty"`
	Token      string `yaml:"token,omitempty"`
	ProjectKey string `yaml:"project_key,omitempty"`
	IssueType  string `yaml:"issue_type,omitempty"`
}

type GitHubConfig struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	Token   string `yaml:"token,omitempty"`
}

type LoggingConfig struct {
	Level  string      `yaml:"level,omitempty"`
	Format string      `yaml:"format,omitempty"`
	Output string      `yaml:"output,omitempty"`
	S3     S3LogConfig `yaml:"s3,omitempty"`
}

type S3LogConfig struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	Bucket  string `yaml:"bucket,omitempty"`
	Prefix  string `yaml:"prefix,omitempty"`
	Region  string `yaml:"region,omitempty"`
}

type CacheConfig struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	TTL     string `yaml:"ttl,omitempty"`
	MaxSize string `yaml:"max_size,omitempty"`
}

type AuthConfig struct {
	Roles  []RoleConfig  `yaml:"roles,omitempty"`
	Users  []UserConfig  `yaml:"users,omitempty"`
	Groups []GroupConfig `yaml:"groups,omitempty"`
}

type RoleConfig struct {
	Name         string   `yaml:"name"`
	DeviceScopes []string `yaml:"device_scopes,omitempty"`
	Methods      []string `yaml:"methods,omitempty"`
	Commands     []string `yaml:"commands,omitempty"`
}

type UserConfig struct {
	Name  string `yaml:"name"`
	Token string `yaml:"token"`
	Role  string `yaml:"role"`
}

type GroupConfig struct {
	Name    string   `yaml:"name"`
	Role    string   `yaml:"role"`
	Members []string `yaml:"members,omitempty"`
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
	// RequestTimeoutSec is the hard wall-clock cap the HTTP layer places on a
	// single request (chi Timeout middleware). It bounds the whole request
	// including slow device commands, so it must be >= gnetcli.max_timeout_sec
	// for large diagnostic outputs (e.g. Eltex MES "show logging") to return.
	// 0 falls back to DefaultRequestTimeoutSec.
	RequestTimeoutSec int `yaml:"request_timeout_sec,omitempty"`
}

// DefaultRequestTimeoutSec is used when APIConfig.RequestTimeoutSec is unset.
// Aligned with the example gnetcli.max_timeout_sec (120) so a per-request
// timeout_s override up to that cap can actually complete through the HTTP layer.
const DefaultRequestTimeoutSec = 120

type StorageConfig struct {
	RoutingFile   string `yaml:"routing_file"`
	InventoryFile string `yaml:"inventory_file,omitempty"`
	// FeatureSetFile points at the feature-set knowledge base (see
	// resources/featuresets.yaml). Optional; when empty the featureset API/CLI
	// report that no knowledge base is loaded.
	FeatureSetFile string `yaml:"featureset_file,omitempty"`
}

type DockerConfig struct {
	Host       string `yaml:"host,omitempty"`
	APIVersion string `yaml:"api_version,omitempty"`
	CertPath   string `yaml:"cert_path,omitempty"`
	TLSVerify  bool   `yaml:"tls_verify,omitempty"`
}

type GnetcliConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	AuthToken string `yaml:"auth_token,omitempty"`
	Login     string `yaml:"login"`
	Password  string `yaml:"password"`
	TLS       bool   `yaml:"tls,omitempty"`

	// Timeouts for a single device command, in seconds. These translate to the
	// gnetcli proto CMD read_timeout / cmd_timeout fields. read_timeout is the
	// gap allowed *between* reads (a stalled pager or slowly-streamed large
	// table trips this); cmd_timeout bounds the whole command. Zero means "do
	// not set" and inherit the gnetcli server default.
	ReadTimeoutSec float64 `yaml:"read_timeout_sec,omitempty"`
	CmdTimeoutSec  float64 `yaml:"cmd_timeout_sec,omitempty"`
	// MaxTimeoutSec caps a per-request timeout_s override. Zero disables the cap.
	MaxTimeoutSec float64 `yaml:"max_timeout_sec,omitempty"`
}

func Load() (*Config, error) {
	return LoadFrom("")
}

func LoadFrom(path string) (*Config, error) {
	if path == "" {
		path = getConfigPath()
	}
	configPath := path

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
			RoutingFile:    "./storage/routing.json",
			FeatureSetFile: "./resources/featuresets.yaml",
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
