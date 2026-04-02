package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
)

func newTestClient(t *testing.T, handler jmaptest.APIHandler) *client.Client {
	t.Helper()
	server := jmaptest.NewServer(t, handler)
	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())
	return &client.Client{JMAP: jmapClient}
}

// withTestClient overrides client.New for the duration of a test so that
// run* functions use a fake JMAP server instead of Fastmail. It restores
// the original after the test completes.
func withTestClient(t *testing.T, handler jmaptest.APIHandler) {
	t.Helper()
	c := newTestClient(t, handler)
	orig := client.New
	client.New = func(string) (*client.Client, error) { return c, nil }
	t.Cleanup(func() { client.New = orig })
}

func mailboxHandler(t *testing.T, mailboxes []map[string]any) jmaptest.APIHandler {
	return func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Mailbox/get" {
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      mailboxes,
				}, call.CallID))
			}
		}
		return responses
	}
}

func TestFindMailboxByName(t *testing.T) {
	mailboxes := []map[string]any{
		{"id": "mbox-1", "name": "Inbox", "role": "inbox"},
		{"id": "mbox-2", "name": "Archive", "role": "archive"},
		{"id": "mbox-3", "name": "Sent", "role": "sent"},
	}

	c := newTestClient(t, mailboxHandler(t, mailboxes))

	id, err := findMailboxByName(c, "Archive")
	require.NoError(t, err)
	assert.Equal(t, jmap.ID("mbox-2"), id)
}

func TestFindMailboxByName_ByID(t *testing.T) {
	mailboxes := []map[string]any{
		{"id": "mbox-1", "name": "Inbox", "role": "inbox"},
	}

	c := newTestClient(t, mailboxHandler(t, mailboxes))

	id, err := findMailboxByName(c, "mbox-1")
	require.NoError(t, err)
	assert.Equal(t, jmap.ID("mbox-1"), id)
}

func TestFindMailboxByName_NotFound(t *testing.T) {
	c := newTestClient(t, mailboxHandler(t, []map[string]any{}))

	_, err := findMailboxByName(c, "NonExistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mailbox not found")
}

func TestFindMailboxByRole(t *testing.T) {
	mailboxes := []map[string]any{
		{"id": "mbox-1", "name": "Inbox", "role": "inbox"},
		{"id": "mbox-2", "name": "Trash", "role": "trash"},
	}

	c := newTestClient(t, mailboxHandler(t, mailboxes))

	id, err := findMailboxByRole(c, mailbox.RoleTrash)
	require.NoError(t, err)
	assert.Equal(t, jmap.ID("mbox-2"), id)
}

func TestFindMailboxByRole_NotFound(t *testing.T) {
	mailboxes := []map[string]any{
		{"id": "mbox-1", "name": "Inbox", "role": "inbox"},
	}

	c := newTestClient(t, mailboxHandler(t, mailboxes))

	_, err := findMailboxByRole(c, mailbox.RoleTrash)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no mailbox with role trash found")
}

func TestPrintMailboxes_Table(t *testing.T) {
	// Verify mailbox list request is constructed correctly
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 1)
		assert.Equal(t, "Mailbox/get", calls[0].Name)
		assert.Equal(t, jmaptest.TestAccountID, calls[0].Args["accountId"])

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("Mailbox/get", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"state":     "s1",
				"list": []map[string]any{
					{
						"id":           "mbox-1",
						"name":         "Inbox",
						"role":         "inbox",
						"unreadEmails": 5,
						"totalEmails":  42,
					},
				},
			}, calls[0].CallID),
		}
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	// Verify request construction
	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{
		Account: jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:mail"],
	})
	resp, err := jmapClient.Do(req)
	require.NoError(t, err)
	require.Len(t, resp.Responses, 1)

	r, ok := resp.Responses[0].Args.(*mailbox.GetResponse)
	require.True(t, ok)
	require.Len(t, r.List, 1)
	assert.Equal(t, "Inbox", r.List[0].Name)
	assert.Equal(t, mailbox.RoleInbox, r.List[0].Role)
	assert.Equal(t, uint64(5), r.List[0].UnreadEmails)
	assert.Equal(t, uint64(42), r.List[0].TotalEmails)
}

func TestPrintMailboxes_JSON(t *testing.T) {
	mailboxes := []*mailbox.Mailbox{
		{
			ID:           "mbox-1",
			Name:         "Inbox",
			Role:         mailbox.RoleInbox,
			UnreadEmails: 5,
			TotalEmails:  42,
		},
	}

	data, err := json.MarshalIndent(mailboxes, "", "  ")
	require.NoError(t, err)

	var decoded []map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Len(t, decoded, 1)
	assert.Equal(t, "Inbox", decoded[0]["name"])
	assert.Equal(t, "inbox", decoded[0]["role"])
}

func TestMailboxCreate(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Mailbox/set" {
				create := call.Args["create"].(map[string]any)
				new1 := create["new1"].(map[string]any)
				assert.Equal(t, "Projects", new1["name"])

				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"new1": map[string]any{
							"id":   "mbox-new",
							"name": "Projects",
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
	req.Invoke(&mailbox.Set{
		Account: accountID,
		Create: map[jmap.ID]*mailbox.Mailbox{
			"new1": {Name: "Projects"},
		},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*mailbox.SetResponse)
	require.Contains(t, r.Created, jmap.ID("new1"))
	assert.Equal(t, "Projects", r.Created["new1"].Name)
}

func TestMailboxRename(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "mbox-1", "name": "OldName"},
					},
				}, call.CallID))
			case "Mailbox/set":
				update := call.Args["update"].(map[string]any)
				patch := update["mbox-1"].(map[string]any)
				assert.Equal(t, "NewName", patch["name"])

				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"mbox-1": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	c := &client.Client{JMAP: &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}}
	require.NoError(t, c.JMAP.Authenticate())

	// Find mailbox
	id, err := findMailboxByName(c, "OldName")
	require.NoError(t, err)
	assert.Equal(t, jmap.ID("mbox-1"), id)
}

