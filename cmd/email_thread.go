package cmd

import (
	"fmt"
	"io"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/thread"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/output"
)

var emailThreadCmd = &cobra.Command{
	Use:   "thread <id>",
	Short: "Display a full email thread",
	Long:  "Show all emails in a thread. Pass an email ID; the thread is resolved automatically.",
	Args:  cobra.ExactArgs(1),
	RunE:  runEmailThread,
}

func init() {
	emailCmd.AddCommand(emailThreadCmd)
}

func runEmailThread(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	emailID := jmap.ID(args[0])

	// Step 1: Get the threadId from the email
	threadID, err := getThreadID(c, emailID)
	if err != nil {
		return err
	}

	// Step 2: Get all email IDs in the thread
	threadEmailIDs, err := getThreadEmailIDs(c, threadID)
	if err != nil {
		return err
	}

	// Step 3: Fetch all emails in the thread
	emails, err := getEmailsByIDs(c, threadEmailIDs)
	if err != nil {
		return err
	}

	if jsonOutput {
		return output.PrintJSON(emails)
	}
	return output.Pager(func(w io.Writer) error {
		return writeThread(w, emails)
	})
}

func getThreadID(c *client.Client, emailID jmap.ID) (jmap.ID, error) {
	req := &jmap.Request{}
	req.Invoke(&email.Get{
		Account:    c.MailAccountID(),
		IDs:        []jmap.ID{emailID},
		Properties: []string{"threadId"},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get email: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return "", fmt.Errorf("JMAP error: %s", r.Error())
		case *email.GetResponse:
			if len(r.List) == 0 {
				return "", fmt.Errorf("email not found: %s", emailID)
			}
			return r.List[0].ThreadID, nil
		}
	}

	return "", fmt.Errorf("unexpected response")
}

func getThreadEmailIDs(c *client.Client, threadID jmap.ID) ([]jmap.ID, error) {
	req := &jmap.Request{}
	req.Invoke(&thread.Get{
		Account: c.MailAccountID(),
		IDs:     []jmap.ID{threadID},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return nil, fmt.Errorf("JMAP error: %s", r.Error())
		case *thread.GetResponse:
			if len(r.List) == 0 {
				return nil, fmt.Errorf("thread not found: %s", threadID)
			}
			return r.List[0].EmailIDs, nil
		}
	}

	return nil, fmt.Errorf("unexpected response")
}

func getEmailsByIDs(c *client.Client, ids []jmap.ID) ([]*email.Email, error) {
	req := &jmap.Request{}
	req.Invoke(&email.Get{
		Account:             c.MailAccountID(),
		IDs:                 ids,
		FetchTextBodyValues: true,
		BodyProperties:      []string{"partId", "blobId", "size", "name", "type", "charset", "disposition"},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get emails: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return nil, fmt.Errorf("JMAP error: %s", r.Error())
		case *email.GetResponse:
			return r.List, nil
		}
	}

	return nil, fmt.Errorf("unexpected response")
}

var threadSeparator = color.New(color.FgHiBlack)

func writeThread(w io.Writer, emails []*email.Email) error {
	for i, e := range emails {
		if i > 0 {
			fmt.Fprintln(w, threadSeparator.Sprint(strings.Repeat("─", 72)))
		}

		fmt.Fprintf(w, "%s %s\n", headerLabel.Sprint("From:   "), formatAddresses(e.From))
		fmt.Fprintf(w, "%s %s\n", headerLabel.Sprint("To:     "), formatAddresses(e.To))
		if len(e.CC) > 0 {
			fmt.Fprintf(w, "%s %s\n", headerLabel.Sprint("CC:     "), formatAddresses(e.CC))
		}
		fmt.Fprintf(w, "%s %s\n", headerLabel.Sprint("Subject:"), e.Subject)
		if e.ReceivedAt != nil {
			fmt.Fprintf(w, "%s %s\n", headerLabel.Sprint("Date:   "), e.ReceivedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Fprintf(w, "%s %s\n", headerLabel.Sprint("ID:     "), e.ID)

		if len(e.Attachments) > 0 {
			fmt.Fprintf(w, "%s\n", headerLabel.Sprint("Attachments:"))
			for _, att := range e.Attachments {
				name := att.Name
				if name == "" {
					name = "(unnamed)"
				}
				fmt.Fprintf(w, "  - %s (%s, %d bytes)\n", name, att.Type, att.Size)
			}
		}

		fmt.Fprintln(w)

		for _, part := range e.TextBody {
			if bv, ok := e.BodyValues[part.PartID]; ok {
				fmt.Fprint(w, bv.Value)
				if !strings.HasSuffix(bv.Value, "\n") {
					fmt.Fprintln(w)
				}
			}
		}
	}

	return nil
}
