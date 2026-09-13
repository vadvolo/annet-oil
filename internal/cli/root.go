package cli

import (
	"context"
	"fmt"
	"os"
	osuser "os/user"

	"github.com/spf13/cobra"

	"annet-oil/internal/annet"
	"annet-oil/internal/audit"
	"annet-oil/internal/checkeast"
	"annet-oil/internal/config"
	"annet-oil/internal/container"
	"annet-oil/internal/featureset"
	"annet-oil/internal/inventory"
	"annet-oil/internal/logging"
	"annet-oil/internal/router"
)

var configPath string

var (
	cfg            *config.Config
	annetService   *annet.Service
	containerMgr   *container.Manager
	routerInstance *router.Router
	s3Uploader     *logging.S3Uploader
	checkeastStore *checkeast.Store
	auditRecorder  audit.Recorder
)

// cliContext augments a command context with the CLI actor identity so audit
// events attribute CLI actions to the local OS user. When invoked as a
// subprocess by the SSH server (which shells out to this binary), the SSH
// identity is propagated via ANNET_OIL_AUDIT_ACTOR/SOURCE env vars and takes
// precedence, so SSH-originated actions are attributed to the remote peer.
func cliContext(ctx context.Context) context.Context {
	if actor := os.Getenv("ANNET_OIL_AUDIT_ACTOR"); actor != "" {
		source := os.Getenv("ANNET_OIL_AUDIT_SOURCE")
		if source == "" {
			source = audit.SourceSSH
		}
		return audit.WithActor(ctx, actor, "", source)
	}

	name := os.Getenv("USER")
	if u, err := osuser.Current(); err == nil && u.Username != "" {
		name = u.Username
	}
	if name == "" {
		name = "cli"
	}
	return audit.WithActor(ctx, name, "", audit.SourceCLI)
}

var rootCmd = &cobra.Command{
	Use:   "annet-oil",
	Short: "Annet Oil - wrapper for multiple annet containers orchestration",
	Long: `Annet Oil is a Go-based wrapper that orchestrates commands across multiple annet containers.
It provides both CLI and REST API interfaces for managing annet gen, diff, patch, and deploy operations
with automatic container routing based on hostname patterns.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := loadConfigAndLogging(); err != nil {
			return err
		}
		return initializeServices()
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if s3Uploader != nil {
			s3Uploader.Stop()
		}
		if auditRecorder != nil {
			if err := auditRecorder.Close(); err != nil {
				logging.Warn("failed to close audit recorder", "error", err)
			}
		}
		if containerMgr != nil {
			return containerMgr.Close()
		}
		return nil
	},
}

func Execute(ctx context.Context) error {
	// Attribute all CLI actions to the local OS user for auditing. HTTP/SSH
	// entrypoints override this per request/session.
	return rootCmd.ExecuteContext(cliContext(ctx))
}

// loadConfigAndLogging loads the config, initializes logging and the S3
// uploader. It does not touch Docker, so lightweight commands (e.g. check) can
// reuse it without requiring a running daemon.
func loadConfigAndLogging() error {
	var err error
	cfg, err = config.LoadFrom(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if _, err := logging.Init(cfg.Logging); err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}

	s3Uploader, err = logging.NewS3Uploader(cfg.Logging)
	if err != nil {
		return fmt.Errorf("failed to initialize S3 uploader: %w", err)
	}
	if s3Uploader != nil {
		s3Uploader.Start()
	}

	auditRecorder, err = audit.NewRecorder(cfg.Audit)
	if err != nil {
		return fmt.Errorf("failed to initialize audit recorder: %w", err)
	}

	return nil
}

// loadInventory loads the inventory file if one is configured. A missing or
// unreadable inventory is logged as a warning rather than treated as fatal.
func loadInventory() {
	if cfg.Storage.InventoryFile == "" {
		return
	}
	if _, err := inventory.Load(cfg.Storage.InventoryFile); err != nil {
		logging.Warn("Failed to load inventory", "path", cfg.Storage.InventoryFile, "error", err)
	} else {
		logging.Info("Loaded inventory", "path", cfg.Storage.InventoryFile)
	}
}

// loadFeatureSets loads the feature-set knowledge base if one is configured. As
// with the inventory, a missing or unreadable file is a warning, not fatal.
func loadFeatureSets() {
	if cfg.Storage.FeatureSetFile == "" {
		return
	}
	if _, err := featureset.Load(cfg.Storage.FeatureSetFile); err != nil {
		logging.Warn("Failed to load feature-set knowledge base", "path", cfg.Storage.FeatureSetFile, "error", err)
	} else {
		logging.Info("Loaded feature-set knowledge base", "path", cfg.Storage.FeatureSetFile)
	}
}

func initializeServices() error {
	var err error

	containerMgr, err = container.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize container manager: %w", err)
	}

	routerInstance = router.New(cfg)
	if err := routerInstance.LoadRoutes(); err != nil {
		return fmt.Errorf("failed to load routes: %w", err)
	}

	loadInventory()
	loadFeatureSets()

	annetService = annet.New(cfg, containerMgr, routerInstance, auditRecorder)

	checkeastStore, err = checkeast.New(cfg.Checkeast.S3)
	if err != nil {
		return fmt.Errorf("failed to init checkeast store: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(genCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(patchCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(containersCmd)
	rootCmd.AddCommand(routingCmd)
	rootCmd.AddCommand(serverCmd)

	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path")
}

func printError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}
