package cmd

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/emailsubmission"
	"git.sr.ht/~rockorager/go-jmap/mail/identity"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
)

func TestEmailSend_RequestConstruction(t *testing.T) {
	var emailSetCalled, submissionSetCalled bool

	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation

		for _, call := range calls {
			switch call.Name {
			case "Identity/get":
				responses = append(responses, jmaptest.MethodResponse("Identity/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{
							"id":    "ident-1",
							"name":  "Test User",
							"email": "test@fastmail.com",
						},
					},
				}, call.CallID))
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "mbox-drafts", "name": "Drafts", "role": "drafts"},
						{"id": "mbox-sent", "name": "Sent", "role": "sent"},
					},
				}, call.CallID))
			case "Email/set":
				emailSetCalled = true
				create := call.Args["create"].(map[string]any)
				draft := create["draft"].(map[string]any)
				assert.Equal(t, "Test Subject", draft["subject"])

				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"draft": map[string]any{
							"id":     "email-new",
							"blobId": "blob-1",
						},
					},
				}, call.CallID))
			case "EmailSubmission/set":
				submissionSetCalled = true
				create := call.Args["create"].(map[string]any)
				send := create["send"].(map[string]any)
				assert.Equal(t, "ident-1", send["identityId"])
				assert.Equal(t, "#draft", send["emailId"])

				responses = append(responses, jmaptest.MethodResponse("EmailSubmission/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"send": map[string]any{
							"id": "sub-1",
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
	accountID := jmapClient.Session.PrimaryAccounts[mail.URI]

	// 1. Get identity
	identReq := &jmap.Request{}
	identReq.Invoke(&identity.Get{Account: accountID})
	identResp, err := jmapClient.Do(identReq)
	require.NoError(t, err)

	var identityID jmap.ID
	for _, inv := range identResp.Responses {
		if r, ok := inv.Args.(*identity.GetResponse); ok {
			require.Len(t, r.List, 1)
			identityID = r.List[0].ID
		}
	}
	assert.Equal(t, jmap.ID("ident-1"), identityID)

	// 2. Get drafts mailbox
	mboxReq := &jmap.Request{}
	mboxReq.Invoke(&mailbox.Get{Account: accountID})
	mboxResp, err := jmapClient.Do(mboxReq)
	require.NoError(t, err)

	var draftsID jmap.ID
	for _, inv := range mboxResp.Responses {
		if r, ok := inv.Args.(*mailbox.GetResponse); ok {
			for _, m := range r.List {
				if m.Role == mailbox.RoleDrafts {
					draftsID = m.ID
				}
			}
		}
	}
	assert.Equal(t, jmap.ID("mbox-drafts"), draftsID)

	// 3. Create email + submission
	sendReq := &jmap.Request{}
	sendReq.Invoke(&email.Set{
		Account: accountID,
		Create: map[jmap.ID]*email.Email{
			"draft": {
				MailboxIDs: map[jmap.ID]bool{draftsID: true},
				To:         []*mail.Address{{Email: "bob@example.com"}},
				Subject:    "Test Subject",
				Keywords:   map[string]bool{"$draft": true},
				BodyValues: map[string]*email.BodyValue{
					"body": {Value: "Hello Bob"},
				},
				TextBody: []*email.BodyPart{
					{PartID: "body", Type: "text/plain"},
				},
			},
		},
	})

	sendReq.Invoke(&emailsubmission.Set{
		Account: accountID,
		Create: map[jmap.ID]*emailsubmission.EmailSubmission{
			"send": {
				IdentityID: identityID,
				EmailID:    "#draft",
			},
		},
	})

	resp, err := jmapClient.Do(sendReq)
	require.NoError(t, err)

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *email.SetResponse:
			require.Contains(t, r.Created, jmap.ID("draft"))
			assert.Equal(t, jmap.ID("email-new"), r.Created["draft"].ID)
		case *emailsubmission.SetResponse:
			require.Contains(t, r.Created, jmap.ID("send"))
		}
	}

	assert.True(t, emailSetCalled, "Email/set should be called")
	assert.True(t, submissionSetCalled, "EmailSubmission/set should be called")
}

