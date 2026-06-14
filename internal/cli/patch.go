package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"annet-oil/internal/annet"
)

var patchCmd = &cobra.Command{
	Use:   "patch",
	Short: "Apply configuration patches",
	Long:  `Apply configuration patches using annet containers with automatic routing`,
	RunE:  runPatchCommand,
}

var (
	patchFilters   []string
	patchContainer string
	patchDryRun    bool
	patchParallel  bool
	patchTimeout   int
	patchFormat    string
)

func init() {
	patchCmd.Flags().StringSliceVarP(&patchFilters, "filters", "g", nil, "Host filters (can be used multiple times)")
	patchCmd.Flags().StringSliceVarP(&patchFilters, "group", "G", nil, "Group filters (alias for -g)")
	patchCmd.Flags().StringVar(&patchContainer, "container", "", "Force specific container")
	patchCmd.Flags().BoolVar(&patchDryRun, "dry-run", false, "Perform a dry run without applying changes")
	patchCmd.Flags().BoolVar(&patchParallel, "parallel", false, "Execute in parallel")
	patchCmd.Flags().IntVar(&patchTimeout, "timeout", 0, "Timeout in seconds")
	patchCmd.Flags().StringVar(&patchFormat, "format", "text", "Output format (text|json)")
}

func runPatchCommand(cmd *cobra.Command, args []string) error {
	filters := append(patchFilters, args...)
	if len(filters) == 0 {
		return fmt.Errorf("at least one filter must be specified")
	}

	req := &annet.CommandRequest{
		Command:   "patch",
		Filters:   filters,
		Container: patchContainer,
		DryRun:    patchDryRun,
		Parallel:  patchParallel,
		Timeout:   patchTimeout,
	}

	resp, err := annetService.ExecuteCommand(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to execute patch command: %w", err)
	}

	return printCommandResponse(resp, patchFormat)
}