package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	cfgFile  string
	logLevel string
	dryRun   bool
)

var rootCmd = &cobra.Command{
	Use:   "veil [flags] -- command [args...]",
	Short: "Security sandbox for AI agents",
	Long: `Veil provides filesystem isolation via macOS seatbelt and network
policy enforcement via an allowlist proxy.

Example:
  veil -- npm install
  veil --dry-run -- make build`,
	Version:               "0.1.0",
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("command required after --")
		}
		// TODO: implement execution
		fmt.Printf("Would run: %v\n", args)
		fmt.Printf("Config: %s, LogLevel: %s, DryRun: %v\n", cfgFile, logLevel, dryRun)
		return nil
	},
}

func init() {
	rootCmd.Flags().StringVar(&cfgFile, "config", "", "config file (default: ~/.veilwarden/config.yaml)")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print seatbelt profile without executing")
}
