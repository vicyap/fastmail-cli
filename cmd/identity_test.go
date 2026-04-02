package cmd

import (
	"net/http"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
)

func TestIdentityList(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 1)
		assert.Equal(t, "Identity/get", calls[0].Name)

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("Identity/get", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"state":     "s1",
				"list": []map[string]any{
					{
						"id":    "ident-1",
						"name":  "Victor Yap",
						"email": "victor@fastmail.com",
					},
					{
						"id":    "ident-2",
						"name":  "V. Yap",
						"email": "v@example.com",
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

	// Identity/get requires submission capability
	req := &jmap.Request{}
	req.Invoke(&identity.Get{
		Account: jmapClient.Session.PrimaryAccounts["urn:ietf:params:jmap:submission"],
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*identity.GetResponse)
	require.Len(t, r.List, 2)
	assert.Equal(t, "Victor Yap", r.List[0].Name)
	assert.Equal(t, "victor@fastmail.com", r.List[0].Email)
	assert.Equal(t, "V. Yap", r.List[1].Name)
}
