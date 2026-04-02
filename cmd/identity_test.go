package cmd

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/jmaptest"
)

func TestPrintIdentities_Table(t *testing.T) {
	identities := []*identity.Identity{
		{ID: "ident-1", Name: "Victor Yap", Email: "victor@fastmail.com"},
		{ID: "ident-2", Name: "V. Yap", Email: "v@example.com"},
	}

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := printIdentities(identities)
	assert.NoError(t, err)
}

func TestPrintIdentities_JSON(t *testing.T) {
	identities := []*identity.Identity{
		{ID: "ident-1", Name: "Victor Yap", Email: "victor@fastmail.com"},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := printIdentities(identities)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	assert.Contains(t, out, "ident-1")
	assert.Contains(t, out, "Victor Yap")
	assert.Contains(t, out, "victor@fastmail.com")
}

func TestPrintIdentities_Empty(t *testing.T) {
	identities := []*identity.Identity{}

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := printIdentities(identities)
	assert.NoError(t, err)
}

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

func TestRunIdentityList(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Identity/get" {
				responses = append(responses, jmaptest.MethodResponse("Identity/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "ident-1", "name": "Victor", "email": "victor@fm.com"},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runIdentityList(nil, nil)
	assert.NoError(t, err)
}

func TestRunIdentityList_JSON(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "Identity/get" {
				responses = append(responses, jmaptest.MethodResponse("Identity/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "ident-1", "name": "Victor", "email": "victor@fm.com"},
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

	err := runIdentityList(nil, nil)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "victor@fm.com")
}
