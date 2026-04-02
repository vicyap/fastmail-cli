package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/output"
	"github.com/vicyap/fastmail-cli/internal/searchsnippet"
)

var (
	emailMailbox     string
	emailLimit       uint64
	emailFrom        string
	emailTo          string
	emailSubjectFlag string
	emailBefore      string
	emailAfter       string
	emailHas         string
	emailShowHTML    bool
	emailShowHeaders bool
)

var emailCmd = &cobra.Command{
	Use:   "email",
	Short: "Manage emails",
}

var emailListCmd = &cobra.Command{
	Use:   "list",
	Short: "List emails in a mailbox",
	RunE:  runEmailList,
}

var emailSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search emails",
	Args:  cobra.ExactArgs(1),
	RunE:  runEmailSearch,
}

var emailReadCmd = &cobra.Command{
	Use:   "read <id>",
	Short: "Read an email",
	Args:  cobra.ExactArgs(1),
	RunE:  runEmailRead,
}

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "List recent inbox emails",
	RunE:  runInbox,
}

func init() {
	rootCmd.AddCommand(emailCmd)
	rootCmd.AddCommand(inboxCmd)

	emailCmd.AddCommand(emailListCmd)
	emailListCmd.Flags().StringVar(&emailMailbox, "mailbox", "Inbox", "Mailbox name or ID")
	emailListCmd.Flags().Uint64Var(&emailLimit, "limit", 20, "Max results")

	emailCmd.AddCommand(emailSearchCmd)
	emailSearchCmd.Flags().StringVar(&emailFrom, "from", "", "Filter by sender")
	emailSearchCmd.Flags().StringVar(&emailTo, "to", "", "Filter by recipient")
	emailSearchCmd.Flags().StringVar(&emailSubjectFlag, "subject", "", "Filter by subject")
	emailSearchCmd.Flags().StringVar(&emailBefore, "before", "", "Received before date (YYYY-MM-DD)")
	emailSearchCmd.Flags().StringVar(&emailAfter, "after", "", "Received after date (YYYY-MM-DD)")
	emailSearchCmd.Flags().StringVar(&emailHas, "has", "", "Filter (e.g., 'attachment')")
	emailSearchCmd.Flags().StringVar(&emailMailbox, "mailbox", "", "Restrict to mailbox")
	emailSearchCmd.Flags().Uint64Var(&emailLimit, "limit", 20, "Max results")

	emailCmd.AddCommand(emailReadCmd)
	emailReadCmd.Flags().BoolVar(&emailShowHTML, "html", false, "Show HTML body")
	emailReadCmd.Flags().BoolVar(&emailShowHeaders, "headers", false, "Show all headers")
}

var emailListProperties = []string{
	"id", "threadId", "mailboxIds", "from", "to", "subject", "receivedAt", "preview",
}

func runInbox(cmd *cobra.Command, args []string) error {
	emailMailbox = "Inbox"
	emailLimit = 20
	return runEmailList(cmd, args)
}

