package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/emailsubmission"
	"git.sr.ht/~rockorager/go-jmap/mail/identity"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/output"
)

var (
	sendTo       []string
	sendCC       []string
	sendBCC      []string
	sendSubject  string
	sendBody     string
	sendIdentity string
)

var emailSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send an email",
	RunE:  runEmailSend,
}

var emailMoveCmd = &cobra.Command{
	Use:   "move <id> <mailbox>",
	Short: "Move an email to a different mailbox",
	Args:  cobra.ExactArgs(2),
	RunE:  runEmailMove,
}

var emailDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an email (move to trash, or --permanent to destroy)",
	Args:  cobra.ExactArgs(1),
	RunE:  runEmailDelete,
}

var emailDeletePermanent bool

func init() {
	emailCmd.AddCommand(emailSendCmd)
	emailSendCmd.Flags().StringSliceVar(&sendTo, "to", nil, "Recipient (required, repeatable)")
	emailSendCmd.Flags().StringSliceVar(&sendCC, "cc", nil, "CC recipient (repeatable)")
	emailSendCmd.Flags().StringSliceVar(&sendBCC, "bcc", nil, "BCC recipient (repeatable)")
	emailSendCmd.Flags().StringVar(&sendSubject, "subject", "", "Subject line (required)")
	emailSendCmd.Flags().StringVar(&sendBody, "body", "", "Plain text body (reads stdin if omitted)")
	emailSendCmd.Flags().StringVar(&sendIdentity, "identity", "", "Sending identity ID (default: primary)")
	emailSendCmd.MarkFlagRequired("to")
	emailSendCmd.MarkFlagRequired("subject")

	emailCmd.AddCommand(emailMoveCmd)

	emailCmd.AddCommand(emailDeleteCmd)
	emailDeleteCmd.Flags().BoolVar(&emailDeletePermanent, "permanent", false, "Permanently destroy instead of trashing")
}

func runEmailSend(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	accountID := c.MailAccountID()

	// Read body from stdin if not provided
	body := sendBody
	if body == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read body from stdin: %w", err)
		}
		body = string(data)
	}

	// Get sending identity
	identityID, err := resolveIdentity(c, sendIdentity)
	if err != nil {
		return err
	}

	// Find drafts and sent mailboxes
	draftsID, err := findMailboxByRole(c, mailbox.RoleDrafts)
	if err != nil {
		return fmt.Errorf("could not find Drafts mailbox: %w", err)
	}

	// Build email
	toAddrs := make([]*mail.Address, len(sendTo))
	for i, addr := range sendTo {
		toAddrs[i] = parseAddress(addr)
	}

	var ccAddrs []*mail.Address
	for _, addr := range sendCC {
		ccAddrs = append(ccAddrs, parseAddress(addr))
	}

	var bccAddrs []*mail.Address
	for _, addr := range sendBCC {
		bccAddrs = append(bccAddrs, parseAddress(addr))
	}

	newEmail := &email.Email{
		MailboxIDs: map[jmap.ID]bool{draftsID: true},
		To:         toAddrs,
		CC:         ccAddrs,
		BCC:        bccAddrs,
		Subject:    sendSubject,
		Keywords:   map[string]bool{"$draft": true},
		BodyValues: map[string]*email.BodyValue{
			"body": {Value: body},
		},
		TextBody: []*email.BodyPart{
			{PartID: "body", Type: "text/plain"},
		},
	}

	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: accountID,
		Create: map[jmap.ID]*email.Email{
			"draft": newEmail,
		},
	})

	req.Invoke(&emailsubmission.Set{
		Account: accountID,
		Create: map[jmap.ID]*emailsubmission.EmailSubmission{
			"send": {
				IdentityID: identityID,
				EmailID:    "#draft",
			},
		},
		OnSuccessUpdateEmail: map[jmap.ID]jmap.Patch{
			"#send": {
				"keywords/$draft":    nil,
				"keywords/$seen":     true,
				"mailboxIds/" + string(draftsID): nil,
			},
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}

	var createdEmailID jmap.ID
	var submissionID jmap.ID

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error (%s): %s", inv.Name, r.Error())
		case *email.SetResponse:
			if r.NotCreated != nil {
				if setErr, ok := r.NotCreated["draft"]; ok {
					return fmt.Errorf("failed to create email: %s", formatEmailSetError(setErr))
				}
			}
			if created, ok := r.Created["draft"]; ok {
				createdEmailID = created.ID
			}
		case *emailsubmission.SetResponse:
			if r.NotCreated != nil {
				if setErr, ok := r.NotCreated["send"]; ok {
					return fmt.Errorf("failed to submit email: %s", formatEmailSetError(setErr))
				}
			}
			if created, ok := r.Created["send"]; ok {
				submissionID = created.ID
			}
		}
	}

	if jsonOutput {
		return output.PrintJSON(map[string]any{
			"emailId":      createdEmailID,
			"submissionId": submissionID,
		})
	}

	fmt.Fprintf(os.Stderr, "Email sent successfully.\n")
	return nil
}

