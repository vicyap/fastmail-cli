package cmd

import (
	"encoding/json"
	"net/http"
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
