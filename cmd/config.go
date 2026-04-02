package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/config"
	"github.com/vicyap/fastmail-cli/internal/output"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default config file",
	RunE:  runConfigInit,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfigShow,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print config file path",
	RunE:  runConfigPath,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
}

const defaultConfigTemplate = `# fm configuration
# See: https://github.com/vicyap/fastmail-cli

# Default sending identity (use "fm identity list" to see available IDs)
# default_identity = ""

# Default mailbox for "fm email list" (default: Inbox)
# default_mailbox = "Inbox"

# Pager for "fm email read" and "fm email thread" (default: $PAGER or less)
# pager = "less -R"

# Enable or disable color output (default: true)
# color = true
`

func runConfigInit(cmd *cobra.Command, args []string) error {
	path, err := config.Path()
	if err != nil {
		return fmt.Errorf("failed to determine config path: %w", err)
	}

	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s", path)
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Config file created: %s\n", path)
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	c, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if jsonOutput {
		return output.PrintJSON(c)
	}

	path, _ := config.Path()
	fmt.Printf("Config file: %s\n\n", path)

	identity := c.DefaultIdentity
	if identity == "" {
		identity = "(not set)"
	}
	fmt.Printf("default_identity = %s\n", identity)

	mailbox := c.DefaultMailbox
	if mailbox == "" {
		mailbox = "(not set, defaults to Inbox)"
	}
	fmt.Printf("default_mailbox  = %s\n", mailbox)

	pager := c.Pager
	if pager == "" {
		pager = "(not set, defaults to $PAGER or less)"
	}
	fmt.Printf("pager            = %s\n", pager)

	fmt.Printf("color            = %v\n", c.ColorEnabled())

	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}
