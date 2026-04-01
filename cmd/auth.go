package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/output"
	"net/http"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with a Fastmail API token",
	RunE:  runAuthLogin,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE:  runAuthStatus,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored API token",
	RunE:  runAuthLogout,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	fmt.Fprint(os.Stderr, "Enter API token: ")
	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read token: %w", err)
	}
	token = strings.TrimSpace(token)

	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	// Validate the token by fetching the session
	jmapClient := &jmap.Client{
		SessionEndpoint: client.SessionEndpoint,
		HttpClient:      http.DefaultClient,
	}
	jmapClient.WithAccessToken(token)

	if err := jmapClient.Authenticate(); err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	// Store in keyring
	if err := client.StoreToken(token); err != nil {
		// Keyring may not be available (headless server). Print the token env var hint.
		fmt.Fprintf(os.Stderr, "Warning: could not store token in keyring: %v\n", err)
		fmt.Fprintf(os.Stderr, "Set FASTMAIL_API_TOKEN environment variable instead.\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Authenticated as %s\n", jmapClient.Session.Username)
	fmt.Fprintf(os.Stderr, "Token stored in keyring.\n")
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	if jsonOutput {
		return output.PrintJSON(map[string]any{
			"username":  c.Username(),
			"accountId": c.MailAccountID(),
		})
	}

	fmt.Printf("Authenticated as: %s\n", c.Username())
	fmt.Printf("Account ID:       %s\n", c.MailAccountID())
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	if err := client.DeleteToken(); err != nil {
		return fmt.Errorf("failed to remove token: %w", err)
	}
	fmt.Fprintln(os.Stderr, "Token removed from keyring.")
	return nil
}
