package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"annet-oil/internal/audit"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Query the audit trail",
	Long: `List recorded audit events (who ran which command on which devices, from
where, and whether it succeeded). Requires audit.enabled=true with a configured
PostgreSQL database.`,
	RunE: runAuditCommand,
}

var (
	auditActor  string
	auditDevice string
	auditAction string
	auditSource string
	auditLimit  int
	auditOffset int
	auditFormat string
)

func init() {
	auditCmd.Flags().StringVar(&auditActor, "actor", "", "Filter by actor")
	auditCmd.Flags().StringVar(&auditDevice, "device", "", "Filter by device (hostname)")
	auditCmd.Flags().StringVar(&auditAction, "action", "", "Filter by action (gen|diff|patch|deploy|execute|check|…)")
	auditCmd.Flags().StringVar(&auditSource, "source", "", "Filter by source (api|cli|ssh|mcp)")
	auditCmd.Flags().IntVar(&auditLimit, "limit", 50, "Max events to return")
	auditCmd.Flags().IntVar(&auditOffset, "offset", 0, "Pagination offset")
	auditCmd.Flags().StringVar(&auditFormat, "format", "text", "Output format (text|json)")

	rootCmd.AddCommand(auditCmd)
}

func runAuditCommand(cmd *cobra.Command, args []string) error {
	if auditRecorder == nil {
		return fmt.Errorf("audit recorder unavailable")
	}

	f := audit.Filter{
		Actor:  auditActor,
		Device: auditDevice,
		Action: auditAction,
		Source: auditSource,
		Limit:  auditLimit,
		Offset: auditOffset,
	}

	events, total, err := auditRecorder.List(cmd.Context(), f)
	if err != nil {
		return fmt.Errorf("failed to list audit events: %w", err)
	}

	if auditFormat == "json" {
		out := map[string]any{"events": events, "total": total}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Audit events (showing %d of %d):\n\n", len(events), total)
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tACTOR\tSOURCE\tACTION\tOK\tDEVICES")
	for _, e := range events {
		ok := "ok"
		if !e.Success {
			ok = "FAIL"
		}
		devices := ""
		for i, d := range e.Devices {
			if i > 0 {
				devices += ","
			}
			devices += d
			if i == 2 && len(e.Devices) > 3 {
				devices += fmt.Sprintf("…(+%d)", len(e.Devices)-3)
				break
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			e.Timestamp.Local().Format(time.RFC3339), e.Actor, e.Source, e.Action, ok, devices)
	}
	return w.Flush()
}
