package cmd

import (
	"net/http"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
	"github.com/vicyap/fastmail-cli/internal/maskedemail"
)

func TestMaskedEmailList(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 1)
		assert.Equal(t, "MaskedEmail/get", calls[0].Name)

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("MaskedEmail/get", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"state":     "s1",
				"list": []map[string]any{
					{
						"id":          "me-1",
						"email":       "abc@fastmail.com",
						"state":       "enabled",
						"forDomain":   "example.com",
						"description": "Newsletter",
					},
					{
						"id":          "me-2",
						"email":       "xyz@fastmail.com",
						"state":       "disabled",
						"forDomain":   "shop.com",
						"description": "Shopping",
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

	accountID := jmapClient.Session.PrimaryAccounts[maskedemail.URI]
	require.Equal(t, jmap.ID(jmaptest.TestAccountID), accountID)

	req := &jmap.Request{}
	req.Invoke(&maskedemail.Get{
		Account: accountID,
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)
	require.Len(t, resp.Responses, 1)

	r := resp.Responses[0].Args.(*maskedemail.GetResponse)
	require.Len(t, r.List, 2)
	assert.Equal(t, "abc@fastmail.com", r.List[0].Email)
	assert.Equal(t, "enabled", r.List[0].State)
	assert.Equal(t, "xyz@fastmail.com", r.List[1].Email)
	assert.Equal(t, "disabled", r.List[1].State)
}

func TestMaskedEmailCreate(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 1)
		assert.Equal(t, "MaskedEmail/set", calls[0].Name)

		// Verify create payload
		create := calls[0].Args["create"].(map[string]any)
		new1 := create["new1"].(map[string]any)
		assert.Equal(t, "example.com", new1["forDomain"])
		assert.Equal(t, "Test signup", new1["description"])

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("MaskedEmail/set", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"oldState":  "s1",
				"newState":  "s2",
				"created": map[string]any{
					"new1": map[string]any{
						"id":    "me-new",
						"email": "generated123@fastmail.com",
						"state": "enabled",
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

	accountID := jmapClient.Session.PrimaryAccounts[maskedemail.URI]

	req := &jmap.Request{}
	req.Invoke(&maskedemail.Set{
		Account: accountID,
		Create: map[jmap.ID]*maskedemail.MaskedEmail{
			"new1": {
				State:       "enabled",
				ForDomain:   "example.com",
				Description: "Test signup",
			},
		},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*maskedemail.SetResponse)
	require.Contains(t, r.Created, jmap.ID("new1"))
	assert.Equal(t, "generated123@fastmail.com", r.Created["new1"].Email)
}

func TestMaskedEmailUpdate(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		require.Len(t, calls, 1)
		assert.Equal(t, "MaskedEmail/set", calls[0].Name)

		update := calls[0].Args["update"].(map[string]any)
		me1 := update["me-1"].(map[string]any)
		assert.Equal(t, "disabled", me1["state"])

		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("MaskedEmail/set", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"oldState":  "s1",
				"newState":  "s2",
				"updated": map[string]any{
					"me-1": nil,
				},
			}, calls[0].CallID),
		}
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	accountID := jmapClient.Session.PrimaryAccounts[maskedemail.URI]

	req := &jmap.Request{}
	req.Invoke(&maskedemail.Set{
		Account: accountID,
		Update: map[jmap.ID]jmap.Patch{
			"me-1": {"state": "disabled"},
		},
	})

	resp, err := jmapClient.Do(req)
	require.NoError(t, err)

	r := resp.Responses[0].Args.(*maskedemail.SetResponse)
	assert.Contains(t, r.Updated, jmap.ID("me-1"))
}

func TestFilterMaskedEmails(t *testing.T) {
	list := []*maskedemail.MaskedEmail{
		{ID: "me-1", Email: "a@fm.com", State: "enabled"},
		{ID: "me-2", Email: "b@fm.com", State: "disabled"},
		{ID: "me-3", Email: "c@fm.com", State: "enabled"},
		{ID: "me-4", Email: "d@fm.com", State: "deleted"},
	}

	enabled := filterMaskedEmails(list, "enabled")
	assert.Len(t, enabled, 2)
	assert.Equal(t, "a@fm.com", enabled[0].Email)
	assert.Equal(t, "c@fm.com", enabled[1].Email)

	disabled := filterMaskedEmails(list, "disabled")
	assert.Len(t, disabled, 1)
	assert.Equal(t, "b@fm.com", disabled[0].Email)

	deleted := filterMaskedEmails(list, "deleted")
	assert.Len(t, deleted, 1)

	pending := filterMaskedEmails(list, "pending")
	assert.Empty(t, pending)
}

func TestCheckSetErrors_NotCreated(t *testing.T) {
	desc := "rate limit exceeded"
	resp := &maskedemail.SetResponse{
		NotCreated: map[jmap.ID]*jmap.SetError{
			"new1": {
				Type:        "rateLimit",
				Description: &desc,
			},
		},
	}

	err := checkSetErrors(resp, "new1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit exceeded")
}

func TestCheckSetErrors_NoCreated(t *testing.T) {
	resp := &maskedemail.SetResponse{}

	err := checkSetErrors(resp, "new1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no masked email created")
}

func TestCheckSetErrors_Success(t *testing.T) {
	resp := &maskedemail.SetResponse{
		Created: map[jmap.ID]*maskedemail.MaskedEmail{
			"new1": {
				ID:    "me-1",
				Email: "test@fm.com",
			},
		},
	}

	err := checkSetErrors(resp, "new1")
	assert.NoError(t, err)
}

func TestMaskedEmailCapabilityInRequest(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		// Verify the masked email capability is in the Using array
		assert.True(t, jmaptest.HasCapability(req, "fastmail.com/dev/maskedemail"),
			"request should include masked email capability")

		calls := jmaptest.ParseCalls(t, req)
		return []jmaptest.RawInvocation{
			jmaptest.MethodResponse("MaskedEmail/get", map[string]any{
				"accountId": jmaptest.TestAccountID,
				"state":     "s1",
				"list":      []map[string]any{},
			}, calls[0].CallID),
		}
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	req := &jmap.Request{}
	req.Invoke(&maskedemail.Get{
		Account: jmapClient.Session.PrimaryAccounts[maskedemail.URI],
	})

	_, err := jmapClient.Do(req)
	require.NoError(t, err)
}
