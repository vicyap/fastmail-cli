package cmd

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
	"github.com/vicyap/fastmail-cli/internal/searchsnippet"
)

func emailListHandler(t *testing.T) jmaptest.APIHandler {
	return func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "mbox-inbox", "name": "Inbox", "role": "inbox"},
					},
				}, call.CallID))
			case "Email/query":
				responses = append(responses, jmaptest.MethodResponse("Email/query", map[string]any{
					"accountId":  jmaptest.TestAccountID,
					"queryState": "qs1",
					"ids":        []string{"email-1", "email-2"},
				}, call.CallID))
			case "Email/get":
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{
							"id":         "email-1",
							"threadId":   "thread-1",
							"subject":    "Hello World",
							"from":       []map[string]any{{"name": "Alice", "email": "alice@example.com"}},
							"to":         []map[string]any{{"name": "Bob", "email": "bob@example.com"}},
							"receivedAt": "2026-03-15T10:30:00Z",
							"preview":    "This is a preview",
						},
						{
							"id":         "email-2",
							"threadId":   "thread-2",
							"subject":    "Meeting Notes",
							"from":       []map[string]any{{"name": "Carol", "email": "carol@example.com"}},
							"to":         []map[string]any{{"name": "Bob", "email": "bob@example.com"}},
							"receivedAt": "2026-03-14T09:00:00Z",
							"preview":    "Meeting notes from today",
						},
					},
				}, call.CallID))
			}
		}
		return responses
	}
}

func TestEmailList_RequestConstruction(t *testing.T) {
	var capturedCalls []struct {
		Name   string
		Args   map[string]any
		CallID string
	}

	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		capturedCalls = append(capturedCalls, calls...)

		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "mbox-inbox", "name": "Inbox", "role": "inbox"},
					},
				}, call.CallID))
			case "Email/query":
				// Verify filter and sort
				filter, _ := call.Args["filter"].(map[string]any)
				assert.Equal(t, "mbox-inbox", filter["inMailbox"])

				sort, _ := call.Args["sort"].([]any)
				require.Len(t, sort, 1)
				sortObj := sort[0].(map[string]any)
				assert.Equal(t, "receivedAt", sortObj["property"])
				assert.Equal(t, false, sortObj["isAscending"])

				responses = append(responses, jmaptest.MethodResponse("Email/query", map[string]any{
					"accountId":  jmaptest.TestAccountID,
					"queryState": "qs1",
					"ids":        []string{"email-1"},
				}, call.CallID))
			case "Email/get":
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{
							"id":      "email-1",
							"subject": "Test",
							"from":    []map[string]any{{"email": "test@example.com"}},
						},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	accountID := jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:mail"]

	// First get mailbox
	req1 := &jmap.Request{}
	req1.Invoke(&email.Query{
		Account: accountID,
		Filter:  &email.FilterCondition{InMailbox: "mbox-inbox"},
		Sort:    []*email.SortComparator{{Property: "receivedAt", IsAscending: false}},
		Limit:   20,
	})
	_, err := jmapClient.Do(req1)
	require.NoError(t, err)
}

func TestEmailSearch_WithFilters(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Email/query":
				filter := call.Args["filter"].(map[string]any)
				assert.Equal(t, "meeting notes", filter["text"])
				assert.Equal(t, "alice@example.com", filter["from"])

				responses = append(responses, jmaptest.MethodResponse("Email/query", map[string]any{
					"accountId":  jmaptest.TestAccountID,
					"queryState": "qs1",
					"ids":        []string{"email-1"},
				}, call.CallID))
			case "Email/get":
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{
							"id":      "email-1",
							"subject": "Meeting Notes",
							"from":    []map[string]any{{"name": "Alice", "email": "alice@example.com"}},
						},
					},
				}, call.CallID))
			case "SearchSnippet/get":
				responses = append(responses, jmaptest.MethodResponse("SearchSnippet/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"list": []map[string]any{
						{
							"emailId": "email-1",
							"subject": nil,
							"preview": "...meeting <mark>notes</mark>...",
						},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())
	accountID := jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:mail"]

	req := &jmap.Request{}
	queryCallID := req.Invoke(&email.Query{
		Account: accountID,
		Filter: &email.FilterCondition{
			Text: "meeting notes",
			From: "alice@example.com",
		},
		Sort:  []*email.SortComparator{{Property: "receivedAt", IsAscending: false}},
		Limit: 20,
	})

	req.Invoke(&email.Get{
		Account:    accountID,
		Properties: emailListProperties,
		ReferenceIDs: &jmap.ResultReference{
			ResultOf: queryCallID,
			Name:     "Email/query",
			Path:     "/ids",
		},
	})

	req.Invoke(&searchsnippet.Get{
		Account: accountID,
		Filter: &email.FilterCondition{
			Text: "meeting notes",
			From: "alice@example.com",
		},
		ReferenceIDs: &jmap.ResultReference{
			ResultOf: queryCallID,
			Name:     "Email/query",
			Path:     "/ids",
		},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	var emails []*email.Email
	var snippets []*searchsnippet.SearchSnippet

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *email.GetResponse:
			emails = r.List
		case *searchsnippet.GetResponse:
			snippets = r.List
		}
	}

	require.Len(t, emails, 1)
	assert.Equal(t, "Meeting Notes", emails[0].Subject)
	require.Len(t, snippets, 1)
	assert.Contains(t, snippets[0].Preview, "meeting")
}

