package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"annet-oil/internal/annet"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show configuration differences",
	Long:  `Show configuration differences using annet containers with automatic routing`,
	RunE:  runDiffCommand,
}

var (
	diffFilters   []string
	diffContainer string
	diffParallel  bool
	diffTimeout   int
	diffFormat    string
	diffQuiet     bool
)

func init() {
	diffCmd.Flags().StringSliceVarP(&diffFilters, "filters", "g", nil, "Host filters (can be used multiple times)")
	diffCmd.Flags().StringSliceVarP(&diffFilters, "group", "G", nil, "Group filters (alias for -g)")
	diffCmd.Flags().StringVar(&diffContainer, "container", "", "Force specific container")
	diffCmd.Flags().BoolVar(&diffParallel, "parallel", false, "Execute in parallel")
	diffCmd.Flags().IntVar(&diffTimeout, "timeout", 0, "Timeout in seconds")
	diffCmd.Flags().StringVar(&diffFormat, "format", "text", "Output format (text|json)")
	diffCmd.Flags().BoolVarP(&diffQuiet, "quiet", "q", false, "Suppress stderr warnings")
}

func runDiffCommand(cmd *cobra.Command, args []string) error {
	// args are hostnames (positional arguments)
	// diffFilters are generator filters (-g)

	if len(args) == 0 && diffContainer == "" {
		return fmt.Errorf("at least one hostname must be specified")
	}

	req := &annet.CommandRequest{
		Command:    "diff",
		Filters:    args,         // hostnames for routing
		Generators: diffFilters,  // generator filters (-g)
		Container:  diffContainer,
		Parallel:   diffParallel,
		Timeout:    diffTimeout,
		Quiet:      diffQuiet,
	}

	resp, err := annetService.ExecuteCommand(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to execute diff command: %w", err)
	}

	return printCommandResponse(resp, diffFormat)
}