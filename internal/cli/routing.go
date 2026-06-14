package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var routingCmd = &cobra.Command{
	Use:   "routing",
	Short: "Routing management commands",
	Long:  `Commands for managing hostname to container routing`,
}

var routingShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current routing",
	Long:  `Show current hostname to container routing`,
	RunE:  runRoutingShowCommand,
}

var routingAddCmd = &cobra.Command{
	Use:   "add [hostname] [container]",
	Short: "Add routing rule",
	Long:  `Add a new hostname to container routing rule`,
	Args:  cobra.ExactArgs(2),
	RunE:  runRoutingAddCommand,
}

var routingRemoveCmd = &cobra.Command{
	Use:   "remove [hostname]",
	Short: "Remove routing rule",
	Long:  `Remove a hostname to container routing rule`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRoutingRemoveCommand,
}

var (
	routingFormat string
)

func init() {
	routingCmd.AddCommand(routingShowCmd)
	routingCmd.AddCommand(routingAddCmd)
	routingCmd.AddCommand(routingRemoveCmd)

	routingShowCmd.Flags().StringVar(&routingFormat, "format", "table", "Output format (table|json)")
}

func runRoutingShowCommand(cmd *cobra.Command, args []string) error {
	routes := routerInstance.GetAllRoutes()

	switch routingFormat {
	case "json":
		data, err := json.MarshalIndent(routes, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal routes: %w", err)
		}
		fmt.Print(string(data))
	case "table":
	default:
		printRoutingTable(routes)
	}

	return nil
}

func runRoutingAddCommand(cmd *cobra.Command, args []string) error {
	hostname := args[0]
	container := args[1]

	if err := routerInstance.AddRoute(hostname, container); err != nil {
		return fmt.Errorf("failed to add route: %w", err)
	}

	fmt.Printf("Added routing: %s -> %s\n", hostname, container)
	return nil
}

func runRoutingRemoveCommand(cmd *cobra.Command, args []string) error {
	hostname := args[0]

	if err := routerInstance.RemoveRoute(hostname); err != nil {
		return fmt.Errorf("failed to remove route: %w", err)
	}

	fmt.Printf("Removed routing for: %s\n", hostname)
	return nil
}

func printRoutingTable(routes map[string]string) {
	if len(routes) == 0 {
		fmt.Println("No routing rules configured.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "HOSTNAME\tCONTAINER")

	for hostname, container := range routes {
		fmt.Fprintf(w, "%s\t%s\n", hostname, container)
	}

	w.Flush()
}