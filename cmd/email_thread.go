package cmd

import (
	"fmt"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/thread"
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

	return printThread(emails)
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

func printThread(emails []*email.Email) error {
	if jsonOutput {
		return output.PrintJSON(emails)
	}

	for i, e := range emails {
		if i > 0 {
			fmt.Println(strings.Repeat("─", 72))
		}

		fmt.Printf("From:    %s\n", formatAddresses(e.From))
		fmt.Printf("To:      %s\n", formatAddresses(e.To))
		if len(e.CC) > 0 {
			fmt.Printf("CC:      %s\n", formatAddresses(e.CC))
		}
		fmt.Printf("Subject: %s\n", e.Subject)
		if e.ReceivedAt != nil {
			fmt.Printf("Date:    %s\n", e.ReceivedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Printf("ID:      %s\n", e.ID)

		if len(e.Attachments) > 0 {
			fmt.Printf("Attachments:\n")
			for _, att := range e.Attachments {
				name := att.Name
				if name == "" {
					name = "(unnamed)"
				}
				fmt.Printf("  - %s (%s, %d bytes)\n", name, att.Type, att.Size)
			}
		}

		fmt.Println()

		for _, part := range e.TextBody {
			if bv, ok := e.BodyValues[part.PartID]; ok {
				fmt.Print(bv.Value)
				if !strings.HasSuffix(bv.Value, "\n") {
					fmt.Println()
				}
			}
		}
	}

	return nil
}
