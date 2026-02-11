package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/seslattery/veil/internal/config"
	"github.com/seslattery/veil/internal/logging"
	"github.com/seslattery/veil/internal/policy"
	"github.com/seslattery/veil/internal/proxy"
	"github.com/seslattery/veil/internal/sandbox"
	"github.com/spf13/cobra"
)

var (
	cfgFile  string
	logLevel string
	dryRun   bool
)

var rootCmd = &cobra.Command{
	Use:   "veil [flags] <command> [args...]",
	Short: "Security sandbox for AI agents",
	Long: `Veil provides filesystem isolation via macOS seatbelt and network
policy enforcement via an allowlist proxy.

Example:
  veil claude
  veil npm install
  veil --dry-run -- make build`,
	Version:               "0.1.0",
	DisableFlagsInUseLine: true,
	SilenceErrors:         true,
	SilenceUsage:          true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return run(cmd, args)
	},
}

func init() {
	rootCmd.Flags().StringVar(&cfgFile, "config", "", "config file (default: ~/.veilwarden/config.yaml)")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print seatbelt profile without executing")
}

func main() {
	Execute()
}

func Execute() {
	args := os.Args[1:]
	if shouldRewriteArgs(args) {
		args = insertArgSeparator(args)
		rootCmd.SetArgs(args)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set up logging
	logger, cleanup, err := logging.Setup(logLevel)
	if err != nil {
		return fmt.Errorf("setting up logging: %w", err)
	}
	defer cleanup()

	// Create policy
	pol, err := policy.New(cfg.Policy.Allowlist)
	if err != nil {
		return fmt.Errorf("creating policy: %w", err)
	}

	// Create and start proxy
	prx, err := proxy.New(pol, logger)
	if err != nil {
		return fmt.Errorf("creating proxy: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Start proxy in background
	proxyErr := make(chan error, 1)
	go func() {
		proxyErr <- prx.Start(ctx)
	}()

	// Create sandbox
	sb := sandbox.New(prx.Port(), cfg.Sandbox.AllowedReadPaths, cfg.Sandbox.AllowedWritePaths)

	// Dry run: just print profile
	if dryRun {
		profile, err := sb.Profile()
		if err != nil {
			return fmt.Errorf("generating profile: %w", err)
		}
		fmt.Println(profile)
		return nil
	}

	// Build environment with proxy settings
	env := buildEnv(prx.Addr())

	logger.Info("executing command",
		"command", args[0],
		"args", args[1:],
		"proxy_port", prx.Port(),
	)

	// Run command in sandbox
	if err := sb.Run(ctx, args[0], args[1:], env); err != nil {
		return fmt.Errorf("sandbox execution: %w", err)
	}

	return nil
}

func buildEnv(proxyAddr string) []string {
	env := os.Environ()
	proxyURL := "http://" + proxyAddr

	// Add proxy environment variables
	env = append(env,
		"HTTP_PROXY="+proxyURL,
		"HTTPS_PROXY="+proxyURL,
		"http_proxy="+proxyURL,
		"https_proxy="+proxyURL,
	)

	return env
}

// These are helper functions so that `veil claude` works. It rewrites it to `veil -- claude`
func isSubcommand(name string) bool {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name {
			return true
		}
	}
	return name == "help"
}

func shouldRewriteArgs(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return !isSubcommand(arg)
	}
	return false
}

func insertArgSeparator(args []string) []string {
	for i, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return append(append(args[:i:i], "--"), args[i:]...)
		}
	}
	return args
}
