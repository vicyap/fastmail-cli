package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/config"
)

var (
	// Version is set at build time via ldflags.
	Version = "dev"

	jsonOutput bool
	tokenFlag  string
	cfg        *config.Config
)

var rootCmd = &cobra.Command{
	Use:     "fm",
	Short:   "A command-line interface for Fastmail",
	Long:    "fm is a CLI for Fastmail built on the JMAP protocol.",
	Version: Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
			cfg = &config.Config{}
		}

		if !cfg.ColorEnabled() {
			color.NoColor = true
		}

		// Apply pager from config if PAGER is not already set
		if cfg.Pager != "" && os.Getenv("PAGER") == "" {
			os.Setenv("PAGER", cfg.Pager)
		}

		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().StringVar(&tokenFlag, "token", "", "API token (overrides env and keyring)")
}