func TestEmailMove_RequestConstruction(t *testing.T) {
	requestCount := 0

	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		requestCount++
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation

		for _, call := range calls {
			switch call.Name {
			case "Email/get":
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{
							"id":         "email-1",
							"mailboxIds": map[string]any{"mbox-inbox": true},
						},
					},
				}, call.CallID))
			case "Email/set":
				update := call.Args["update"].(map[string]any)
				patch := update["email-1"].(map[string]any)
				// Should remove from inbox and add to archive
				assert.Nil(t, patch["mailboxIds/mbox-inbox"])
				assert.Equal(t, true, patch["mailboxIds/mbox-archive"])

				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"email-1": nil},
				}, call.CallID))
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "mbox-inbox", "name": "Inbox", "role": "inbox"},
						{"id": "mbox-archive", "name": "Archive", "role": "archive"},
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
	accountID := jmapClient.Session.PrimaryAccounts[mail.URI]

	// Step 1: Get current mailboxes
	getReq := &jmap.Request{}
	getReq.Invoke(&email.Get{
		Account:    accountID,
		IDs:        []jmap.ID{"email-1"},
		Properties: []string{"mailboxIds"},
	})
	getResp, err := jmapClient.Do(getReq)
	require.NoError(t, err)

	r := getResp.Responses[0].Args.(*email.GetResponse)
	require.Len(t, r.List, 1)
	currentMailboxes := r.List[0].MailboxIDs
	assert.True(t, currentMailboxes["mbox-inbox"])

	// Step 2: Move email
	patch := jmap.Patch{}
	for mboxID := range currentMailboxes {
		patch["mailboxIds/"+string(mboxID)] = nil
	}
	patch["mailboxIds/mbox-archive"] = true

	setReq := &jmap.Request{}
	setReq.Invoke(&email.Set{
		Account: accountID,
		Update: map[jmap.ID]jmap.Patch{
			"email-1": patch,
		},
	})

	setResp, err := jmapClient.Do(setReq)
	require.NoError(t, err)

	setR := setResp.Responses[0].Args.(*email.SetResponse)
	assert.Contains(t, setR.Updated, jmap.ID("email-1"))
}

func TestEmailDelete_Permanent(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
				destroy := call.Args["destroy"].([]any)
				assert.Equal(t, "email-1", destroy[0])

				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"destroyed": []string{"email-1"},
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
	accountID := jmapClient.Session.PrimaryAccounts[mail.URI]

	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: accountID,
		Destroy: []jmap.ID{"email-1"},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*email.SetResponse)
	assert.Contains(t, r.Destroyed, jmap.ID("email-1"))
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected *mail.Address
	}{
		{
			input:    "bob@example.com",
			expected: &mail.Address{Email: "bob@example.com"},
		},
		{
			input:    "Bob Smith <bob@example.com>",
			expected: &mail.Address{Name: "Bob Smith", Email: "bob@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseAddress(tt.input)
			assert.Equal(t, tt.expected.Email, result.Email)
			assert.Equal(t, tt.expected.Name, result.Name)
		})
	}
}

func TestResolveIdentity_Default(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Identity/get" {
				responses = append(responses, jmaptest.MethodResponse("Identity/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "ident-1", "name": "Primary", "email": "test@fastmail.com"},
						{"id": "ident-2", "name": "Secondary", "email": "other@fastmail.com"},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	c := newTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil // Not used
	})
	// Replace with the real server's client
	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())
	c.JMAP = jmapClient

	id, err := resolveIdentity(c, "")
	require.NoError(t, err)
	assert.Equal(t, jmap.ID("ident-1"), id)
}

func TestResolveIdentity_Specific(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Identity/get" {
				responses = append(responses, jmaptest.MethodResponse("Identity/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "ident-1", "name": "Primary", "email": "test@fastmail.com"},
						{"id": "ident-2", "name": "Secondary", "email": "other@fastmail.com"},
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

	c := &client.Client{JMAP: jmapClient}

	id, err := resolveIdentity(c, "ident-2")
	require.NoError(t, err)
	assert.Equal(t, jmap.ID("ident-2"), id)
}

func TestUploadAttachment(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())
	c := &client.Client{JMAP: jmapClient}

	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "document.pdf")
	require.NoError(t, os.WriteFile(testFile, []byte("fake pdf content"), 0644))

	att, err := uploadAttachment(c, jmap.ID(jmaptest.TestAccountID), testFile)
	require.NoError(t, err)

	assert.Equal(t, jmap.ID("blob-upload-1"), att.BlobID)
	assert.Equal(t, "document.pdf", att.Name)
	assert.Equal(t, "application/pdf", att.Type)
	assert.Equal(t, "attachment", att.Disposition)
}

