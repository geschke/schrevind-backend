package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/geschke/schrevind/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "schrevind",
		Short: "Backend service for schrevind.",
		Long: `schrevind is a backend service with a small command-line interface.

It loads configuration from a file, environment variables, and CLI flags,
opens the SQLite database, and starts the HTTP server when requested.

Typical usage:
  schrevind serve --config /path/to/config.yaml

schrevind keeps the CLI intentionally small and focused.`,

		// Uncomment the following line if your bare application
		// has an action associated with it:
		// Run: func(cmd *cobra.Command, args []string) { },
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			//fmt.Printf("Inside rootCmd PersistentPreRun with args: %v\n", args)
			err := config.InitAndLoad(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to init configuration: %w", err)
			}
			return nil
		},
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// init configures package-level command and flag wiring.
func init() {
	// Global config file flag
	rootCmd.PersistentFlags().StringVar(
		&cfgFile,
		"config",
		"",
		"Path to config file (config.yaml, json, toml, or .env). Defaults: config.* → .env",
	)

	// Server listen address (overrides server.listen from config)
	rootCmd.PersistentFlags().String(
		"listen",
		":8080",
		"Listen address for the HTTP server (e.g. :8080 or 0.0.0.0:8080)",
	)
	if err := viper.BindPFlag(
		"server.listen",
		rootCmd.PersistentFlags().Lookup("listen"),
	); err != nil {
		log.Fatalf("Failed to bind listen flag: %v", err)
	}

	// Optional: global CORS default (can still be overridden per form in config)
	rootCmd.PersistentFlags().StringSlice(
		"cors-allowed-origins",
		[]string{},
		"Default CORS allowed origins (comma-separated or repeated).",
	)
	if err := viper.BindPFlag(
		"cors.allowed_origins",
		rootCmd.PersistentFlags().Lookup("cors-allowed-origins"),
	); err != nil {
		log.Fatalf("Failed to bind CORS flag: %v", err)
	}

	// Logging flags (kept for future use; currently not wired into AppConfig)
	/*
		rootCmd.PersistentFlags().String("log-file", "", "Path to log file (default: stdout)")
		rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
		rootCmd.PersistentFlags().String("log-format", "", "Log format: json or console (default)")

		if err := viper.BindPFlag("log_file", rootCmd.PersistentFlags().Lookup("log-file")); err != nil {
			log.Fatalf("Failed to bind log_file flag: %v", err)
		}
		if err := viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level")); err != nil {
			log.Fatalf("Failed to bind log_level flag: %v", err)
		}
		if err := viper.BindPFlag("log_format", rootCmd.PersistentFlags().Lookup("log-format")); err != nil {
			log.Fatalf("Failed to bind log_format flag: %v", err)
		}
	*/
}
