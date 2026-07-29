package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"annet-oil/internal/featureset"
	"annet-oil/internal/inventory"
)

var featureSetCmd = &cobra.Command{
	Use:   "featureset [host]",
	Short: "Report feature/capability support for a device platform",
	Long: `Report which features (and feature modes) a device platform supports, based on
its vendor, model and software version, using the curated feature-set knowledge
base (storage.featureset_file).

This lets an operator — or an AI agent — avoid proposing configuration the
hardware cannot run, e.g. PTP Boundary Clock on a switch that only implements
Transparent Clock.

Examples:
  annet-oil featureset --vendor juniper --model EX4100-48MP --version 24.4R2.23
  annet-oil featureset --vendor juniper --model EX4100-48MP --feature ptp
  annet-oil featureset leaf-01 --model EX4100-48MP --version 24.4R2.23

With a positional host the vendor is resolved from the inventory when --vendor
is not given (model/version still have to be supplied).`,
	// Like check, this command only needs config + inventory + the knowledge
	// base, not Docker, so it overrides the root PersistentPreRunE.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := loadConfigAndLogging(); err != nil {
			return err
		}
		loadInventory()
		loadFeatureSets()
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if s3Uploader != nil {
			s3Uploader.Stop()
		}
		return nil
	},
	RunE: runFeatureSetCommand,
}

var (
	fsVendor  string
	fsModel   string
	fsVersion string
	fsFeature string
	fsFormat  string
)

func init() {
	featureSetCmd.Flags().StringVar(&fsVendor, "vendor", "", "Device vendor (e.g. juniper, arista, cisco)")
	featureSetCmd.Flags().StringVar(&fsModel, "model", "", "Device model (e.g. EX4100-48MP)")
	featureSetCmd.Flags().StringVar(&fsVersion, "version", "", "Software version (e.g. 24.4R2.23); enables version gating")
	featureSetCmd.Flags().StringVar(&fsFeature, "feature", "", "Report only this feature (by name, e.g. ptp)")
	featureSetCmd.Flags().StringVar(&fsFormat, "format", "table", "Output format (table|json)")

	rootCmd.AddCommand(featureSetCmd)
}

func runFeatureSetCommand(cmd *cobra.Command, args []string) error {
	vendor := fsVendor

	// A positional host resolves the vendor from the inventory when --vendor is
	// not supplied.
	if len(args) > 0 && vendor == "" {
		if dev, err := inventory.GetDevice(args[0]); err == nil {
			vendor = dev.Vendor
		} else {
			return fmt.Errorf("host %q not found in inventory; pass --vendor explicitly", args[0])
		}
	}

	if strings.TrimSpace(vendor) == "" || strings.TrimSpace(fsModel) == "" {
		return fmt.Errorf("vendor and model are required (use --vendor/--model, or a host that resolves to a vendor)")
	}

	result := featureset.Resolve(featureset.Query{
		Vendor:  vendor,
		Model:   fsModel,
		Version: fsVersion,
		Feature: fsFeature,
	})

	switch fsFormat {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	default:
		printFeatureSetTable(result)
		return nil
	}
}

func printFeatureSetTable(fs *featureset.FeatureSet) {
	header := fmt.Sprintf("%s / %s", fs.Vendor, fs.Model)
	if fs.Version != "" {
		header += " / " + fs.Version
	}
	if fs.Family != "" {
		header += fmt.Sprintf("  (family: %s)", fs.Family)
	}
	fmt.Println(header)

	for _, wmsg := range fs.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", wmsg)
	}
	if len(fs.Features) == 0 {
		fmt.Println("  (no features)")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FEATURE\tCATEGORY\tSUPPORT\tMODES\tNOTES")
	for _, f := range fs.Features {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			f.Name, dash(f.Category), f.Support, formatModes(f.Modes), dash(f.Notes))
	}
	w.Flush()
}

// formatModes renders modes compactly as "name=support" joined by commas.
func formatModes(modes []featureset.Mode) string {
	if len(modes) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(modes))
	for _, m := range modes {
		parts = append(parts, fmt.Sprintf("%s=%s", m.Name, m.Support))
	}
	return strings.Join(parts, ", ")
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
