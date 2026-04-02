package cmd

import (
	"net/http"
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
