package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"annet-oil/internal/container"
)

var containersCmd = &cobra.Command{
	Use:   "containers",
	Short: "Container management commands",
	Long:  `Commands for managing annet containers`,
}

var containersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured containers",
	Long:  `List all configured annet containers with their status`,
	RunE:  runContainersListCommand,
}

var (
	containersFormat string
)

func init() {
	containersCmd.AddCommand(containersListCmd)

	containersListCmd.Flags().StringVar(&containersFormat, "format", "table", "Output format (table|json)")
}

func runContainersListCommand(cmd *cobra.Command, args []string) error {
	status, err := annetService.GetContainerStatus(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get container status: %w", err)
	}

	switch containersFormat {
	case "json":
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal status: %w", err)
		}
		fmt.Print(string(data))
	case "table":
	default:
		printContainersTable(status)
	}

	return nil
}

func printContainersTable(status map[string]*container.ContainerStatus) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tCONTAINER\tSTATUS\tDEFAULT\tDESCRIPTION\tERROR")

	for _, container := range status {
		defaultMark := ""
		if container.Default {
			defaultMark = "✓"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			container.Name,
			container.ContainerName,
			container.Status,
			defaultMark,
			container.Description,
			container.Error)
	}

	w.Flush()
}