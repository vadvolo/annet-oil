package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"annet-oil/internal/annet"
	"annet-oil/internal/checkeast"
)

var checkeastCmd = &cobra.Command{
	Use:   "checkeast [hostname ...]",
	Short: "Calculate a diff and archive it to S3",
	Long: `Calculate a configuration diff for one or more devices (exactly what 'diff'
does) and archive the result to S3-compatible storage: a combined JSON of the
whole run plus one raw .diff file per host. Archived diffs expire after the
configured retention (default 3 days).`,
	RunE: runCheckeastCommand,
}

var (
	checkeastFilters   []string
	checkeastContainer string
	checkeastParallel  bool
	checkeastTimeout   int
	checkeastFormat    string
	checkeastQuiet     bool
)

func init() {
	checkeastCmd.Flags().StringSliceVarP(&checkeastFilters, "filters", "g", nil, "Generator filters (-g)")
	checkeastCmd.Flags().StringVar(&checkeastContainer, "container", "", "Force specific container")
	checkeastCmd.Flags().BoolVar(&checkeastParallel, "parallel", false, "Execute in parallel")
	checkeastCmd.Flags().IntVar(&checkeastTimeout, "timeout", 0, "Timeout in seconds")
	checkeastCmd.Flags().StringVar(&checkeastFormat, "format", "text", "Output format (text|json)")
	checkeastCmd.Flags().BoolVarP(&checkeastQuiet, "quiet", "q", false, "Suppress stderr warnings")

	rootCmd.AddCommand(checkeastCmd)
}

func runCheckeastCommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && checkeastContainer == "" {
		return fmt.Errorf("at least one hostname must be specified")
	}

	req := &annet.CommandRequest{
		Command:    "diff",
		Filters:    args,             // hostnames for routing
		Generators: checkeastFilters, // generator filters (-g)
		Container:  checkeastContainer,
		Parallel:   checkeastParallel,
		Timeout:    checkeastTimeout,
		Quiet:      checkeastQuiet,
	}

	resp, err := annetService.ExecuteCommand(cmd.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to execute diff command: %w", err)
	}

	if err := printCommandResponse(resp, checkeastFormat); err != nil {
		return err
	}

	if checkeastStore == nil {
		fmt.Fprintln(os.Stderr, "S3 archival disabled (checkeast.s3.enabled=false)")
		return nil
	}

	report, _ := checkeastStore.Archive(cmd.Context(), req, resp)
	printCheckeastReport(report)
	return nil
}

func printCheckeastReport(report *checkeast.Report) {
	if report == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "\nArchived to S3 (bucket=%s, run=%s): stored=%d failed=%d\n",
		report.Bucket, report.RunID, report.StoredHosts, report.FailedHosts)
	if report.Combined != nil && report.Combined.Location != "" {
		fmt.Fprintf(os.Stderr, "  combined: %s\n", report.Combined.Location)
	}
	for host, art := range report.PerHost {
		if art == nil {
			continue
		}
		note := ""
		if art.Error != nil {
			note = fmt.Sprintf(" (%s: %s)", art.Error.Type, art.Error.Message)
		}
		fmt.Fprintf(os.Stderr, "  %s: %s%s\n", host, art.Location, note)
	}
	if report.Error != nil {
		fmt.Fprintf(os.Stderr, "  archive error: %s: %s\n", report.Error.Type, report.Error.Message)
	}
}
