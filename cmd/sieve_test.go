package cmd

import (
	"net/http"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/client"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
	"github.com/vicyap/fastmail-cli/internal/sieve"
)

func TestSieveList(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 1)
		assert.Equal(t, "SieveScript/get", calls[0].Name)

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("SieveScript/get", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"state":     "s1",
				"list": []map[string]any{
					{
						"id":       "sieve-1",
						"name":     "main-filter",
						"blobId":   "blob-1",
						"isActive": true,
					},
					{
						"id":       "sieve-2",
						"name":     "backup",
						"blobId":   "blob-2",
						"isActive": false,
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

	accountID := jmapClient.Session.PrimaryAccounts[sieve.URI]
	require.Equal(t, jmap.ID(jmaptest.TestAccountID), accountID)

	req := &jmap.Request{}
	req.Invoke(&sieve.Get{
		Account: accountID,
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*sieve.GetResponse)
	require.Len(t, r.List, 2)
	assert.Equal(t, "main-filter", *r.List[0].Name)
	assert.True(t, r.List[0].IsActive)
	assert.Equal(t, "backup", *r.List[1].Name)
	assert.False(t, r.List[1].IsActive)
}

func TestSieveActivate(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 1)
		assert.Equal(t, "SieveScript/set", calls[0].Name)
		assert.Equal(t, "sieve-2", calls[0].Args["onSuccessActivateScript"])

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("SieveScript/set", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"oldState":  "s1",
				"newState":  "s2",
			}, calls[0].CallID),
		}
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	scriptID := jmap.ID("sieve-2")
	req := &jmap.Request{}
	req.Invoke(&sieve.Set{
		Account:                 jmapClient.Session.PrimaryAccounts[sieve.URI],
		OnSuccessActivateScript: &scriptID,
	})

	_, err := jmapClient.Do(req)
	require.NoError(t, err)
}

func TestSieveDelete(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 1)
		assert.Equal(t, "SieveScript/set", calls[0].Name)

		destroy := calls[0].Args["destroy"].([]any)
		assert.Equal(t, "sieve-1", destroy[0])

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("SieveScript/set", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"oldState":  "s1",
				"newState":  "s2",
				"destroyed": []string{"sieve-1"},
			}, calls[0].CallID),
		}
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	req := &jmap.Request{}
	req.Invoke(&sieve.Set{
		Account: jmapClient.Session.PrimaryAccounts[sieve.URI],
		Destroy: []jmap.ID{"sieve-1"},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*sieve.SetResponse)
	assert.Contains(t, r.Destroyed, jmap.ID("sieve-1"))
}

func TestSieveAccountID(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	c := &client.Client{JMAP: jmapClient}
	accountID := sieveAccountID(c)
	assert.Equal(t, jmap.ID(jmaptest.TestAccountID), accountID)
}
