package cmd

import (
	"net/http"
	"os"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/thread"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
)

func threadHandler(t *testing.T) jmaptest.APIHandler {
	return func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Email/get":
				ids := call.Args["ids"].([]any)
				if len(ids) == 1 && ids[0] == "email-1" {
					// Getting threadId
					responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
						"accountId": jmaptest.TestAccountID,
						"state":     "s1",
						"list": []map[string]any{
							{"id": "email-1", "threadId": "thread-1"},
						},
					}, call.CallID))
				} else {
					// Getting full emails for thread
					responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
						"accountId": jmaptest.TestAccountID,
						"state":     "s1",
						"list": []map[string]any{
							{
								"id":       "email-1",
								"threadId": "thread-1",
								"subject":  "Original",
								"from":     []map[string]any{{"email": "alice@example.com"}},
								"to":       []map[string]any{{"email": "bob@example.com"}},
								"textBody": []map[string]any{{"partId": "1"}},
								"bodyValues": map[string]any{
									"1": map[string]any{"value": "Hello"},
								},
								"receivedAt": "2026-03-15T10:00:00Z",
							},
							{
								"id":       "email-2",
								"threadId": "thread-1",
								"subject":  "Re: Original",
								"from":     []map[string]any{{"email": "bob@example.com"}},
								"to":       []map[string]any{{"email": "alice@example.com"}},
								"textBody": []map[string]any{{"partId": "1"}},
								"bodyValues": map[string]any{
									"1": map[string]any{"value": "Hi Alice!"},
								},
								"receivedAt": "2026-03-15T11:00:00Z",
							},
						},
					}, call.CallID))
				}
			case "Thread/get":
				ids := call.Args["ids"].([]any)
				assert.Equal(t, "thread-1", ids[0])

				responses = append(responses, jmaptest.MethodResponse("Thread/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{
							"id":       "thread-1",
							"emailIds": []string{"email-1", "email-2"},
						},
					},
				}, call.CallID))
			}
		}
		return responses
	}
}

func TestGetThreadID(t *testing.T) {
	c := newTestClient(t, threadHandler(t))

	threadID, err := getThreadID(c, "email-1")
	require.NoError(t, err)
	assert.Equal(t, jmap.ID("thread-1"), threadID)
}

func TestGetThreadEmailIDs(t *testing.T) {
	c := newTestClient(t, threadHandler(t))

	emailIDs, err := getThreadEmailIDs(c, "thread-1")
	require.NoError(t, err)
	assert.Equal(t, []jmap.ID{"email-1", "email-2"}, emailIDs)
}

func TestGetEmailsByIDs(t *testing.T) {
	c := newTestClient(t, threadHandler(t))

	emails, err := getEmailsByIDs(c, []jmap.ID{"email-1", "email-2"})
	require.NoError(t, err)
	require.Len(t, emails, 2)
	assert.Equal(t, "Original", emails[0].Subject)
	assert.Equal(t, "Re: Original", emails[1].Subject)
}

func TestEmailThread_FullFlow(t *testing.T) {
	server := jmaptest.NewServer(t, threadHandler(t))

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())
	accountID := jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:mail"]

	// Step 1: Get thread ID from email
	req1 := &jmap.Request{}
	req1.Invoke(&email.Get{
		Account:    accountID,
		IDs:        []jmap.ID{"email-1"},
		Properties: []string{"threadId"},
	})
	resp1, err := jmapClient.Do(req1)
	require.NoError(t, err)

	r1 := resp1.Responses[0].Args.(*email.GetResponse)
	require.Len(t, r1.List, 1)
	threadID := r1.List[0].ThreadID
	assert.Equal(t, jmap.ID("thread-1"), threadID)

	// Step 2: Get thread
	req2 := &jmap.Request{}
	req2.Invoke(&thread.Get{
		Account: accountID,
		IDs:     []jmap.ID{threadID},
	})
	resp2, err := jmapClient.Do(req2)
	require.NoError(t, err)

	r2 := resp2.Responses[0].Args.(*thread.GetResponse)
	require.Len(t, r2.List, 1)
	assert.Equal(t, []jmap.ID{"email-1", "email-2"}, r2.List[0].EmailIDs)

	// Step 3: Get all emails
	req3 := &jmap.Request{}
	req3.Invoke(&email.Get{
		Account:             accountID,
		IDs:                 r2.List[0].EmailIDs,
		FetchTextBodyValues: true,
	})
	resp3, err := jmapClient.Do(req3)
	require.NoError(t, err)

	r3 := resp3.Responses[0].Args.(*email.GetResponse)
	require.Len(t, r3.List, 2)
	assert.Equal(t, "Original", r3.List[0].Subject)
	assert.Equal(t, "Re: Original", r3.List[1].Subject)
}

func TestGetThreadID_NotFound(t *testing.T) {
	handler := func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
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
	}

	c := newTestClient(t, handler)

	_, err := getThreadID(c, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email not found")
}

func TestAttachmentDownload(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/get" {
				responses = append(responses, jmaptest.MethodResponse("Email/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{
							"id": "email-1",
							"attachments": []map[string]any{
								{
									"partId":      "att1",
									"blobId":      "blob-att1",
									"name":        "document.pdf",
									"type":        "application/pdf",
									"size":        12345,
									"disposition":  "attachment",
								},
							},
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

	c := &client.Client{JMAP: jmapClient}

	// Download attachment
	tmpDir := t.TempDir()
	attachmentOutput = tmpDir

	err := downloadAttachment(c, &email.BodyPart{
		BlobID: "blob-att1",
		Name:   "document.pdf",
		Type:   "application/pdf",
		Size:   12345,
	})
	require.NoError(t, err)

	// Verify file was created (content is "test-blob-content" from our test server)
	content, err := os.ReadFile(tmpDir + "/document.pdf")
	require.NoError(t, err)
	assert.Equal(t, "test-blob-content", string(content))
}
