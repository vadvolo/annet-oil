package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"annet-oil/internal/annet"
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate configurations",
	Long:  `Generate configurations using annet containers with automatic routing`,
	RunE:  runGenCommand,
}

var (
	genFilters   []string
	genContainer string
	genParallel  bool
	genTimeout   int
	genFormat    string
	genQuiet     bool
)

func init() {
	genCmd.Flags().StringSliceVarP(&genFilters, "filters", "g", nil, "Host filters (can be used multiple times)")
	genCmd.Flags().StringSliceVarP(&genFilters, "group", "G", nil, "Group filters (alias for -g)")
	genCmd.Flags().StringVar(&genContainer, "container", "", "Force specific container")
	genCmd.Flags().BoolVar(&genParallel, "parallel", false, "Execute in parallel")
	genCmd.Flags().IntVar(&genTimeout, "timeout", 0, "Timeout in seconds")
	genCmd.Flags().StringVar(&genFormat, "format", "text", "Output format (text|json)")
	genCmd.Flags().BoolVarP(&genQuiet, "quiet", "q", false, "Suppress stderr warnings")
}

func runGenCommand(cmd *cobra.Command, args []string) error {
	// args - это hostnames (позиционные аргументы)
	// genFilters - это generator фильтры (-g)

	if len(args) == 0 && genContainer == "" {
		return fmt.Errorf("at least one hostname must be specified")
	}

	req := &annet.CommandRequest{
		Command:    "gen",
		Filters:    args,        // hostnames для маршрутизации
		Generators: genFilters,  // generator фильтры (-g)
		Container:  genContainer,
		Parallel:   genParallel,
		Timeout:    genTimeout,
		Quiet:      genQuiet,
	}

	resp, err := annetService.ExecuteCommand(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to execute gen command: %w", err)
	}

	return printCommandResponse(resp, genFormat)
}

func printCommandResponse(resp *annet.CommandResponse, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		fmt.Print(string(data))
	case "text":
		fallthrough
	default:
		printTextResponse(resp)
	}
	return nil
}

func printTextResponse(resp *annet.CommandResponse) {
	if !resp.Success {
		fmt.Printf("Command failed: %s\n", resp.Error)
		return
	}

	fmt.Printf("Command completed successfully\n")
	fmt.Printf("Total hosts: %d, Success: %d, Failed: %d\n\n",
		resp.TotalHosts, resp.SuccessHosts, resp.FailedHosts)

	for hostname, result := range resp.Results {
		fmt.Printf("=== %s ===\n", hostname)
		fmt.Printf("Container: %s\n", result.Container)
		fmt.Printf("Exit code: %d\n", result.ExitCode)

		if result.Error != "" {
			fmt.Printf("Error: %s\n", result.Error)
		}

		if result.Stdout != "" {
			fmt.Printf("Output:\n%s\n", strings.TrimSpace(result.Stdout))
		}

		if result.Stderr != "" && result.Error == "" {
			fmt.Printf("Stderr:\n%s\n", strings.TrimSpace(result.Stderr))
		}

		fmt.Println()
	}
}