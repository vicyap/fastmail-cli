package cmd

import (
	"fmt"
	"os"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/output"
)

var emailFlagCmd = &cobra.Command{
	Use:   "flag <id>",
	Short: "Flag an email",
	Args:  cobra.ExactArgs(1),
	RunE:  runEmailFlag,
}

var emailUnflagCmd = &cobra.Command{
	Use:   "unflag <id>",
	Short: "Unflag an email",
	Args:  cobra.ExactArgs(1),
	RunE:  runEmailUnflag,
}

var emailMarkReadCmd = &cobra.Command{
	Use:   "mark-read <id>",
	Short: "Mark an email as read",
	Args:  cobra.ExactArgs(1),
	RunE:  runEmailMarkRead,
}

var emailMarkUnreadCmd = &cobra.Command{
	Use:   "mark-unread <id>",
	Short: "Mark an email as unread",
	Args:  cobra.ExactArgs(1),
	RunE:  runEmailMarkUnread,
}

func init() {
	emailCmd.AddCommand(emailFlagCmd)
	emailCmd.AddCommand(emailUnflagCmd)
	emailCmd.AddCommand(emailMarkReadCmd)
	emailCmd.AddCommand(emailMarkUnreadCmd)
}

func runEmailFlag(cmd *cobra.Command, args []string) error {
	return setEmailKeyword(args[0], "$flagged", true)
}

func runEmailUnflag(cmd *cobra.Command, args []string) error {
	return setEmailKeyword(args[0], "$flagged", nil)
}

func runEmailMarkRead(cmd *cobra.Command, args []string) error {
	return setEmailKeyword(args[0], "$seen", true)
}

func runEmailMarkUnread(cmd *cobra.Command, args []string) error {
	return setEmailKeyword(args[0], "$seen", nil)
}

func setEmailKeyword(id string, keyword string, value any) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	emailID := jmap.ID(id)

	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: c.MailAccountID(),
		Update: map[jmap.ID]jmap.Patch{
			emailID: {
				"keywords/" + keyword: value,
			},
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("email update failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *email.SetResponse:
			if r.NotUpdated != nil {
				if setErr, ok := r.NotUpdated[emailID]; ok {
					return fmt.Errorf("failed to update email: %s", formatEmailSetError(setErr))
				}
			}
		}
	}

	action := "flagged"
	if value == nil {
		action = "unflagged"
	}

	if jsonOutput {
		return output.PrintJSON(map[string]string{"id": id, "action": action})
	}

	fmt.Fprintf(os.Stderr, "Email %s.\n", action)
	return nil
}
