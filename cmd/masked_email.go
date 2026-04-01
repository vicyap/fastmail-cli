package cmd

import (
	"fmt"
	"os"

	"git.sr.ht/~rockorager/go-jmap"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/maskedemail"
	"github.com/vicyap/fastmail-cli/internal/output"
)

var (
	maskedEmailState       string
	maskedEmailDomain      string
	maskedEmailDescription string
	maskedEmailPrefix      string
)

var maskedEmailCmd = &cobra.Command{
	Use:     "masked-email",
	Aliases: []string{"me"},
	Short:   "Manage masked email addresses",
}

var maskedEmailListCmd = &cobra.Command{
	Use:   "list",
	Short: "List masked email addresses",
	RunE:  runMaskedEmailList,
}

var maskedEmailCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new masked email address",
	RunE:  runMaskedEmailCreate,
}

var maskedEmailEnableCmd = &cobra.Command{
	Use:   "enable <id>",
	Short: "Enable a masked email address",
	Args:  cobra.ExactArgs(1),
	RunE:  runMaskedEmailEnable,
}

var maskedEmailDisableCmd = &cobra.Command{
	Use:   "disable <id>",
	Short: "Disable a masked email address",
	Args:  cobra.ExactArgs(1),
	RunE:  runMaskedEmailDisable,
}

var maskedEmailDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a masked email address",
	Args:  cobra.ExactArgs(1),
	RunE:  runMaskedEmailDelete,
}

func init() {
	rootCmd.AddCommand(maskedEmailCmd)

	maskedEmailCmd.AddCommand(maskedEmailListCmd)
	maskedEmailListCmd.Flags().StringVar(&maskedEmailState, "state", "", "Filter by state (enabled, disabled, pending, deleted)")

	maskedEmailCmd.AddCommand(maskedEmailCreateCmd)
	maskedEmailCreateCmd.Flags().StringVar(&maskedEmailDomain, "domain", "", "Associate with a domain")
	maskedEmailCreateCmd.Flags().StringVar(&maskedEmailDescription, "description", "", "Human-readable description")
	maskedEmailCreateCmd.Flags().StringVar(&maskedEmailPrefix, "prefix", "", "Email prefix (optional)")

	maskedEmailCmd.AddCommand(maskedEmailEnableCmd)
	maskedEmailCmd.AddCommand(maskedEmailDisableCmd)
	maskedEmailCmd.AddCommand(maskedEmailDeleteCmd)
}

func maskedEmailAccountID(c *client.Client) jmap.ID {
	return c.JMAP.Session.PrimaryAccounts[maskedemail.URI]
}

func runMaskedEmailList(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&maskedemail.Get{
		Account: maskedEmailAccountID(c),
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("masked-email list failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *maskedemail.GetResponse:
			list := r.List
			if maskedEmailState != "" {
				list = filterMaskedEmails(list, maskedEmailState)
			}
			return printMaskedEmails(list)
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runMaskedEmailCreate(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	newME := &maskedemail.MaskedEmail{
		State: "enabled",
	}
	if maskedEmailDomain != "" {
		newME.ForDomain = maskedEmailDomain
	}
	if maskedEmailDescription != "" {
		newME.Description = maskedEmailDescription
	}
	if maskedEmailPrefix != "" {
		newME.EmailPrefix = maskedEmailPrefix
	}

	req := &jmap.Request{}
	req.Invoke(&maskedemail.Set{
		Account: maskedEmailAccountID(c),
		Create: map[jmap.ID]*maskedemail.MaskedEmail{
			"new1": newME,
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("masked-email create failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *maskedemail.SetResponse:
			if err := checkSetErrors(r, "new1"); err != nil {
				return err
			}
			created := r.Created["new1"]
			if jsonOutput {
				return output.PrintJSON(created)
			}
			fmt.Println(created.Email)
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runMaskedEmailEnable(cmd *cobra.Command, args []string) error {
	return updateMaskedEmailState(args[0], "enabled")
}

func runMaskedEmailDisable(cmd *cobra.Command, args []string) error {
	return updateMaskedEmailState(args[0], "disabled")
}

func runMaskedEmailDelete(cmd *cobra.Command, args []string) error {
	return updateMaskedEmailState(args[0], "deleted")
}

func updateMaskedEmailState(id string, state string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&maskedemail.Set{
		Account: maskedEmailAccountID(c),
		Update: map[jmap.ID]jmap.Patch{
			jmap.ID(id): {
				"state": state,
			},
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("masked-email update failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *maskedemail.SetResponse:
			if r.NotUpdated != nil {
				if setErr, ok := r.NotUpdated[jmap.ID(id)]; ok {
					return fmt.Errorf("failed to update %s: %s", id, formatSetError(setErr))
				}
			}
			if jsonOutput {
				return output.PrintJSON(map[string]string{
					"id":    id,
					"state": state,
				})
			}
			fmt.Fprintf(os.Stderr, "Masked email %s set to %s\n", id, state)
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func filterMaskedEmails(list []*maskedemail.MaskedEmail, state string) []*maskedemail.MaskedEmail {
	var filtered []*maskedemail.MaskedEmail
	for _, me := range list {
		if me.State == state {
			filtered = append(filtered, me)
		}
	}
	return filtered
}

func printMaskedEmails(list []*maskedemail.MaskedEmail) error {
	if jsonOutput {
		return output.PrintJSON(list)
	}

	tbl := output.NewTable(os.Stdout)
	tbl.Headers("ID", "EMAIL", "STATE", "DOMAIN", "DESCRIPTION", "LAST MESSAGE")
	for _, me := range list {
		lastMsg := me.LastMessageAt
		if lastMsg == "" {
			lastMsg = "-"
		}
		desc := me.Description
		if desc == "" {
			desc = "-"
		}
		domain := me.ForDomain
		if domain == "" {
			domain = "-"
		}
		tbl.Row(string(me.ID), me.Email, me.State, domain, desc, lastMsg)
	}
	return tbl.Flush()
}

func formatSetError(e *jmap.SetError) string {
	if e.Description != nil {
		return fmt.Sprintf("%s: %s", e.Type, *e.Description)
	}
	return e.Type
}

func checkSetErrors(r *maskedemail.SetResponse, createID jmap.ID) error {
	if r.NotCreated != nil {
		if setErr, ok := r.NotCreated[createID]; ok {
			return fmt.Errorf("failed to create masked email: %s", formatSetError(setErr))
		}
	}
	if r.Created == nil {
		return fmt.Errorf("no masked email created")
	}
	if _, ok := r.Created[createID]; !ok {
		return fmt.Errorf("masked email creation response missing")
	}
	return nil
}
