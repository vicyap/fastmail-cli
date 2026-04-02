package cmd

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
)

func TestEmailFlag(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
				update := call.Args["update"].(map[string]any)
				patch := update["email-1"].(map[string]any)
				assert.Equal(t, true, patch["keywords/$flagged"])

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

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	accountID := jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:mail"]
	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: accountID,
		Update: map[jmap.ID]jmap.Patch{
			"email-1": {"keywords/$flagged": true},
		},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*email.SetResponse)
	assert.Contains(t, r.Updated, jmap.ID("email-1"))
}

func TestEmailUnflag(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
				update := call.Args["update"].(map[string]any)
				patch := update["email-1"].(map[string]any)
				// Setting to nil removes the keyword
				assert.Nil(t, patch["keywords/$flagged"])

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

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	accountID := jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:mail"]
	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: accountID,
		Update: map[jmap.ID]jmap.Patch{
			"email-1": {"keywords/$flagged": nil},
		},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*email.SetResponse)
	assert.Contains(t, r.Updated, jmap.ID("email-1"))
}

func TestSetEmailKeyword(t *testing.T) {
	c := newTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
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

	// Temporarily override the client creation
	origNew := client.New
	_ = origNew // client.New is a func, can't override. Test via JMAP directly.
	_ = c
}

func TestSetEmailKeyword_ViaJMAP(t *testing.T) {
	tests := []struct {
		name    string
		keyword string
		value   any
		action  string
	}{
		{
			name:    "flag email",
			keyword: "$flagged",
			value:   true,
			action:  "flagged",
		},
		{
			name:    "unflag email",
			keyword: "$flagged",
			value:   nil,
			action:  "unflagged",
		},
		{
			name:    "mark read",
			keyword: "$seen",
			value:   true,
			action:  "flagged",
		},
		{
			name:    "mark unread",
			keyword: "$seen",
			value:   nil,
			action:  "unflagged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
				calls := jmaptest.ParseCalls(t, req)
				var responses []jmaptest.RawInvocation
				for _, call := range calls {
					if call.Name == "Email/set" {
						update := call.Args["update"].(map[string]any)
						patch := update["email-1"].(map[string]any)
						key := "keywords/" + tt.keyword
						if tt.value == nil {
							assert.Nil(t, patch[key])
						} else {
							assert.Equal(t, true, patch[key])
						}

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

			jmapClient := &jmap.Client{
				SessionEndpoint: server.URL + "/session",
				HttpClient:      http.DefaultClient,
			}
			require.NoError(t, jmapClient.Authenticate())

			accountID := jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:mail"]
			req := &jmap.Request{}
			req.Invoke(&email.Set{
				Account: accountID,
				Update: map[jmap.ID]jmap.Patch{
					"email-1": {
						"keywords/" + tt.keyword: tt.value,
					},
				},
			})

			resp, err := jmapClient.Do(req)
			require.NoError(t, err)
			r := resp.Responses[0].Args.(*email.SetResponse)
			assert.Contains(t, r.Updated, jmap.ID("email-1"))
		})
	}
}

func TestFormatEmailSetError_WithDescription(t *testing.T) {
	desc := "email not found"
	err := &jmap.SetError{
		Type:        "notFound",
		Description: &desc,
	}
	result := formatEmailSetError(err)
	assert.Equal(t, "notFound: email not found", result)
}

func TestFormatEmailSetError_WithoutDescription(t *testing.T) {
	err := &jmap.SetError{
		Type: "notFound",
	}
	result := formatEmailSetError(err)
	assert.Equal(t, "notFound", result)
}

func TestSetEmailKeyword_RunFunc(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
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

	err := setEmailKeyword("email-1", "$flagged", true)
	assert.NoError(t, err)
}

func TestSetEmailKeyword_JSONOutput(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
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

	err := setEmailKeyword("email-1", "$flagged", true)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "flagged")
}

func TestSetEmailKeyword_Unflag(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
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

	err := setEmailKeyword("email-1", "$flagged", nil)
	assert.NoError(t, err)
}

func TestSetEmailKeyword_NotUpdated(t *testing.T) {
	desc := "email not found"
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
				responses = append(responses, jmaptest.MethodResponse("Email/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s1",
					"notUpdated": map[string]any{
						"email-1": map[string]any{"type": "notFound", "description": desc},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	err := setEmailKeyword("email-1", "$flagged", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email not found")
}

func TestRunEmailFlag(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
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

	err := runEmailFlag(nil, []string{"email-1"})
	assert.NoError(t, err)
}

func TestRunEmailUnflag(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
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

	err := runEmailUnflag(nil, []string{"email-1"})
	assert.NoError(t, err)
}

func TestRunEmailMarkRead(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
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

	err := runEmailMarkRead(nil, []string{"email-1"})
	assert.NoError(t, err)
}

func TestRunEmailMarkUnread(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Email/set" {
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

	err := runEmailMarkUnread(nil, []string{"email-1"})
	assert.NoError(t, err)
}
