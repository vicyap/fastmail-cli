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
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/output"
)

var (
	replyAll     bool
	replyBody    string
	replyHTML    bool
)

var emailReplyCmd = &cobra.Command{
	Use:   "reply <id>",
	Short: "Reply to an email",
	Args:  cobra.ExactArgs(1),
	RunE:  runEmailReply,
}

var emailForwardCmd = &cobra.Command{
	Use:   "forward <id> --to=<addr>",
	Short: "Forward an email",
	Args:  cobra.ExactArgs(1),
	RunE:  runEmailForward,
}

var forwardTo []string

func init() {
	emailCmd.AddCommand(emailReplyCmd)
	emailReplyCmd.Flags().BoolVar(&replyAll, "all", false, "Reply to all recipients")
	emailReplyCmd.Flags().StringVar(&replyBody, "body", "", "Reply body (reads stdin if omitted)")
	emailReplyCmd.Flags().BoolVar(&replyHTML, "html", false, "Send body as HTML")

	emailCmd.AddCommand(emailForwardCmd)
	emailForwardCmd.Flags().StringSliceVar(&forwardTo, "to", nil, "Forward recipient (required, repeatable)")
	emailForwardCmd.Flags().StringVar(&replyBody, "body", "", "Additional message (reads stdin if omitted)")
	emailForwardCmd.MarkFlagRequired("to")
}

func runEmailReply(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	accountID := c.MailAccountID()
	originalID := jmap.ID(args[0])

	// Fetch the original email
	original, err := fetchEmailForReply(c, originalID)
	if err != nil {
		return err
	}

	// Read reply body
	body := replyBody
	if body == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read body from stdin: %w", err)
		}
		body = string(data)
	}

	// Get identity
	identFlag := sendIdentity
	if identFlag == "" && cfg != nil {
		identFlag = cfg.DefaultIdentity
	}
	identityID, err := resolveIdentity(c, identFlag)
	if err != nil {
		return err
	}

	draftsID, err := findMailboxByRole(c, mailbox.RoleDrafts)
	if err != nil {
		return fmt.Errorf("could not find Drafts mailbox: %w", err)
	}

	// Build reply headers
	subject := original.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	// Reply-To: original sender (or reply-to all)
	var toAddrs []*mail.Address
	if len(original.ReplyTo) > 0 {
		toAddrs = original.ReplyTo
	} else {
		toAddrs = original.From
	}

	var ccAddrs []*mail.Address
	if replyAll {
		// Add original To and CC minus our own address
		ccAddrs = append(ccAddrs, original.To...)
		ccAddrs = append(ccAddrs, original.CC...)
	}

	// Build In-Reply-To and References
	var inReplyTo []string
	if len(original.MessageID) > 0 {
		inReplyTo = original.MessageID
	}
	references := append(original.References, original.MessageID...)

	newEmail := &email.Email{
		MailboxIDs: map[jmap.ID]bool{draftsID: true},
		To:         toAddrs,
		CC:         ccAddrs,
		Subject:    subject,
		InReplyTo:  inReplyTo,
		References: references,
		Keywords:   map[string]bool{"$draft": true},
		BodyValues: map[string]*email.BodyValue{
			"body": {Value: body},
		},
	}

	if replyHTML {
		newEmail.HTMLBody = []*email.BodyPart{
			{PartID: "body", Type: "text/html"},
		}
	} else {
		newEmail.TextBody = []*email.BodyPart{
			{PartID: "body", Type: "text/plain"},
		}
	}

	return sendEmail(c, accountID, identityID, draftsID, newEmail)
}

func runEmailForward(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	accountID := c.MailAccountID()
	originalID := jmap.ID(args[0])

	// Fetch the original email with body
	original, err := fetchEmailForReply(c, originalID)
	if err != nil {
		return err
	}

	// Read additional body
	body := replyBody
	if body == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read body from stdin: %w", err)
		}
		body = string(data)
	}

	// Get identity
	identFlag := sendIdentity
	if identFlag == "" && cfg != nil {
		identFlag = cfg.DefaultIdentity
	}
	identityID, err := resolveIdentity(c, identFlag)
	if err != nil {
		return err
	}

	draftsID, err := findMailboxByRole(c, mailbox.RoleDrafts)
	if err != nil {
		return fmt.Errorf("could not find Drafts mailbox: %w", err)
	}

	// Build forward headers
	subject := original.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "fwd:") {
		subject = "Fwd: " + subject
	}

	toAddrs := make([]*mail.Address, len(forwardTo))
	for i, addr := range forwardTo {
		toAddrs[i] = parseAddress(addr)
	}

	// Build forwarded body with original message inline
	var forwardBody string
	originalBody := extractTextBody(original)
	fromStr := ""
	if len(original.From) > 0 {
		fromStr = formatAddress(original.From[0])
	}
	forwardBody = body + "\n\n---------- Forwarded message ----------\n"
	forwardBody += fmt.Sprintf("From: %s\n", fromStr)
	forwardBody += fmt.Sprintf("Subject: %s\n", original.Subject)
	if original.ReceivedAt != nil {
		forwardBody += fmt.Sprintf("Date: %s\n", original.ReceivedAt.Format("2006-01-02 15:04"))
	}
	forwardBody += "\n" + originalBody

	references := append(original.References, original.MessageID...)

	newEmail := &email.Email{
		MailboxIDs: map[jmap.ID]bool{draftsID: true},
		To:         toAddrs,
		Subject:    subject,
		References: references,
		Keywords:   map[string]bool{"$draft": true},
		BodyValues: map[string]*email.BodyValue{
			"body": {Value: forwardBody},
		},
		TextBody: []*email.BodyPart{
			{PartID: "body", Type: "text/plain"},
		},
	}

	return sendEmail(c, accountID, identityID, draftsID, newEmail)
}

func fetchEmailForReply(c *client.Client, emailID jmap.ID) (*email.Email, error) {
	req := &jmap.Request{}
	req.Invoke(&email.Get{
		Account:             c.MailAccountID(),
		IDs:                 []jmap.ID{emailID},
		FetchTextBodyValues: true,
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch email: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return nil, fmt.Errorf("JMAP error: %s", r.Error())
		case *email.GetResponse:
			if len(r.List) == 0 {
				return nil, fmt.Errorf("email not found: %s", emailID)
			}
			return r.List[0], nil
		}
	}

	return nil, fmt.Errorf("unexpected response")
}

func extractTextBody(e *email.Email) string {
	var parts []string
	for _, part := range e.TextBody {
		if bv, ok := e.BodyValues[part.PartID]; ok {
			parts = append(parts, bv.Value)
		}
	}
	return strings.Join(parts, "\n")
}

func sendEmail(c *client.Client, accountID, identityID, draftsID jmap.ID, newEmail *email.Email) error {
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
				"keywords/$draft":                       nil,
				"keywords/$seen":                        true,
				"mailboxIds/" + string(draftsID):        nil,
			},
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("send failed: %w", err)
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

	fmt.Fprintln(os.Stderr, "Email sent.")
	return nil
}