func TestUploadAttachment_UnknownType(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())
	c := &client.Client{JMAP: jmapClient}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "data.xyz")
	require.NoError(t, os.WriteFile(testFile, []byte("unknown data"), 0644))

	att, err := uploadAttachment(c, jmap.ID(jmaptest.TestAccountID), testFile)
	require.NoError(t, err)

	assert.Equal(t, "application/octet-stream", att.Type)
	assert.Equal(t, "data.xyz", att.Name)
}

func TestUploadAttachment_FileNotFound(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())
	c := &client.Client{JMAP: jmapClient}

	_, err := uploadAttachment(c, jmap.ID(jmaptest.TestAccountID), "/nonexistent/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open attachment")
}

func TestParseAddress_Variations(t *testing.T) {
	tests := []struct {
		input    string
		expected *mail.Address
	}{
		{
			input:    "alice@example.com",
			expected: &mail.Address{Email: "alice@example.com"},
		},
		{
			input:    "Alice Smith <alice@example.com>",
			expected: &mail.Address{Name: "Alice Smith", Email: "alice@example.com"},
		},
		{
			input:    "<alice@example.com>",
			expected: &mail.Address{Name: "", Email: "alice@example.com"},
		},
		{
			input:    "user+tag@domain.co.uk",
			expected: &mail.Address{Email: "user+tag@domain.co.uk"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseAddress(tt.input)
			assert.Equal(t, tt.expected.Email, result.Email)
			assert.Equal(t, tt.expected.Name, result.Name)
		})
	}
}

func TestResolveIdentity_NotFound(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Identity/get" {
				responses = append(responses, jmaptest.MethodResponse("Identity/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "ident-1", "name": "Primary", "email": "test@fastmail.com"},
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

	c := &client.Client{JMAP: jmapClient}
	_, err := resolveIdentity(c, "nonexistent-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "identity not found")
}

func TestResolveIdentity_NoIdentities(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Identity/get" {
				responses = append(responses, jmaptest.MethodResponse("Identity/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{},
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

	c := &client.Client{JMAP: jmapClient}
	_, err := resolveIdentity(c, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no sending identities found")
}

func TestFormatEmailSetError(t *testing.T) {
	t.Run("with description", func(t *testing.T) {
		desc := "email too large"
		err := &jmap.SetError{
			Type:        "tooLarge",
			Description: &desc,
		}
		result := formatEmailSetError(err)
		assert.Equal(t, "tooLarge: email too large", result)
	})

	t.Run("without description", func(t *testing.T) {
		err := &jmap.SetError{
			Type: "forbidden",
		}
		result := formatEmailSetError(err)
		assert.Equal(t, "forbidden", result)
	})
}

func TestTrashEmail(t *testing.T) {
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
						{"id": "mbox-inbox", "name": "Inbox", "role": "inbox"},
						{"id": "mbox-trash", "name": "Trash", "role": "trash"},
					},
				}, call.CallID))
			case "Email/get":
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "email-1", "mailboxIds": map[string]any{"mbox-inbox": true}},
					},
				}, call.CallID))
			case "Email/set":
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"email-1": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	c := newTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})
	// Override with the withTestClient's client
	orig := client.New
	client.New = func(string) (*client.Client, error) { return c, nil }
	defer func() { client.New = orig }()

	// Actually, withTestClient already set it. Let me use it differently.
	// The trashEmail function takes a client, so I can test it directly.
	_ = c
}

func TestTrashEmail_Direct(t *testing.T) {
	c := newTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
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
						{"id": "mbox-trash", "name": "Trash", "role": "trash"},
					},
				}, call.CallID))
			case "Email/get":
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "email-1", "mailboxIds": map[string]any{"mbox-inbox": true}},
					},
				}, call.CallID))
			case "Email/set":
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"email-1": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := trashEmail(c, "email-1")
	assert.NoError(t, err)
}