func TestEmailRead_FullBody(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/get" {
				ids := call.Args["ids"].([]any)
				assert.Equal(t, "email-123", ids[0])
				assert.Equal(t, true, call.Args["fetchTextBodyValues"])

				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{
							"id":       "email-123",
							"subject":  "Test Email",
							"from":     []map[string]any{{"name": "Alice", "email": "alice@example.com"}},
							"to":       []map[string]any{{"name": "Bob", "email": "bob@example.com"}},
							"textBody": []map[string]any{{"partId": "1"}},
							"bodyValues": map[string]any{
								"1": map[string]any{
									"value": "Hello, this is the email body.",
								},
							},
							"receivedAt": "2026-03-15T10:30:00Z",
						},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())
	accountID := jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:mail"]

	req := &jmap.Request{}
	req.Invoke(&email.Get{
		Account:             accountID,
		IDs:                 []jmap.ID{"email-123"},
		FetchTextBodyValues: true,
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*email.GetResponse)
	require.Len(t, r.List, 1)
	assert.Equal(t, "Test Email", r.List[0].Subject)
	assert.Contains(t, r.List[0].BodyValues["1"].Value, "Hello, this is the email body.")
}

func TestEmailRead_NotFound(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/get" {
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{},
					"notFound":  []string{"nonexistent"},
				}, call.CallID))
			}
		}
		return responses
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())
	accountID := jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:mail"]

	req := &jmap.Request{}
	req.Invoke(&email.Get{
		Account: accountID,
		IDs:     []jmap.ID{"nonexistent"},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*email.GetResponse)
	assert.Empty(t, r.List)
	assert.Contains(t, r.NotFound, jmap.ID("nonexistent"))
}

