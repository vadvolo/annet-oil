package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
	checkOutputFormat string
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
	checkCmd.Flags().IntVar(&checkLoginTO, "login-timeout", 8, "SSH handshake timeout in seconds")
	checkCmd.Flags().BoolVar(&checkNoLogin, "no-login", false, "Only check port reachability, skip SSH login")
	checkCmd.Flags().StringVarP(&checkOutput, "output", "o", "", "Write availability report to this file (format inferred from .json/.csv extension, or set with --output-format)")
	checkCmd.Flags().StringVar(&checkOutputFormat, "output-format", "", "Report file format (json|csv); default inferred from --output extension")
	checkCmd.Flags().StringVar(&checkFormat, "format", "table", "Console output format (table|json|csv|summary)")
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
				// Host not in inventory: probe directly, taking credentials
				// from the environment (as the execute handler does).
				devices = append(devices, inventory.Device{
					Hostname:    host,
					IP:          host,
					Credentials: envCredentials(),
				})
			}
		}
		return devices, nil
	}

	if inventory.GetInventory() == nil {
		return nil, fmt.Errorf("no inventory loaded; configure storage.inventory_file or pass hostnames as arguments")
	}
	return inventory.FilterDevices(checkVendor, checkPlatform, checkPattern), nil
}

// envCredentials returns device credentials sourced from the DEVICE_USERNAME /
// DEVICE_PASSWORD environment variables, used for hosts not present in the
// inventory.
func envCredentials() inventory.DeviceCredentials {
	return inventory.DeviceCredentials{
		Login:    os.Getenv("DEVICE_USERNAME"),
		Password: os.Getenv("DEVICE_PASSWORD"),
	}
}

func writeCheckReport(path string, report *check.BatchReport) error {
	format := resolveOutputFormat(path, checkOutputFormat)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	switch format {
	case "csv":
		return writeCheckCSV(f, report)
	default:
		return writeCheckJSON(f, report)
	}
}

// resolveOutputFormat picks the report file format: the explicit --output-format
// when set, otherwise inferred from the file extension, defaulting to json.
func resolveOutputFormat(path, explicit string) string {
	if explicit != "" {
		return strings.ToLower(explicit)
	}
	if strings.EqualFold(filepath.Ext(path), ".csv") {
		return "csv"
	}
	return "json"
}

func printCheckReport(report *check.BatchReport, format string) error {
	switch format {
	case "json":
		return writeCheckJSON(os.Stdout, report)
	case "csv":
		return writeCheckCSV(os.Stdout, report)
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

func writeCheckJSON(w io.Writer, report *check.BatchReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// writeCheckCSV writes one row per checked device. Open/closed ports are
// semicolon-joined so each device stays on a single line.
func writeCheckCSV(w io.Writer, report *check.BatchReport) error {
	cw := csv.NewWriter(w)

	header := []string{
		"hostname", "ip", "vendor", "reachable", "login",
		"open_ports", "closed_ports", "error_type", "error_message",
		"duration_ms", "timestamp",
	}
	if err := cw.Write(header); err != nil {
		return err
	}

	for _, r := range report.Results {
		var open, closed []string
		for _, p := range r.Ports {
			if p.Open {
				open = append(open, strconv.Itoa(p.Port))
			} else {
				closed = append(closed, strconv.Itoa(p.Port))
			}
		}

		errType, errMsg := "", ""
		if r.Error != nil {
			errType = r.Error.Type
			errMsg = r.Error.Message
		}

		row := []string{
			r.Hostname, r.IP, r.Vendor,
			strconv.FormatBool(r.Reachable), r.Login,
			strings.Join(open, ";"), strings.Join(closed, ";"),
			errType, errMsg,
			strconv.FormatInt(r.DurationMs, 10),
			r.Timestamp.Format(time.RFC3339),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
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
	processed := report.Reachable + report.Unreachable + report.Canceled
	fmt.Printf("\nChecked %d of %d device(s) in %dms (concurrency %d)\n",
		processed, report.Total, report.DurationMs, report.Concurrency)
	fmt.Printf("  Reachable:     %d\n", report.Reachable)
	fmt.Printf("  Unreachable:   %d\n", report.Unreachable)
	if report.Canceled > 0 {
		fmt.Printf("  Canceled:      %d\n", report.Canceled)
	}
	fmt.Printf("  Login OK:      %d\n", report.LoginOK)
	fmt.Printf("  Login failed:  %d\n", report.LoginFailed)
	fmt.Printf("  Login skipped: %d\n", report.LoginSkipped)
}
