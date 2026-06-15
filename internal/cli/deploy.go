package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"annet-oil/internal/annet"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy configurations",
	Long:  `Deploy configurations using annet containers with automatic routing`,
	RunE:  runDeployCommand,
}

var (
	deployFilters   []string
	deployContainer string
	deployDryRun    bool
	deployParallel  bool
	deployTimeout   int
	deployFormat    string
	deployQuiet     bool
)

func init() {
	deployCmd.Flags().StringSliceVarP(&deployFilters, "filters", "g", nil, "Host filters (can be used multiple times)")
	deployCmd.Flags().StringSliceVarP(&deployFilters, "group", "G", nil, "Group filters (alias for -g)")
	deployCmd.Flags().StringVar(&deployContainer, "container", "", "Force specific container")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Perform a dry run without applying changes")
	deployCmd.Flags().BoolVar(&deployParallel, "parallel", false, "Execute in parallel")
	deployCmd.Flags().IntVar(&deployTimeout, "timeout", 0, "Timeout in seconds")
	deployCmd.Flags().StringVar(&deployFormat, "format", "text", "Output format (text|json)")
	deployCmd.Flags().BoolVarP(&deployQuiet, "quiet", "q", false, "Suppress stderr warnings")
}

func runDeployCommand(cmd *cobra.Command, args []string) error {
	// args - это hostnames (позиционные аргументы)
	// deployFilters - это generator фильтры (-g)

	if len(args) == 0 && deployContainer == "" {
		return fmt.Errorf("at least one hostname must be specified")
	}

	req := &annet.CommandRequest{
		Command:    "deploy",
		Filters:    args,           // hostnames для маршрутизации
		Generators: deployFilters,  // generator фильтры (-g)
		Container:  deployContainer,
		DryRun:     deployDryRun,
		Parallel:   deployParallel,
		Timeout:    deployTimeout,
		Quiet:      deployQuiet,
	}

	resp, err := annetService.ExecuteCommand(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to execute deploy command: %w", err)
	}

	return printCommandResponse(resp, deployFormat)
}