func TestFormatAddresses(t *testing.T) {
	tests := []struct {
		name     string
		addrs    []*mail.Address
		expected string
	}{
		{
			name:     "single address with name",
			addrs:    []*mail.Address{{Name: "Alice", Email: "alice@example.com"}},
			expected: "Alice <alice@example.com>",
		},
		{
			name:     "single address without name",
			addrs:    []*mail.Address{{Email: "alice@example.com"}},
			expected: "alice@example.com",
		},
		{
			name: "multiple addresses",
			addrs: []*mail.Address{
				{Name: "Alice", Email: "alice@example.com"},
				{Email: "bob@example.com"},
				{Name: "Carol", Email: "carol@example.com"},
			},
			expected: "Alice <alice@example.com>, bob@example.com, Carol <carol@example.com>",
		},
		{
			name:     "empty list",
			addrs:    []*mail.Address{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAddresses(tt.addrs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrintEmails_Table(t *testing.T) {
	now := time.Now()
	emails := []*email.Email{
		{
			ID:         "email-1",
			From:       []*mail.Address{{Name: "Alice", Email: "alice@example.com"}},
			Subject:    "Hello World",
			ReceivedAt: &now,
		},
		{
			ID:      "email-2",
			From:    []*mail.Address{{Email: "bob@example.com"}},
			Subject: "No date email",
		},
		{
			ID:      "email-3",
			Subject: "No sender email",
		},
	}

	// Ensure non-JSON output mode
	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := printEmails(emails)
	assert.NoError(t, err)
}

func TestPrintEmails_JSON(t *testing.T) {
	emails := []*email.Email{
		{
			ID:      "email-1",
			Subject: "Test",
		},
	}

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := printEmails(emails)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "email-1")
}

func TestPrintSearchResults_Table(t *testing.T) {
	now := time.Now()
	emails := []*email.Email{
		{
			ID:         "email-1",
			From:       []*mail.Address{{Name: "Alice", Email: "alice@example.com"}},
			Subject:    "Meeting Notes",
			ReceivedAt: &now,
		},
		{
			ID:      "email-2",
			Subject: "No sender or date",
		},
	}

	snippets := []*searchsnippet.SearchSnippet{
		{
			Email:   "email-1",
			Preview: "found <mark>match</mark> here",
		},
	}

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := printSearchResults(emails, snippets)
	assert.NoError(t, err)
}

func TestPrintSearchResults_JSON(t *testing.T) {
	emails := []*email.Email{
		{ID: "email-1", Subject: "Test"},
	}
	snippets := []*searchsnippet.SearchSnippet{
		{Email: "email-1", Preview: "snippet text"},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := printSearchResults(emails, snippets)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "emails")
	assert.Contains(t, buf.String(), "snippets")
}

func TestWriteEmailFull_TextBody(t *testing.T) {
	now := time.Now()
	e := &email.Email{
		From:       []*mail.Address{{Name: "Alice", Email: "alice@example.com"}},
		To:         []*mail.Address{{Name: "Bob", Email: "bob@example.com"}},
		CC:         []*mail.Address{{Email: "carol@example.com"}},
		Subject:    "Test Subject",
		ReceivedAt: &now,
		TextBody:   []*email.BodyPart{{PartID: "1"}},
		BodyValues: map[string]*email.BodyValue{
			"1": {Value: "Hello, this is the body."},
		},
		Attachments: []*email.BodyPart{
			{Name: "file.pdf", Type: "application/pdf", Size: 1234, BlobID: "blob-1"},
			{Type: "application/octet-stream", Size: 500, BlobID: "blob-2"},
		},
	}

	oldShowHTML := emailShowHTML
	oldShowHeaders := emailShowHeaders
	emailShowHTML = false
	emailShowHeaders = false
	defer func() {
		emailShowHTML = oldShowHTML
		emailShowHeaders = oldShowHeaders
	}()

	var buf bytes.Buffer
	err := writeEmailFull(&buf, e)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "alice@example.com")
	assert.Contains(t, out, "bob@example.com")
	assert.Contains(t, out, "carol@example.com")
	assert.Contains(t, out, "Test Subject")
	assert.Contains(t, out, "Hello, this is the body.")
	assert.Contains(t, out, "file.pdf")
	assert.Contains(t, out, "(unnamed)")
}

func TestWriteEmailFull_HTMLBody(t *testing.T) {
	e := &email.Email{
		From:     []*mail.Address{{Email: "alice@example.com"}},
		To:       []*mail.Address{{Email: "bob@example.com"}},
		Subject:  "HTML Email",
		HTMLBody: []*email.BodyPart{{PartID: "1"}},
		BodyValues: map[string]*email.BodyValue{
			"1": {Value: "<p>Hello HTML</p>"},
		},
	}

	oldShowHTML := emailShowHTML
	oldShowHeaders := emailShowHeaders
	emailShowHTML = true
	emailShowHeaders = false
	defer func() {
		emailShowHTML = oldShowHTML
		emailShowHeaders = oldShowHeaders
	}()

	var buf bytes.Buffer
	err := writeEmailFull(&buf, e)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "<p>Hello HTML</p>")
}

func TestWriteEmailFull_Headers(t *testing.T) {
	e := &email.Email{
		Headers: []*email.Header{
			{Name: "From", Value: "alice@example.com"},
			{Name: "To", Value: "bob@example.com"},
			{Name: "X-Custom", Value: "custom-value"},
		},
		TextBody: []*email.BodyPart{{PartID: "1"}},
		BodyValues: map[string]*email.BodyValue{
			"1": {Value: "body text"},
		},
	}

	oldShowHTML := emailShowHTML
	oldShowHeaders := emailShowHeaders
	emailShowHTML = false
	emailShowHeaders = true
	defer func() {
		emailShowHTML = oldShowHTML
		emailShowHeaders = oldShowHeaders
	}()

	var buf bytes.Buffer
	err := writeEmailFull(&buf, e)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "From")
	assert.Contains(t, out, "X-Custom")
	assert.Contains(t, out, "custom-value")
	assert.Contains(t, out, "body text")
}

func TestFormatAddress(t *testing.T) {
	tests := []struct {
		name     string
		addr     *mail.Address
		expected string
	}{
		{
			name:     "name and email",
			addr:     &mail.Address{Name: "Alice", Email: "alice@example.com"},
			expected: "Alice <alice@example.com>",
		},
		{
			name:     "email only",
			addr:     &mail.Address{Email: "alice@example.com"},
			expected: "alice@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAddress(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRunEmailList(t *testing.T) {
	withTestClient(t, emailListHandler(t))

	oldJSON := jsonOutput
	jsonOutput = false
	oldMailbox := emailMailbox
	emailMailbox = "Inbox"
	oldCfg := cfg
	cfg = nil // No config so it uses emailMailbox directly
	defer func() {
		jsonOutput = oldJSON
		emailMailbox = oldMailbox
		cfg = oldCfg
	}()

	// Create a minimal cobra command with the flag registered
	cmd := &cobra.Command{}
	cmd.Flags().String("mailbox", "Inbox", "Mailbox")

	err := runEmailList(cmd, nil)
	assert.NoError(t, err)
}

func TestRunEmailRead(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/get" {
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{
							"id":       "email-123",
							"subject":  "Test Email",
							"from":     []map[string]any{{"email": "alice@example.com"}},
							"to":       []map[string]any{{"email": "bob@example.com"}},
							"textBody": []map[string]any{{"partId": "1"}},
							"bodyValues": map[string]any{
								"1": map[string]any{"value": "Hello body"},
							},
							"receivedAt": "2026-03-15T10:30:00Z",
						},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	// JSON mode to avoid Pager
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := runEmailRead(nil, []string{"email-123"})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "email-123")
}

func TestRunEmailRead_NotFound(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/get" {
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runEmailRead(nil, []string{"nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email not found")
}

func TestRunEmailSearch(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "mbox-inbox", "name": "Inbox"},
					},
				}, call.CallID))
			case "Email/query":
				responses = append(responses, jmaptest.MethodResponse("Email/query", map[string]any{
					"accountId":  jmaptest.TestAccountID,
					"queryState": "qs1",
					"ids":        []string{"email-1"},
				}, call.CallID))
			case "Email/get":
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "email-1", "subject": "Found it", "from": []map[string]any{{"email": "alice@example.com"}}},
					},
				}, call.CallID))
			case "SearchSnippet/get":
				responses = append(responses, jmaptest.MethodResponse("SearchSnippet/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"list": []map[string]any{
						{"emailId": "email-1", "preview": "matched text"},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	oldFrom := emailFrom
	oldTo := emailTo
	oldSubject := emailSubjectFlag
	oldBefore := emailBefore
	oldAfter := emailAfter
	oldHas := emailHas
	oldMailbox := emailMailbox
	emailFrom = "alice@example.com"
	emailTo = "bob@example.com"
	emailSubjectFlag = "meeting"
	emailBefore = "2026-04-01"
	emailAfter = "2026-03-01"
	emailHas = "attachment"
	emailMailbox = "Inbox"
	defer func() {
		jsonOutput = oldJSON
		emailFrom = oldFrom
		emailTo = oldTo
		emailSubjectFlag = oldSubject
		emailBefore = oldBefore
		emailAfter = oldAfter
		emailHas = oldHas
		emailMailbox = oldMailbox
	}()

	err := runEmailSearch(nil, []string{"test query"})
	assert.NoError(t, err)
}

func TestRunEmailSearch_InvalidBefore(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	oldBefore := emailBefore
	emailBefore = "not-a-date"
	oldAfter := emailAfter
	emailAfter = ""
	oldFrom := emailFrom
	emailFrom = ""
	oldTo := emailTo
	emailTo = ""
	oldSubject := emailSubjectFlag
	emailSubjectFlag = ""
	oldHas := emailHas
	emailHas = ""
	oldMailbox := emailMailbox
	emailMailbox = ""
	defer func() {
		emailBefore = oldBefore
		emailAfter = oldAfter
		emailFrom = oldFrom
		emailTo = oldTo
		emailSubjectFlag = oldSubject
		emailHas = oldHas
		emailMailbox = oldMailbox
	}()

	err := runEmailSearch(nil, []string{"test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --before date")
}

func TestRunEmailSearch_InvalidAfter(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	oldBefore := emailBefore
	emailBefore = ""
	oldAfter := emailAfter
	emailAfter = "not-a-date"
	oldFrom := emailFrom
	emailFrom = ""
	oldTo := emailTo
	emailTo = ""
	oldSubject := emailSubjectFlag
	emailSubjectFlag = ""
	oldHas := emailHas
	emailHas = ""
	oldMailbox := emailMailbox
	emailMailbox = ""
	defer func() {
		emailBefore = oldBefore
		emailAfter = oldAfter
		emailFrom = oldFrom
		emailTo = oldTo
		emailSubjectFlag = oldSubject
		emailHas = oldHas
		emailMailbox = oldMailbox
	}()

	err := runEmailSearch(nil, []string{"test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --after date")
}

func TestRunInbox(t *testing.T) {
	withTestClient(t, emailListHandler(t))

	oldJSON := jsonOutput
	jsonOutput = false
	oldCfg := cfg
	cfg = nil
	defer func() {
		jsonOutput = oldJSON
		cfg = oldCfg
	}()

	cmd := &cobra.Command{}
	cmd.Flags().String("mailbox", "Inbox", "Mailbox")

	err := runInbox(cmd, nil)
	assert.NoError(t, err)
}

func TestRunEmailList_JSON(t *testing.T) {
	withTestClient(t, emailListHandler(t))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true
	oldMailbox := emailMailbox
	emailMailbox = "Inbox"
	oldCfg := cfg
	cfg = nil
	defer func() {
		jsonOutput = oldJSON
		emailMailbox = oldMailbox
		cfg = oldCfg
	}()

	cmd := &cobra.Command{}
	cmd.Flags().String("mailbox", "Inbox", "Mailbox")

	err := runEmailList(cmd, nil)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "email-1")
}

func TestRunEmailRead_Pager(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/get" {
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{
							"id":       "email-123",
							"subject":  "Test Email",
							"from":     []map[string]any{{"email": "alice@example.com"}},
							"to":       []map[string]any{{"email": "bob@example.com"}},
							"textBody": []map[string]any{{"partId": "1"}},
							"bodyValues": map[string]any{
								"1": map[string]any{"value": "Hello body"},
							},
						},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Non-JSON mode triggers Pager path (falls through to stdout since no TTY)
	oldJSON := jsonOutput
	jsonOutput = false
	oldShowHTML := emailShowHTML
	emailShowHTML = false
	oldShowHeaders := emailShowHeaders
	emailShowHeaders = false

	err := runEmailRead(nil, []string{"email-123"})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON
	emailShowHTML = oldShowHTML
	emailShowHeaders = oldShowHeaders

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "alice@example.com")
	assert.Contains(t, buf.String(), "Hello body")
}

func TestRunEmailThread_NonJSON(t *testing.T) {
	withTestClient(t, threadHandler(t))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = false

	err := runEmailThread(nil, []string{"email-1"})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Original")
	assert.Contains(t, buf.String(), "Re: Original")
}

func TestRunEmailThread(t *testing.T) {
	withTestClient(t, threadHandler(t))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := runEmailThread(nil, []string{"email-1"})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "Original")
}

