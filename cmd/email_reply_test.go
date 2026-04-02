package cmd

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
)

func TestFetchEmailForReply(t *testing.T) {
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
							"id":         "email-1",
							"subject":    "Original Subject",
							"messageId":  []string{"<msg-1@example.com>"},
							"references": []string{"<prev@example.com>"},
							"from":       []map[string]any{{"name": "Alice", "email": "alice@example.com"}},
							"to":         []map[string]any{{"email": "bob@example.com"}},
							"textBody":   []map[string]any{{"partId": "1"}},
							"bodyValues": map[string]any{
								"1": map[string]any{"value": "Hello Bob"},
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

	original, err := fetchEmailForReply(c, "email-1")
	require.NoError(t, err)
	assert.Equal(t, "Original Subject", original.Subject)
	assert.Equal(t, "alice@example.com", original.From[0].Email)
	assert.Equal(t, []string{"<msg-1@example.com>"}, original.MessageID)
}

func TestExtractTextBody(t *testing.T) {
	e := &email.Email{
		TextBody: []*email.BodyPart{
			{PartID: "1"},
			{PartID: "2"},
		},
		BodyValues: map[string]*email.BodyValue{
			"1": {Value: "Hello"},
			"2": {Value: "World"},
		},
	}

	result := extractTextBody(e)
	assert.Equal(t, "Hello\nWorld", result)
}

func TestReply_SubjectPrefix(t *testing.T) {
	// Verify Re: prefix logic
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello", "Re: Hello"},
		{"Re: Hello", "Re: Hello"},
		{"RE: Hello", "RE: Hello"},
		{"re: Hello", "re: Hello"},
	}

	for _, tt := range tests {
		subject := tt.input
		if !hasReplyPrefix(subject) {
			subject = "Re: " + subject
		}
		assert.Equal(t, tt.expected, subject)
	}
}

func hasReplyPrefix(subject string) bool {
	lower := stringToLower(subject)
	return len(lower) >= 3 && lower[:3] == "re:"
}

func stringToLower(s string) string {
	result := make([]byte, len(s))
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func TestForward_SubjectPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello", "Fwd: Hello"},
		{"Fwd: Hello", "Fwd: Hello"},
		{"FWD: Hello", "FWD: Hello"},
	}

	for _, tt := range tests {
		subject := tt.input
		lower := stringToLower(subject)
		if !(len(lower) >= 4 && lower[:4] == "fwd:") {
			subject = "Fwd: " + subject
		}
		assert.Equal(t, tt.expected, subject)
	}
}

func TestExtractTextBody_Empty(t *testing.T) {
	e := &email.Email{
		TextBody:   []*email.BodyPart{},
		BodyValues: map[string]*email.BodyValue{},
	}
	result := extractTextBody(e)
	assert.Equal(t, "", result)
}

func TestExtractTextBody_MissingBodyValue(t *testing.T) {
	e := &email.Email{
		TextBody:   []*email.BodyPart{{PartID: "1"}, {PartID: "2"}},
		BodyValues: map[string]*email.BodyValue{
			"1": {Value: "Only first part"},
		},
	}
	result := extractTextBody(e)
	assert.Equal(t, "Only first part", result)
}

func TestFetchEmailForReply_NotFound(t *testing.T) {
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

	c := &client.Client{JMAP: jmapClient}
	_, err := fetchEmailForReply(c, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email not found")
}

func TestSendEmail_JSONOutput(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Email/set":
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"draft": map[string]any{"id": "email-sent"},
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

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	c := &client.Client{JMAP: jmapClient}

	// Capture JSON output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	replyEmail := &email.Email{
		MailboxIDs: map[jmap.ID]bool{"mbox-drafts": true},
		To:         []*mail.Address{{Email: "alice@example.com"}},
		Subject:    "Re: Hello",
		Keywords:   map[string]bool{"$draft": true},
		BodyValues: map[string]*email.BodyValue{
			"body": {Value: "Thanks!"},
		},
		TextBody: []*email.BodyPart{
			{PartID: "body", Type: "text/plain"},
		},
	}

	err := sendEmail(c, c.MailAccountID(), "ident-1", "mbox-drafts", replyEmail)
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "emailId")
	assert.Contains(t, buf.String(), "submissionId")
}

