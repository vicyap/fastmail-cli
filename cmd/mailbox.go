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

func init() {
	rootCmd.AddCommand(mailboxCmd)
	mailboxCmd.AddCommand(mailboxListCmd)
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