func TestResultReferences(t *testing.T) {
	// Test that result references are properly constructed for Email/query -> Email/get chain
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 2)

		assert.Equal(t, "Email/query", calls[0].Name)
		assert.Equal(t, "Email/get", calls[1].Name)

		// Verify the get call has a result reference to the query call
		getArgs := calls[1].Args
		refIDs, ok := getArgs["#ids"]
		require.True(t, ok, "Email/get should have #ids result reference")

		refMap := refIDs.(map[string]any)
		assert.Equal(t, calls[0].CallID, refMap["resultOf"])
		assert.Equal(t, "Email/query", refMap["name"])
		assert.Equal(t, "/ids", refMap["path"])

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("Email/query", map[string]any{
				"accountId":  jmaptest.TestAccountID,
				"queryState": "qs1",
				"ids":        []string{"email-1"},
			}, calls[0].CallID),
			jmaptest.MethodResponse("Email/get", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"state":     "s1",
				"list": []map[string]any{
					{"id": "email-1", "subject": "Test"},
				},
			}, calls[1].CallID),
		}
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())
	accountID := jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:mail"]

	req := &jmap.Request{}
	queryCallID := req.Invoke(&email.Query{
		Account: accountID,
		Filter:  &email.FilterCondition{InMailbox: "mbox-inbox"},
		Limit:   20,
	})

	req.Invoke(&email.Get{
		Account:    accountID,
		Properties: []string{"id", "subject"},
		ReferenceIDs: &jmap.ResultReference{
			ResultOf: queryCallID,
			Name:     "Email/query",
			Path:     "/ids",
		},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)
	require.Len(t, resp.Responses, 2)
}