func TestFormatMailboxSetError_WithDescription(t *testing.T) {
	desc := "mailbox has children"
	err := &jmap.SetError{
		Type:        "mailboxHasChild",
		Description: &desc,
	}
	result := formatMailboxSetError(err)
	assert.Equal(t, "mailboxHasChild: mailbox has children", result)
}

func TestFormatMailboxSetError_WithoutDescription(t *testing.T) {
	err := &jmap.SetError{
		Type: "notFound",
	}
	result := formatMailboxSetError(err)
	assert.Equal(t, "notFound", result)
}

func TestPrintMailboxes_Direct(t *testing.T) {
	mailboxes := []*mailbox.Mailbox{
		{
			ID:           "mbox-1",
			Name:         "Inbox",
			Role:         mailbox.RoleInbox,
			UnreadEmails: 5,
			TotalEmails:  42,
		},
		{
			ID:          "mbox-2",
			Name:        "Projects",
			UnreadEmails: 0,
			TotalEmails:  10,
		},
	}

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := printMailboxes(mailboxes)
	assert.NoError(t, err)
}

func TestPrintMailboxes_JSON_Direct(t *testing.T) {
	mailboxes := []*mailbox.Mailbox{
		{
			ID:   "mbox-1",
			Name: "Inbox",
			Role: mailbox.RoleInbox,
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := printMailboxes(mailboxes)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "mbox-1")
	assert.Contains(t, buf.String(), "Inbox")
}

func TestMailboxDelete(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Mailbox/set" {
				destroy := call.Args["destroy"].([]any)
				assert.Equal(t, "mbox-1", destroy[0])

				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"destroyed": []string{"mbox-1"},
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

	req := &jmap.Request{}
	req.Invoke(&mailbox.Set{
		Account: jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:mail"],
		Destroy: []jmap.ID{"mbox-1"},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*mailbox.SetResponse)
	assert.Contains(t, r.Destroyed, jmap.ID("mbox-1"))
}

func TestRunMailboxList(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Mailbox/get" {
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "mbox-1", "name": "Inbox", "role": "inbox", "unreadEmails": 3, "totalEmails": 20},
						{"id": "mbox-2", "name": "Sent", "role": "sent", "unreadEmails": 0, "totalEmails": 100},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runMailboxList(nil, nil)
	assert.NoError(t, err)
}

func TestRunMailboxCreate(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Mailbox/set" {
				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"new1": map[string]any{"id": "mbox-new", "name": "Projects"},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runMailboxCreate(nil, []string{"Projects"})
	assert.NoError(t, err)
}

func TestRunMailboxCreate_JSON(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Mailbox/set" {
				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"new1": map[string]any{"id": "mbox-new", "name": "Projects"},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := runMailboxCreate(nil, []string{"Projects"})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "mbox-new")
}

func TestRunMailboxCreate_Error(t *testing.T) {
	desc := "name already exists"
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Mailbox/set" {
				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s1",
					"notCreated": map[string]any{
						"new1": map[string]any{"type": "invalidArguments", "description": desc},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	err := runMailboxCreate(nil, []string{"Inbox"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name already exists")
}

func TestRunMailboxRename(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "mbox-1", "name": "OldName"}},
				}, call.CallID))
			case "Mailbox/set":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"mbox-1": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runMailboxRename(nil, []string{"OldName", "NewName"})
	assert.NoError(t, err)
}

func TestRunMailboxRename_JSON(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "mbox-1", "name": "OldName"}},
				}, call.CallID))
			case "Mailbox/set":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"mbox-1": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := runMailboxRename(nil, []string{"OldName", "NewName"})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "mbox-1")
}

func TestRunMailboxRename_NotUpdated(t *testing.T) {
	desc := "forbidden"
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "mbox-1", "name": "OldName"}},
				}, call.CallID))
			case "Mailbox/set":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s1",
					"notUpdated": map[string]any{
						"mbox-1": map[string]any{"type": "forbidden", "description": desc},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	err := runMailboxRename(nil, []string{"OldName", "NewName"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestRunMailboxDelete_Success(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "mbox-1", "name": "Projects"}},
				}, call.CallID))
			case "Mailbox/set":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"destroyed": []string{"mbox-1"},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runMailboxDelete(nil, []string{"Projects"})
	assert.NoError(t, err)
}

func TestRunMailboxDelete_JSON(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "mbox-1", "name": "Projects"}},
				}, call.CallID))
			case "Mailbox/set":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"destroyed": []string{"mbox-1"},
				}, call.CallID))
			}
		}
		return responses
	})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := runMailboxDelete(nil, []string{"Projects"})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "deleted")
}

func TestRunMailboxDelete_NotDestroyed(t *testing.T) {
	desc := "mailbox has child"
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "mbox-1", "name": "Projects"}},
				}, call.CallID))
			case "Mailbox/set":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/set", map[string]any{
					"accountId":    jmaptest.TestAccountID,
					"oldState":     "s1",
					"newState":     "s1",
					"notDestroyed": map[string]any{
						"mbox-1": map[string]any{"type": "mailboxHasChild", "description": desc},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	err := runMailboxDelete(nil, []string{"Projects"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mailbox has child")
}
