package cmd

import (
	"net/http"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"github.com/vicyap/fastmail-cli/internal/searchsnippet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
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