func TestSendEmail_RequestConstruction(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			switch call.Name {
			case "Email/set":
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"draft": map[string]any{
							"id": "email-reply",
						},
					},
				}, call.CallID))
			case "EmailSubmission/set":
				responses = append(responses, jmaptest.MethodResponse("EmailSubmission/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"send": map[string]any{
							"id": "sub-reply",
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

	replyEmail := &email.Email{
		MailboxIDs: map[jmap.ID]bool{"mbox-drafts": true},
		To:         []*mail.Address{{Email: "alice@example.com"}},
		Subject:    "Re: Hello",
		InReplyTo:  []string{"<msg-1@example.com>"},
		Keywords:   map[string]bool{"$draft": true},
		BodyValues: map[string]*email.BodyValue{
			"body": {Value: "Thanks!"},
		},
		TextBody: []*email.BodyPart{
			{PartID: "body", Type: "text/plain"},
		},
	}

	err := sendEmail(c, c.MailAccountID(), "ident-1", "mbox-drafts", replyEmail)
	require.NoError(t, err)
}

func replyHandler(t *testing.T) jmaptest.APIHandler {
	return func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
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
							"id":        "email-orig",
							"subject":   "Original Subject",
							"messageId": []string{"<msg-1@example.com>"},
							"from":      []map[string]any{{"name": "Alice", "email": "alice@example.com"}},
							"to":        []map[string]any{{"email": "bob@example.com"}},
							"cc":        []map[string]any{{"email": "carol@example.com"}},
							"textBody":  []map[string]any{{"partId": "1"}},
							"bodyValues": map[string]any{
								"1": map[string]any{"value": "Original body"},
							},
							"receivedAt": "2026-03-15T10:00:00Z",
						},
					},
				}, call.CallID))
			case "Identity/get":
				responses = append(responses, jmaptest.MethodResponse("Identity/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "ident-1", "name": "Test", "email": "test@fm.com"}},
				}, call.CallID))
			case "Mailbox/get":
				responses = append(responses, jmaptest.MethodResponse("Mailbox/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{{"id": "mbox-drafts", "name": "Drafts", "role": "drafts"}},
				}, call.CallID))
			case "Email/set":
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState": "s1", "newState": "s2",
					"created": map[string]any{"draft": map[string]any{"id": "email-reply"}},
				}, call.CallID))
			case "EmailSubmission/set":
				responses = append(responses, jmaptest.MethodResponse("EmailSubmission/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState": "s1", "newState": "s2",
					"created": map[string]any{"send": map[string]any{"id": "sub-1"}},
				}, call.CallID))
			}
		}
		return responses
	}
}

func TestRunEmailReply(t *testing.T) {
	withTestClient(t, replyHandler(t))

	oldBody := replyBody
	oldHTML := replyHTML
	oldAll := replyAll
	oldIdent := sendIdentity
	oldJSON := jsonOutput
	oldCfg := cfg

	replyBody = "Thanks for the email!"
	replyHTML = false
	replyAll = false
	sendIdentity = ""
	jsonOutput = false
	cfg = nil
	defer func() {
		replyBody = oldBody
		replyHTML = oldHTML
		replyAll = oldAll
		sendIdentity = oldIdent
		jsonOutput = oldJSON
		cfg = oldCfg
	}()

	err := runEmailReply(nil, []string{"email-orig"})
	assert.NoError(t, err)
}

func TestRunEmailReply_All(t *testing.T) {
	withTestClient(t, replyHandler(t))

	oldBody := replyBody
	oldHTML := replyHTML
	oldAll := replyAll
	oldIdent := sendIdentity
	oldJSON := jsonOutput
	oldCfg := cfg

	replyBody = "Reply all!"
	replyHTML = true
	replyAll = true
	sendIdentity = ""
	jsonOutput = false
	cfg = nil
	defer func() {
		replyBody = oldBody
		replyHTML = oldHTML
		replyAll = oldAll
		sendIdentity = oldIdent
		jsonOutput = oldJSON
		cfg = oldCfg
	}()

	err := runEmailReply(nil, []string{"email-orig"})
	assert.NoError(t, err)
}

func TestRunEmailForward(t *testing.T) {
	withTestClient(t, replyHandler(t))

	oldBody := replyBody
	oldTo := forwardTo
	oldIdent := sendIdentity
	oldJSON := jsonOutput
	oldCfg := cfg

	replyBody = "FYI see below"
	forwardTo = []string{"dave@example.com"}
	sendIdentity = ""
	jsonOutput = false
	cfg = nil
	defer func() {
		replyBody = oldBody
		forwardTo = oldTo
		sendIdentity = oldIdent
		jsonOutput = oldJSON
		cfg = oldCfg
	}()

	err := runEmailForward(nil, []string{"email-orig"})
	assert.NoError(t, err)
}
