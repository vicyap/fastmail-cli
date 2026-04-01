package cmd

import (
	"net/http"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/vacationresponse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
)

func TestVacationGet(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 1)
		assert.Equal(t, "VacationResponse/get", calls[0].Name)

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("VacationResponse/get", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"state":     "s1",
				"list": []map[string]any{
					{
						"id":        "singleton",
						"isEnabled": true,
						"subject":   "Out of office",
						"textBody":  "I'm on vacation until next week.",
						"fromDate":  "2026-04-01T00:00:00Z",
						"toDate":    "2026-04-15T00:00:00Z",
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

	accountID := jmapClient.Session.PrimaryAccounts[vacationresponse.URI]

	req := &jmap.Request{}
	req.Invoke(&vacationresponse.Get{
		Account: accountID,
		IDs:     []jmap.ID{"singleton"},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*vacationresponse.GetResponse)
	require.Len(t, r.List, 1)
	assert.True(t, r.List[0].IsEnabled)
	assert.Equal(t, "Out of office", *r.List[0].Subject)
	assert.Equal(t, "I'm on vacation until next week.", *r.List[0].TextBody)
}

func TestVacationSet_Enable(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 1)
		assert.Equal(t, "VacationResponse/set", calls[0].Name)

		update := calls[0].Args["update"].(map[string]any)
		singleton := update["singleton"].(map[string]any)
		assert.Equal(t, true, singleton["isEnabled"])
		assert.Equal(t, "On holiday", singleton["subject"])

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("VacationResponse/set", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"oldState":  "s1",
				"newState":  "s2",
				"updated": map[string]any{
					"singleton": nil,
				},
			}, calls[0].CallID),
		}
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	accountID := jmapClient.Session.PrimaryAccounts[vacationresponse.URI]

	req := &jmap.Request{}
	req.Invoke(&vacationresponse.Set{
		Account: accountID,
		Update: map[jmap.ID]jmap.Patch{
			"singleton": {
				"isEnabled": true,
				"subject":   "On holiday",
			},
		},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*vacationresponse.SetResponse)
	assert.Contains(t, r.Updated, jmap.ID("singleton"))
}

func TestVacationSet_Disable(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 1)
		assert.Equal(t, "VacationResponse/set", calls[0].Name)

		update := calls[0].Args["update"].(map[string]any)
		singleton := update["singleton"].(map[string]any)
		assert.Equal(t, false, singleton["isEnabled"])

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("VacationResponse/set", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"oldState":  "s1",
				"newState":  "s2",
				"updated": map[string]any{
					"singleton": nil,
				},
			}, calls[0].CallID),
		}
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	accountID := jmapClient.Session.PrimaryAccounts[vacationresponse.URI]

	req := &jmap.Request{}
	req.Invoke(&vacationresponse.Set{
		Account: accountID,
		Update: map[jmap.ID]jmap.Patch{
			"singleton": {
				"isEnabled": false,
			},
		},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*vacationresponse.SetResponse)
	assert.Contains(t, r.Updated, jmap.ID("singleton"))
}