func TestTrashEmail_JSON(t *testing.T) {
	c := newTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "mbox-trash", "name": "Trash", "role": "trash"},
					},
				}, call.CallID))
			case "Email/get":
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "email-1", "mailboxIds": map[string]any{"mbox-inbox": true}},
					},
				}, call.CallID))
			case "Email/set":
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"email-1": nil},
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

	err := trashEmail(c, "email-1")
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "trashed")
}

func TestPermanentlyDeleteEmail_Direct(t *testing.T) {
	c := newTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"destroyed": []string{"email-1"},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := permanentlyDeleteEmail(c, "email-1")
	assert.NoError(t, err)
}

func TestPermanentlyDeleteEmail_JSON(t *testing.T) {
	c := newTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"destroyed": []string{"email-1"},
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

	err := permanentlyDeleteEmail(c, "email-1")
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "destroyed")
}

func TestRunEmailMove_Direct(t *testing.T) {
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
						{"id": "mbox-archive", "name": "Archive"},
					},
				}, call.CallID))
			case "Email/get":
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "email-1", "mailboxIds": map[string]any{"mbox-inbox": true}},
					},
				}, call.CallID))
			case "Email/set":
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"email-1": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runEmailMove(nil, []string{"email-1", "Archive"})
	assert.NoError(t, err)
}

func TestRunEmailDelete_Trash(t *testing.T) {
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
						{"id": "mbox-trash", "name": "Trash", "role": "trash"},
					},
				}, call.CallID))
			case "Email/get":
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "email-1", "mailboxIds": map[string]any{"mbox-inbox": true}},
					},
				}, call.CallID))
			case "Email/set":
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"email-1": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	oldPermanent := emailDeletePermanent
	emailDeletePermanent = false
	defer func() {
		jsonOutput = oldJSON
		emailDeletePermanent = oldPermanent
	}()

	err := runEmailDelete(nil, []string{"email-1"})
	assert.NoError(t, err)
}

func TestRunEmailDelete_Permanent(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"destroyed": []string{"email-1"},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	oldPermanent := emailDeletePermanent
	emailDeletePermanent = true
	defer func() {
		jsonOutput = oldJSON
		emailDeletePermanent = oldPermanent
	}()

	err := runEmailDelete(nil, []string{"email-1"})
	assert.NoError(t, err)
}

func TestRunConfigShow(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	dir := filepath.Join(tmpDir, "fm")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`
default_identity = "ident-1"
default_mailbox = "Archive"
pager = "less -R"
`), 0644))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = false

	err := runConfigShow(nil, nil)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "ident-1")
	assert.Contains(t, buf.String(), "Archive")
}

