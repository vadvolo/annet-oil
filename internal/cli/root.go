package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"annet-oil/internal/annet"
	"annet-oil/internal/config"
	"annet-oil/internal/container"
	"annet-oil/internal/router"
)

var configPath string

var (
	cfg             *config.Config
	annetService    *annet.Service
	containerMgr    *container.Manager
	routerInstance  *router.Router
)

var rootCmd = &cobra.Command{
	Use:   "annet-oil",
	Short: "Annet Oil - wrapper for multiple annet containers orchestration",
	Long: `Annet Oil is a Go-based wrapper that orchestrates commands across multiple annet containers.
It provides both CLI and REST API interfaces for managing annet gen, diff, patch, and deploy operations
with automatic container routing based on hostname patterns.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.LoadFrom(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		return initializeServices()
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if containerMgr != nil {
			return containerMgr.Close()
		}
		return nil
	},
}

func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
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

	annetService = annet.New(cfg, containerMgr, routerInstance)

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