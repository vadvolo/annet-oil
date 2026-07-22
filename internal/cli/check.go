package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"annet-oil/internal/check"
	"annet-oil/internal/inventory"
)

var checkCmd = &cobra.Command{
	Use:   "check [hostname|ip ...]",
	Short: "Check device availability (ports + SSH login)",
	Long: `Check network device availability across configured ports and, optionally,
verify SSH login.

Without positional arguments it checks every device in the inventory (subject to
--vendor/--platform/--pattern filters), running the probes in parallel batches.
With positional arguments it checks only the given hostnames/IPs.

The report can be written as JSON with --output for later inspection.`,
	// The check command only needs config + inventory, not Docker, so it
	// overrides the root PersistentPreRunE to avoid connecting to the daemon.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := loadConfigAndLogging(); err != nil {
			return err
		}
		loadInventory()
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if s3Uploader != nil {
			s3Uploader.Stop()
		}
		return nil
	},
	RunE: runCheckCommand,
}

var (
	checkPorts        []int
	checkVendor       string
	checkPlatform     string
	checkPattern      string
	checkConcurrency  int
	checkTimeout      int
	checkLoginTO      int
	checkNoLogin      bool
	checkOutput       string
	checkFormat       string
	checkFailuresOnly bool
)

func init() {
	checkCmd.Flags().IntSliceVar(&checkPorts, "ports", nil, "Extra ports to probe (device's own port is always included), e.g. --ports 22,23,10022")
	checkCmd.Flags().StringVar(&checkVendor, "vendor", "", "Filter inventory by vendor")
	checkCmd.Flags().StringVar(&checkPlatform, "platform", "", "Filter inventory by platform")
	checkCmd.Flags().StringVar(&checkPattern, "pattern", "", "Filter inventory by hostname pattern (supports wildcards)")
	checkCmd.Flags().IntVar(&checkConcurrency, "concurrency", 50, "Number of devices checked in parallel per batch")
	checkCmd.Flags().IntVar(&checkTimeout, "timeout", 3, "Per-port TCP dial timeout in seconds")
	checkCmd.Flags().IntVar(&checkLoginTO, "login-timeout", 5, "SSH login timeout in seconds")
	checkCmd.Flags().BoolVar(&checkNoLogin, "no-login", false, "Only check port reachability, skip SSH login")
	checkCmd.Flags().StringVarP(&checkOutput, "output", "o", "", "Write JSON availability report to this file")
	checkCmd.Flags().StringVar(&checkFormat, "format", "table", "Console output format (table|json|summary)")
	checkCmd.Flags().BoolVar(&checkFailuresOnly, "failures-only", false, "Only print unreachable / login-failed devices to console")

	rootCmd.AddCommand(checkCmd)
}

func runCheckCommand(cmd *cobra.Command, args []string) error {
	devices, err := resolveCheckDevices(args)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return fmt.Errorf("no devices to check (inventory empty or filters matched nothing)")
	}

	opts := check.Options{
		Ports:        checkPorts,
		DialTimeout:  time.Duration(checkTimeout) * time.Second,
		LoginTimeout: time.Duration(checkLoginTO) * time.Second,
		CheckLogin:   !checkNoLogin,
	}

	fmt.Fprintf(os.Stderr, "Checking %d device(s) with concurrency %d...\n", len(devices), checkConcurrency)
	report := check.Devices(cmd.Context(), devices, opts, checkConcurrency)

	if checkOutput != "" {
		if err := writeCheckReport(checkOutput, report); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Report written to %s\n", checkOutput)
	}

	return printCheckReport(report, checkFormat)
}

// resolveCheckDevices returns the devices to check: the explicit args (resolved
// through the inventory, falling back to a bare target) or the filtered inventory.
func resolveCheckDevices(args []string) ([]inventory.Device, error) {
	if len(args) > 0 {
		devices := make([]inventory.Device, 0, len(args))
		for _, host := range args {
			if dev, err := inventory.GetDevice(host); err == nil {
				devices = append(devices, *dev)
			} else {
				devices = append(devices, inventory.Device{Hostname: host, IP: host})
			}
		}
		return devices, nil
	}

	if inventory.GetInventory() == nil {
		return nil, fmt.Errorf("no inventory loaded; configure storage.inventory_file or pass hostnames as arguments")
	}
	return inventory.FilterDevices(checkVendor, checkPlatform, checkPattern), nil
}

func writeCheckReport(path string, report *check.BatchReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func printCheckReport(report *check.BatchReport, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case "summary":
		printCheckSummary(report)
	case "table":
		fallthrough
	default:
		printCheckTable(report)
		printCheckSummary(report)
	}
	return nil
}

func printCheckTable(report *check.BatchReport) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "HOSTNAME\tIP\tREACHABLE\tOPEN PORTS\tLOGIN\tERROR")

	for _, r := range report.Results {
		if checkFailuresOnly && r.OK() {
			continue
		}
		reachable := "no"
		if r.Reachable {
			reachable = "yes"
		}
		errMsg := ""
		if r.Error != nil {
			errMsg = fmt.Sprintf("%s: %s", r.Error.Type, r.Error.Message)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Hostname, r.IP, reachable, openPortsString(r), r.Login, errMsg)
	}
	w.Flush()
}

func openPortsString(r *check.Result) string {
	var open []string
	for _, p := range r.Ports {
		if p.Open {
			open = append(open, fmt.Sprint(p.Port))
		}
	}
	if len(open) == 0 {
		return "-"
	}
	return strings.Join(open, ",")
}

func printCheckSummary(report *check.BatchReport) {
	fmt.Printf("\nChecked %d device(s) in %dms (concurrency %d)\n",
		report.Total, report.DurationMs, report.Concurrency)
	fmt.Printf("  Reachable:    %d\n", report.Reachable)
	fmt.Printf("  Unreachable:  %d\n", report.Unreachable)
	fmt.Printf("  Login OK:     %d\n", report.LoginOK)
	fmt.Printf("  Login failed: %d\n", report.LoginFailed)
}
