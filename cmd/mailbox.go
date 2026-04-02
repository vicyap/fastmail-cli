package cmd

import (
	"fmt"
	"os"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/output"
)

var mailboxCmd = &cobra.Command{
	Use:   "mailbox",
	Short: "Manage mailboxes",
}

var mailboxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all mailboxes",
	RunE:  runMailboxList,
}

var mailboxCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new mailbox",
	Args:  cobra.ExactArgs(1),
	RunE:  runMailboxCreate,
}

var mailboxRenameCmd = &cobra.Command{
	Use:   "rename <name|id> <new-name>",
	Short: "Rename a mailbox",
	Args:  cobra.ExactArgs(2),
	RunE:  runMailboxRename,
}

var mailboxDeleteCmd = &cobra.Command{
	Use:   "delete <name|id>",
	Short: "Delete a mailbox",
	Args:  cobra.ExactArgs(1),
	RunE:  runMailboxDelete,
}

var mailboxDeleteForce bool

func init() {
	rootCmd.AddCommand(mailboxCmd)
	mailboxCmd.AddCommand(mailboxListCmd)
	mailboxCmd.AddCommand(mailboxCreateCmd)
	mailboxCmd.AddCommand(mailboxRenameCmd)
	mailboxCmd.AddCommand(mailboxDeleteCmd)
	mailboxDeleteCmd.Flags().BoolVar(&mailboxDeleteForce, "force", false, "Delete mailbox and all emails in it")
}

func runMailboxList(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{
		Account: c.MailAccountID(),
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("mailbox list failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *mailbox.GetResponse:
			return printMailboxes(r.List)
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runMailboxCreate(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&mailbox.Set{
		Account: c.MailAccountID(),
		Create: map[jmap.ID]*mailbox.Mailbox{
			"new1": {Name: args[0]},
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("mailbox create failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *mailbox.SetResponse:
			if r.NotCreated != nil {
				if setErr, ok := r.NotCreated["new1"]; ok {
					return fmt.Errorf("failed to create mailbox: %s", formatMailboxSetError(setErr))
				}
			}
			created := r.Created["new1"]
			if jsonOutput {
				return output.PrintJSON(created)
			}
			fmt.Fprintf(os.Stderr, "Mailbox '%s' created (ID: %s)\n", args[0], created.ID)
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runMailboxRename(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	mailboxID, err := findMailboxByName(c, args[0])
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&mailbox.Set{
		Account: c.MailAccountID(),
		Update: map[jmap.ID]jmap.Patch{
			mailboxID: {"name": args[1]},
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("mailbox rename failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *mailbox.SetResponse:
			if r.NotUpdated != nil {
				if setErr, ok := r.NotUpdated[mailboxID]; ok {
					return fmt.Errorf("failed to rename mailbox: %s", formatMailboxSetError(setErr))
				}
			}
			if jsonOutput {
				return output.PrintJSON(map[string]string{"id": string(mailboxID), "name": args[1]})
			}
			fmt.Fprintf(os.Stderr, "Mailbox renamed to '%s'.\n", args[1])
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runMailboxDelete(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	mailboxID, err := findMailboxByName(c, args[0])
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&mailbox.Set{
		Account:               c.MailAccountID(),
		Destroy:               []jmap.ID{mailboxID},
		OnDestroyRemoveEmails: mailboxDeleteForce,
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("mailbox delete failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *mailbox.SetResponse:
			if r.NotDestroyed != nil {
				if setErr, ok := r.NotDestroyed[mailboxID]; ok {
					return fmt.Errorf("failed to delete mailbox: %s", formatMailboxSetError(setErr))
				}
			}
			if jsonOutput {
				return output.PrintJSON(map[string]string{"id": string(mailboxID), "action": "deleted"})
			}
			fmt.Fprintf(os.Stderr, "Mailbox '%s' deleted.\n", args[0])
			return nil
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func formatMailboxSetError(e *jmap.SetError) string {
	if e.Description != nil {
		return fmt.Sprintf("%s: %s", e.Type, *e.Description)
	}
	return e.Type
}

func printMailboxes(mailboxes []*mailbox.Mailbox) error {
	if jsonOutput {
		return output.PrintJSON(mailboxes)
	}

	tbl := output.NewTable(os.Stdout)
	tbl.Headers("NAME", "ROLE", "UNREAD", "TOTAL")
	for _, mbox := range mailboxes {
		role := string(mbox.Role)
		if role == "" {
			role = "-"
		}
		tbl.Row(
			mbox.Name,
			role,
			fmt.Sprintf("%d", mbox.UnreadEmails),
			fmt.Sprintf("%d", mbox.TotalEmails),
		)
	}
	return tbl.Flush()
}

// findMailboxByName looks up a mailbox ID by name or returns the input as an ID if not found.
func findMailboxByName(c *client.Client, nameOrID string) (jmap.ID, error) {
	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{
		Account: c.MailAccountID(),
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch mailboxes: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *mailbox.GetResponse:
			for _, mbox := range r.List {
				if string(mbox.Name) == nameOrID {
					return mbox.ID, nil
				}
				if string(mbox.ID) == nameOrID {
					return mbox.ID, nil
				}
			}
			return "", fmt.Errorf("mailbox not found: %s", nameOrID)
		case *jmap.MethodError:
			return "", fmt.Errorf("JMAP error: %s", r.Error())
		}
	}

	return "", fmt.Errorf("unexpected response")
}

// findMailboxByRole looks up a mailbox ID by its role.
func findMailboxByRole(c *client.Client, role mailbox.Role) (jmap.ID, error) {
	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{
		Account: c.MailAccountID(),
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch mailboxes: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *mailbox.GetResponse:
			for _, mbox := range r.List {
				if mbox.Role == role {
					return mbox.ID, nil
				}
			}
			return "", fmt.Errorf("no mailbox with role %s found", role)
		case *jmap.MethodError:
			return "", fmt.Errorf("JMAP error: %s", r.Error())
		}
	}

	return "", fmt.Errorf("unexpected response")
}
