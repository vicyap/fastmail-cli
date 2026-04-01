package cmd

import (
	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	tokenFlag  string
)

var rootCmd = &cobra.Command{
	Use:   "fm",
	Short: "A command-line interface for Fastmail",
	Long:  "fm is a CLI for Fastmail built on the JMAP protocol.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().StringVar(&tokenFlag, "token", "", "API token (overrides env and keyring)")
}
