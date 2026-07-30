package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"annet-oil/internal/gnetcli"
	"annet-oil/internal/inventory"
	"annet-oil/internal/opstate"
)

var stateCmd = &cobra.Command{
	Use:   "state [hostname|ip]",
	Short: "Collect normalized operational state from a device as JSON",
	Long: `Collect operational state (facts, interfaces, LLDP neighbors, and optionally
MAC/ARP/route tables) from a device and print it as a single compact,
vendor-neutral JSON document.

Raw "show" output is verbose and vendor-specific; this normalizes it so an AI
agent spends fewer tokens and does not re-parse free text. Juniper is collected
via native "| display json"; Cisco and Eltex via text parsing; MikroTik via
RouterOS "print terse".

Examples:
  annet-oil state leaf-01
  annet-oil state leaf-01 --states facts,interfaces
  annet-oil state leaf-01 --states all --force
  annet-oil state 10.0.0.1 --vendor cisco`,
	Args: cobra.ExactArgs(1),
	// Needs config + inventory + gnetcli, not Docker.
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
	RunE: runStateCommand,
}

var (
	stateStates []string
	stateForce  bool
	stateVendor string
)

func init() {
	stateCmd.Flags().StringSliceVar(&stateStates, "states", nil, "Sections to collect (facts,interfaces,lldp,mac,arp,routes or 'all'); default facts,interfaces,lldp")
	stateCmd.Flags().BoolVar(&stateForce, "force", false, "Bypass the cache and re-query the device")
	stateCmd.Flags().StringVar(&stateVendor, "vendor", "", "Override the device vendor from inventory")

	rootCmd.AddCommand(stateCmd)
}

func runStateCommand(cmd *cobra.Command, args []string) error {
	states, err := opstate.ParseStateTypes(stateStates)
	if err != nil {
		return err
	}

	device, derr := inventory.GetDevice(args[0])
	if derr != nil {
		device = &inventory.Device{
			Hostname:    args[0],
			IP:          args[0],
			Vendor:      stateVendor,
			Credentials: envCredentials(),
		}
	}
	if stateVendor != "" {
		device.Vendor = stateVendor
	}
	if device.Vendor == "" {
		return fmt.Errorf("vendor unknown for %s; set it in inventory or pass --vendor", args[0])
	}

	client, err := gnetcli.New(&cfg.Gnetcli)
	if err != nil {
		return fmt.Errorf("failed to connect to gnetcli: %w", err)
	}
	defer client.Close()

	collector := opstate.NewCollector(newCLIExecutor(client, device), opstate.NewCache(time.Minute))
	result, cerr := collector.Collect(cmd.Context(), device.Hostname, device.Vendor, device.Platform, opstate.CollectOptions{
		States: states,
		Force:  stateForce,
	})
	if cerr != nil {
		return cerr
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func newCLIExecutor(client *gnetcli.Client, device *inventory.Device) opstate.Executor {
	return opstate.ExecFunc(func(ctx context.Context, host, command string) (string, error) {
		target := device.IP
		if target == "" {
			target = device.Hostname
		}
		creds := inventory.PrimaryCredentials(device)
		res, err := client.ExecWithDevice(ctx, target, command, device.Vendor, creds.Login, creds.Password, device.GetPort())
		if err != nil {
			return "", err
		}
		if res.Output == "" && res.Error != "" {
			return "", fmt.Errorf("%s", res.Error)
		}
		return res.Output, nil
	})
}