func runEmailMove(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	emailID := jmap.ID(args[0])
	targetMailbox := args[1]

	targetID, err := findMailboxByName(c, targetMailbox)
	if err != nil {
		return err
	}

	// First, get current mailboxes to remove the email from them
	req := &jmap.Request{}
	req.Invoke(&email.Get{
		Account:    c.MailAccountID(),
		IDs:        []jmap.ID{emailID},
		Properties: []string{"mailboxIds"},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get email: %w", err)
	}

	var currentMailboxes map[jmap.ID]bool
	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *email.GetResponse:
			if len(r.List) == 0 {
				return fmt.Errorf("email not found: %s", emailID)
			}
			currentMailboxes = r.List[0].MailboxIDs
		}
	}

	// Build patch: remove from current mailboxes, add to target
	patch := jmap.Patch{}
	for mboxID := range currentMailboxes {
		patch["mailboxIds/"+string(mboxID)] = nil
	}
	patch["mailboxIds/"+string(targetID)] = true

	req2 := &jmap.Request{}
	req2.Invoke(&email.Set{
		Account: c.MailAccountID(),
		Update: map[jmap.ID]jmap.Patch{
			emailID: patch,
		},
	})

	resp2, err := c.JMAP.Do(req2)
	if err != nil {
		return fmt.Errorf("email move failed: %w", err)
	}

	for _, inv := range resp2.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *email.SetResponse:
			if r.NotUpdated != nil {
				if setErr, ok := r.NotUpdated[emailID]; ok {
					return fmt.Errorf("failed to move email: %s", formatEmailSetError(setErr))
				}
			}
		}
	}

	if jsonOutput {
		return output.PrintJSON(map[string]string{
			"id":      string(emailID),
			"mailbox": string(targetID),
		})
	}

	fmt.Fprintf(os.Stderr, "Email moved to %s.\n", targetMailbox)
	return nil
}

func runEmailDelete(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	emailID := jmap.ID(args[0])

	if emailDeletePermanent {
		return permanentlyDeleteEmail(c, emailID)
	}
	return trashEmail(c, emailID)
}

func trashEmail(c *client.Client, emailID jmap.ID) error {
	trashID, err := findMailboxByRole(c, mailbox.RoleTrash)
	if err != nil {
		return fmt.Errorf("could not find Trash mailbox: %w", err)
	}

	// Get current mailboxes
	req := &jmap.Request{}
	req.Invoke(&email.Get{
		Account:    c.MailAccountID(),
		IDs:        []jmap.ID{emailID},
		Properties: []string{"mailboxIds"},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get email: %w", err)
	}

	var currentMailboxes map[jmap.ID]bool
	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *email.GetResponse:
			if len(r.List) == 0 {
				return fmt.Errorf("email not found: %s", emailID)
			}
			currentMailboxes = r.List[0].MailboxIDs
		}
	}

	patch := jmap.Patch{}
	for mboxID := range currentMailboxes {
		patch["mailboxIds/"+string(mboxID)] = nil
	}
	patch["mailboxIds/"+string(trashID)] = true

	req2 := &jmap.Request{}
	req2.Invoke(&email.Set{
		Account: c.MailAccountID(),
		Update: map[jmap.ID]jmap.Patch{
			emailID: patch,
		},
	})

	resp2, err := c.JMAP.Do(req2)
	if err != nil {
		return fmt.Errorf("email delete failed: %w", err)
	}

	for _, inv := range resp2.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *email.SetResponse:
			if r.NotUpdated != nil {
				if setErr, ok := r.NotUpdated[emailID]; ok {
					return fmt.Errorf("failed to trash email: %s", formatEmailSetError(setErr))
				}
			}
		}
	}

	if jsonOutput {
		return output.PrintJSON(map[string]string{"id": string(emailID), "action": "trashed"})
	}

	fmt.Fprintln(os.Stderr, "Email moved to Trash.")
	return nil
}

func permanentlyDeleteEmail(c *client.Client, emailID jmap.ID) error {
	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: c.MailAccountID(),
		Destroy: []jmap.ID{emailID},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("email destroy failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *email.SetResponse:
			if r.NotDestroyed != nil {
				if setErr, ok := r.NotDestroyed[emailID]; ok {
					return fmt.Errorf("failed to destroy email: %s", formatEmailSetError(setErr))
				}
			}
		}
	}

	if jsonOutput {
		return output.PrintJSON(map[string]string{"id": string(emailID), "action": "destroyed"})
	}

	fmt.Fprintln(os.Stderr, "Email permanently destroyed.")
	return nil
}

func resolveIdentity(c *client.Client, identityFlag string) (jmap.ID, error) {
	req := &jmap.Request{}
	req.Invoke(&identity.Get{
		Account: c.MailAccountID(),
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get identities: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return "", fmt.Errorf("JMAP error: %s", r.Error())
		case *identity.GetResponse:
			if identityFlag != "" {
				for _, ident := range r.List {
					if string(ident.ID) == identityFlag {
						return ident.ID, nil
					}
				}
				return "", fmt.Errorf("identity not found: %s", identityFlag)
			}
			// Return first identity as default
			if len(r.List) > 0 {
				return r.List[0].ID, nil
			}
			return "", fmt.Errorf("no sending identities found")
		}
	}

	return "", fmt.Errorf("unexpected response")
}

func parseAddress(addr string) *mail.Address {
	// Handle "Name <email>" format
	if idx := strings.Index(addr, "<"); idx >= 0 {
		name := strings.TrimSpace(addr[:idx])
		emailAddr := strings.Trim(addr[idx:], "<> ")
		return &mail.Address{Name: name, Email: emailAddr}
	}
	return &mail.Address{Email: addr}
}

func formatEmailSetError(e *jmap.SetError) string {
	if e.Description != nil {
		return fmt.Sprintf("%s: %s", e.Type, *e.Description)
	}
	return e.Type
}