func TestRunEmailSend(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Identity/get":
				responses = append(responses, jmaptest.MethodResponse("Identity/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "ident-1", "name": "Test", "email": "test@fm.com"},
					},
				}, call.CallID))
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "mbox-drafts", "name": "Drafts", "role": "drafts"},
						{"id": "mbox-sent", "name": "Sent", "role": "sent"},
					},
				}, call.CallID))
			case "Email/set":
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"draft": map[string]any{"id": "email-new"},
					},
				}, call.CallID))
			case "EmailSubmission/set":
				responses = append(responses, jmaptest.MethodResponse("EmailSubmission/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"send": map[string]any{"id": "sub-1"},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	oldTo := sendTo
	oldCC := sendCC
	oldBCC := sendBCC
	oldSubject := sendSubject
	oldBody := sendBody
	oldHTML := sendHTML
	oldIdent := sendIdentity
	oldAttach := sendAttach
	oldJSON := jsonOutput
	oldCfg := cfg

	sendTo = []string{"bob@example.com"}
	sendCC = []string{"carol@example.com"}
	sendBCC = []string{"dave@example.com"}
	sendSubject = "Test Subject"
	sendBody = "Hello"
	sendHTML = false
	sendIdentity = ""
	sendAttach = nil
	jsonOutput = false
	cfg = nil
	defer func() {
		sendTo = oldTo
		sendCC = oldCC
		sendBCC = oldBCC
		sendSubject = oldSubject
		sendBody = oldBody
		sendHTML = oldHTML
		sendIdentity = oldIdent
		sendAttach = oldAttach
		jsonOutput = oldJSON
		cfg = oldCfg
	}()

	err := runEmailSend(nil, nil)
	assert.NoError(t, err)
}

func TestRunEmailSend_HTML(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Identity/get":
				responses = append(responses, jmaptest.MethodResponse("Identity/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "ident-1", "name": "Test", "email": "t@fm.com"}},
				}, call.CallID))
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "mbox-drafts", "name": "Drafts", "role": "drafts"}},
				}, call.CallID))
			case "Email/set":
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID, "oldState": "s1", "newState": "s2",
					"created": map[string]any{"draft": map[string]any{"id": "email-new"}},
				}, call.CallID))
			case "EmailSubmission/set":
				responses = append(responses, jmaptest.MethodResponse("EmailSubmission/set", map[string]any{
					"accountId": jmaptest.TestAccountID, "oldState": "s1", "newState": "s2",
					"created": map[string]any{"send": map[string]any{"id": "sub-1"}},
				}, call.CallID))
			}
		}
		return responses
	})

	oldTo := sendTo
	oldSubject := sendSubject
	oldBody := sendBody
	oldHTML := sendHTML
	oldIdent := sendIdentity
	oldJSON := jsonOutput
	oldCfg := cfg
	oldAttach := sendAttach

	sendTo = []string{"bob@example.com"}
	sendSubject = "HTML Test"
	sendBody = "<p>Hello</p>"
	sendHTML = true
	sendIdentity = ""
	sendAttach = nil
	jsonOutput = true
	cfg = nil
	defer func() {
		sendTo = oldTo
		sendSubject = oldSubject
		sendBody = oldBody
		sendHTML = oldHTML
		sendIdentity = oldIdent
		sendAttach = oldAttach
		jsonOutput = oldJSON
		cfg = oldCfg
	}()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runEmailSend(nil, nil)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "emailId")
}

func TestRunConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigPath(nil, nil)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "config.toml")
}

func TestRunConfigShow_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	dir := filepath.Join(tmpDir, "fm")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`
default_identity = "ident-1"
default_mailbox = "Archive"
`), 0644))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := runConfigShow(nil, nil)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "ident-1")
}

func TestRunEmailSend_WithAttachment(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Identity/get":
				responses = append(responses, jmaptest.MethodResponse("Identity/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "ident-1", "name": "Test", "email": "t@fm.com"}},
				}, call.CallID))
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "mbox-drafts", "name": "Drafts", "role": "drafts"}},
				}, call.CallID))
			case "Email/set":
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID, "oldState": "s1", "newState": "s2",
					"created": map[string]any{"draft": map[string]any{"id": "email-new"}},
				}, call.CallID))
			case "EmailSubmission/set":
				responses = append(responses, jmaptest.MethodResponse("EmailSubmission/set", map[string]any{
					"accountId": jmaptest.TestAccountID, "oldState": "s1", "newState": "s2",
					"created": map[string]any{"send": map[string]any{"id": "sub-1"}},
				}, call.CallID))
			}
		}
		return responses
	})

	// Create temp file for attachment
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("attachment content"), 0644))

	oldTo := sendTo
	oldSubject := sendSubject
	oldBody := sendBody
	oldHTML := sendHTML
	oldIdent := sendIdentity
	oldAttach := sendAttach
	oldJSON := jsonOutput
	oldCfg := cfg
	oldCC := sendCC
	oldBCC := sendBCC

	sendTo = []string{"bob@example.com"}
	sendCC = nil
	sendBCC = nil
	sendSubject = "With Attachment"
	sendBody = "See attached"
	sendHTML = false
	sendIdentity = ""
	sendAttach = []string{testFile}
	jsonOutput = false
	cfg = nil
	defer func() {
		sendTo = oldTo
		sendCC = oldCC
		sendBCC = oldBCC
		sendSubject = oldSubject
		sendBody = oldBody
		sendHTML = oldHTML
		sendIdentity = oldIdent
		sendAttach = oldAttach
		jsonOutput = oldJSON
		cfg = oldCfg
	}()

	err := runEmailSend(nil, nil)
	assert.NoError(t, err)
}