func runEmailList(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	mailboxID, err := findMailboxByName(c, emailMailbox)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	queryCallID := req.Invoke(&email.Query{
		Account: c.MailAccountID(),
		Filter: &email.FilterCondition{
			InMailbox: mailboxID,
		},
		Sort: []*email.SortComparator{
			{Property: "receivedAt", IsAscending: false},
		},
		Limit: emailLimit,
	})

	req.Invoke(&email.Get{
		Account:    c.MailAccountID(),
		Properties: emailListProperties,
		ReferenceIDs: &jmap.ResultReference{
			ResultOf: queryCallID,
			Name:     "Email/query",
			Path:     "/ids",
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("email list failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *email.GetResponse:
			return printEmails(r.List)
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func runEmailSearch(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	filter := &email.FilterCondition{
		Text: args[0],
	}

	if emailFrom != "" {
		filter.From = emailFrom
	}
	if emailTo != "" {
		filter.To = emailTo
	}
	if emailSubjectFlag != "" {
		filter.Subject = emailSubjectFlag
	}
	if emailBefore != "" {
		t, err := time.Parse("2006-01-02", emailBefore)
		if err != nil {
			return fmt.Errorf("invalid --before date: %w", err)
		}
		filter.Before = &t
	}
	if emailAfter != "" {
		t, err := time.Parse("2006-01-02", emailAfter)
		if err != nil {
			return fmt.Errorf("invalid --after date: %w", err)
		}
		filter.After = &t
	}
	if emailHas == "attachment" {
		filter.HasAttachment = true
	}
	if emailMailbox != "" {
		mailboxID, err := findMailboxByName(c, emailMailbox)
		if err != nil {
			return err
		}
		filter.InMailbox = mailboxID
	}

	req := &jmap.Request{}
	queryCallID := req.Invoke(&email.Query{
		Account: c.MailAccountID(),
		Filter:  filter,
		Sort: []*email.SortComparator{
			{Property: "receivedAt", IsAscending: false},
		},
		Limit: emailLimit,
	})

	req.Invoke(&email.Get{
		Account:    c.MailAccountID(),
		Properties: emailListProperties,
		ReferenceIDs: &jmap.ResultReference{
			ResultOf: queryCallID,
			Name:     "Email/query",
			Path:     "/ids",
		},
	})

	req.Invoke(&searchsnippet.Get{
		Account: c.MailAccountID(),
		Filter:  filter,
		ReferenceIDs: &jmap.ResultReference{
			ResultOf: queryCallID,
			Name:     "Email/query",
			Path:     "/ids",
		},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("email search failed: %w", err)
	}

	var emails []*email.Email
	var snippets []*searchsnippet.SearchSnippet

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *email.GetResponse:
			emails = r.List
		case *searchsnippet.GetResponse:
			snippets = r.List
		}
	}

	return printSearchResults(emails, snippets)
}

func runEmailRead(cmd *cobra.Command, args []string) error {
	c, err := client.New(tokenFlag)
	if err != nil {
		return err
	}

	req := &jmap.Request{}
	req.Invoke(&email.Get{
		Account:             c.MailAccountID(),
		IDs:                 []jmap.ID{jmap.ID(args[0])},
		FetchTextBodyValues: !emailShowHTML,
		FetchHTMLBodyValues: emailShowHTML,
		BodyProperties:      []string{"partId", "blobId", "size", "name", "type", "charset", "disposition", "cid"},
	})

	resp, err := c.JMAP.Do(req)
	if err != nil {
		return fmt.Errorf("email read failed: %w", err)
	}

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			return fmt.Errorf("JMAP error: %s", r.Error())
		case *email.GetResponse:
			if len(r.List) == 0 {
				return fmt.Errorf("email not found: %s", args[0])
			}
			if jsonOutput {
				return output.PrintJSON(r.List[0])
			}
			return output.Pager(func(w io.Writer) error {
				return writeEmailFull(w, r.List[0])
			})
		}
	}

	return fmt.Errorf("unexpected response from server")
}

func printEmails(emails []*email.Email) error {
	if jsonOutput {
		return output.PrintJSON(emails)
	}

	tbl := output.NewTable(os.Stdout)
	tbl.Headers("ID", "FROM", "SUBJECT", "DATE")
	for _, e := range emails {
		from := ""
		if len(e.From) > 0 {
			from = formatAddress(e.From[0])
		}
		date := ""
		if e.ReceivedAt != nil {
			date = e.ReceivedAt.Format("2006-01-02 15:04")
		}
		tbl.Row(string(e.ID), from, e.Subject, date)
	}
	return tbl.Flush()
}

func printSearchResults(emails []*email.Email, snippets []*searchsnippet.SearchSnippet) error {
	if jsonOutput {
		return output.PrintJSON(map[string]any{
			"emails":   emails,
			"snippets": snippets,
		})
	}

	snippetMap := make(map[jmap.ID]*searchsnippet.SearchSnippet)
	for _, s := range snippets {
		snippetMap[s.Email] = s
	}

	tbl := output.NewTable(os.Stdout)
	tbl.Headers("ID", "FROM", "SUBJECT", "DATE", "SNIPPET")
	for _, e := range emails {
		from := ""
		if len(e.From) > 0 {
			from = formatAddress(e.From[0])
		}
		date := ""
		if e.ReceivedAt != nil {
			date = e.ReceivedAt.Format("2006-01-02 15:04")
		}
		snippet := ""
		if s, ok := snippetMap[e.ID]; ok && s.Preview != "" {
			snippet = s.Preview
		}
		tbl.Row(string(e.ID), from, e.Subject, date, snippet)
	}
	return tbl.Flush()
}

var headerLabel = color.New(color.FgCyan, color.Bold)

func writeEmailFull(w io.Writer, e *email.Email) error {
	if emailShowHeaders {
		for _, h := range e.Headers {
			fmt.Fprintf(w, "%s: %s\n", headerLabel.Sprint(h.Name), h.Value)
		}
		fmt.Fprintln(w)
	} else {
		fmt.Fprintf(w, "%s %s\n", headerLabel.Sprint("From:   "), formatAddresses(e.From))
		fmt.Fprintf(w, "%s %s\n", headerLabel.Sprint("To:     "), formatAddresses(e.To))
		if len(e.CC) > 0 {
			fmt.Fprintf(w, "%s %s\n", headerLabel.Sprint("CC:     "), formatAddresses(e.CC))
		}
		fmt.Fprintf(w, "%s %s\n", headerLabel.Sprint("Subject:"), e.Subject)
		if e.ReceivedAt != nil {
			fmt.Fprintf(w, "%s %s\n", headerLabel.Sprint("Date:   "), e.ReceivedAt.Format(time.RFC1123))
		}

		if len(e.Attachments) > 0 {
			fmt.Fprintf(w, "%s\n", headerLabel.Sprint("Attachments:"))
			for _, att := range e.Attachments {
				name := att.Name
				if name == "" {
					name = "(unnamed)"
				}
				fmt.Fprintf(w, "  - %s (%s, %d bytes, blob:%s)\n", name, att.Type, att.Size, att.BlobID)
			}
		}

		fmt.Fprintln(w)
	}

	if emailShowHTML {
		for _, part := range e.HTMLBody {
			if bv, ok := e.BodyValues[part.PartID]; ok {
				fmt.Fprint(w, bv.Value)
			}
		}
	} else {
		for _, part := range e.TextBody {
			if bv, ok := e.BodyValues[part.PartID]; ok {
				fmt.Fprint(w, bv.Value)
			}
		}
	}

	return nil
}

func formatAddress(addr *mail.Address) string {
	if addr.Name != "" {
		return fmt.Sprintf("%s <%s>", addr.Name, addr.Email)
	}
	return addr.Email
}

func formatAddresses(addrs []*mail.Address) string {
	parts := make([]string, len(addrs))
	for i, a := range addrs {
		parts[i] = formatAddress(a)
	}
	return strings.Join(parts, ", ")
}
