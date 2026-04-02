package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

		if cfg.Pager != "" && os.Getenv("PAGER") == "" {
			os.Setenv("PAGER", cfg.Pager)
		}

		return nil
	},
}

var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Output command tree as JSON (for AI agents)",
	RunE: func(cmd *cobra.Command, args []string) error {
		schema := buildCommandSchema(rootCmd, true)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(schema)
	},
}

// RootCmd returns the root command for documentation generation.
func RootCmd() *cobra.Command {
	return rootCmd
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().StringVar(&tokenFlag, "token", "", "API token (overrides env and keyring)")
	rootCmd.AddCommand(describeCmd)
}

type commandSchema struct {
	Name        string          `json:"name"`
	Usage       string          `json:"usage"`
	Short       string          `json:"short"`
	Long        string          `json:"long,omitempty"`
	Example     string          `json:"example,omitempty"`
	Flags       []flagSchema    `json:"flags,omitempty"`
	Subcommands []commandSchema `json:"subcommands,omitempty"`
}

type flagSchema struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default,omitempty"`
	Usage     string `json:"usage"`
}

func buildCommandSchema(cmd *cobra.Command, recurse bool) commandSchema {
	schema := commandSchema{
		Name:    cmd.Name(),
		Usage:   cmd.UseLine(),
		Short:   cmd.Short,
		Long:    cmd.Long,
		Example: cmd.Example,
	}

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		schema.Flags = append(schema.Flags, flagSchema{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Type:      f.Value.Type(),
			Default:   f.DefValue,
			Usage:     f.Usage,
		})
	})

	if recurse {
		for _, sub := range cmd.Commands() {
			if sub.IsAvailableCommand() {
				schema.Subcommands = append(schema.Subcommands, buildCommandSchema(sub, true))
			}
		}
	}

	return schema
}
