package cmd

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/client"
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

func TestPrintMaskedEmails_Table(t *testing.T) {
	list := []*maskedemail.MaskedEmail{
		{
			ID:            "me-1",
			Email:         "abc@fastmail.com",
			State:         "enabled",
			ForDomain:     "example.com",
			Description:   "Newsletter",
			LastMessageAt: "2026-03-15T10:00:00Z",
		},
		{
			ID:    "me-2",
			Email: "xyz@fastmail.com",
			State: "disabled",
			// empty ForDomain, Description, LastMessageAt to test fallback to "-"
		},
	}

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := printMaskedEmails(list)
	assert.NoError(t, err)
}

func TestPrintMaskedEmails_JSON(t *testing.T) {
	list := []*maskedemail.MaskedEmail{
		{ID: "me-1", Email: "abc@fastmail.com", State: "enabled"},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := printMaskedEmails(list)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "abc@fastmail.com")
}

func TestFormatSetError_WithDescription(t *testing.T) {
	desc := "rate limit exceeded"
	err := &jmap.SetError{
		Type:        "rateLimit",
		Description: &desc,
	}
	result := formatSetError(err)
	assert.Equal(t, "rateLimit: rate limit exceeded", result)
}

func TestFormatSetError_WithoutDescription(t *testing.T) {
	err := &jmap.SetError{
		Type: "notFound",
	}
	result := formatSetError(err)
	assert.Equal(t, "notFound", result)
}

func TestCheckSetErrors_MissingCreateID(t *testing.T) {
	resp := &maskedemail.SetResponse{
		Created: map[jmap.ID]*maskedemail.MaskedEmail{
			"other": {ID: "me-1", Email: "test@fm.com"},
		},
	}

	err := checkSetErrors(resp, "new1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "creation response missing")
}

func TestMaskedEmailAccountID(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	c := &client.Client{JMAP: jmapClient}
	accountID := maskedEmailAccountID(c)
	assert.Equal(t, jmap.ID(jmaptest.TestAccountID), accountID)
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

func TestRunMaskedEmailList(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "MaskedEmail/get" {
				responses = append(responses, jmaptest.MethodResponse("MaskedEmail/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "me-1", "email": "a@fm.com", "state": "enabled", "forDomain": "example.com"},
						{"id": "me-2", "email": "b@fm.com", "state": "disabled"},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	oldState := maskedEmailState
	maskedEmailState = ""
	defer func() {
		jsonOutput = oldJSON
		maskedEmailState = oldState
	}()

	err := runMaskedEmailList(nil, nil)
	assert.NoError(t, err)
}

func TestRunMaskedEmailList_FilterByState(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "MaskedEmail/get" {
				responses = append(responses, jmaptest.MethodResponse("MaskedEmail/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "me-1", "email": "a@fm.com", "state": "enabled"},
						{"id": "me-2", "email": "b@fm.com", "state": "disabled"},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	oldState := maskedEmailState
	maskedEmailState = "enabled"
	defer func() {
		jsonOutput = oldJSON
		maskedEmailState = oldState
	}()

	err := runMaskedEmailList(nil, nil)
	assert.NoError(t, err)
}

func TestRunMaskedEmailCreate(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "MaskedEmail/set" {
				responses = append(responses, jmaptest.MethodResponse("MaskedEmail/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"new1": map[string]any{"id": "me-new", "email": "gen@fm.com", "state": "enabled"},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = false
	oldDomain := maskedEmailDomain
	oldDesc := maskedEmailDescription
	oldPrefix := maskedEmailPrefix
	maskedEmailDomain = "example.com"
	maskedEmailDescription = "Test"
	maskedEmailPrefix = "prefix"
	defer func() {
		jsonOutput = oldJSON
		maskedEmailDomain = oldDomain
		maskedEmailDescription = oldDesc
		maskedEmailPrefix = oldPrefix
	}()

	err := runMaskedEmailCreate(nil, nil)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "gen@fm.com")
}

func TestRunMaskedEmailCreate_JSON(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "MaskedEmail/set" {
				responses = append(responses, jmaptest.MethodResponse("MaskedEmail/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"new1": map[string]any{"id": "me-new", "email": "gen@fm.com", "state": "enabled"},
					},
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
	oldDomain := maskedEmailDomain
	maskedEmailDomain = ""
	defer func() {
		jsonOutput = oldJSON
		maskedEmailDomain = oldDomain
	}()

	err := runMaskedEmailCreate(nil, nil)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "me-new")
}

func TestRunUpdateMaskedEmailState(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "MaskedEmail/set" {
				responses = append(responses, jmaptest.MethodResponse("MaskedEmail/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"me-1": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := updateMaskedEmailState("me-1", "disabled")
	assert.NoError(t, err)
}

func TestRunUpdateMaskedEmailState_JSON(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "MaskedEmail/set" {
				responses = append(responses, jmaptest.MethodResponse("MaskedEmail/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"me-1": nil},
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

	err := updateMaskedEmailState("me-1", "enabled")
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "enabled")
}

func TestRunUpdateMaskedEmailState_NotUpdated(t *testing.T) {
	desc := "not found"
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "MaskedEmail/set" {
				responses = append(responses, jmaptest.MethodResponse("MaskedEmail/set", map[string]any{
					"accountId":  jmaptest.TestAccountID,
					"oldState":   "s1",
					"newState":   "s1",
					"notUpdated": map[string]any{
						"me-1": map[string]any{"type": "notFound", "description": desc},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	err := updateMaskedEmailState("me-1", "disabled")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunMaskedEmailEnable(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "MaskedEmail/set" {
				responses = append(responses, jmaptest.MethodResponse("MaskedEmail/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"me-1": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runMaskedEmailEnable(nil, []string{"me-1"})
	assert.NoError(t, err)
}

func TestRunMaskedEmailDisable(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "MaskedEmail/set" {
				responses = append(responses, jmaptest.MethodResponse("MaskedEmail/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"me-1": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runMaskedEmailDisable(nil, []string{"me-1"})
	assert.NoError(t, err)
}

func TestRunMaskedEmailDelete(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "MaskedEmail/set" {
				responses = append(responses, jmaptest.MethodResponse("MaskedEmail/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"me-1": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runMaskedEmailDelete(nil, []string{"me-1"})
	assert.NoError(t, err)
}
