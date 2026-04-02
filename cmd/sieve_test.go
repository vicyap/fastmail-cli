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

func TestPrintSieveScripts_Table(t *testing.T) {
	name1 := "main-filter"
	name2 := "backup"

	scripts := []*sieve.SieveScript{
		{ID: "sieve-1", Name: &name1, IsActive: true},
		{ID: "sieve-2", Name: &name2, IsActive: false},
		{ID: "sieve-3", IsActive: false}, // nil Name
	}

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := printSieveScripts(scripts)
	assert.NoError(t, err)
}

func TestPrintSieveScripts_JSON(t *testing.T) {
	name := "test-script"
	scripts := []*sieve.SieveScript{
		{ID: "sieve-1", Name: &name, IsActive: true},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := printSieveScripts(scripts)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "sieve-1")
	assert.Contains(t, buf.String(), "test-script")
}

func TestFormatSieveSetError_WithDescription(t *testing.T) {
	desc := "script syntax error"
	err := &jmap.SetError{
		Type:        "invalidScript",
		Description: &desc,
	}
	result := formatSieveSetError(err)
	assert.Equal(t, "invalidScript: script syntax error", result)
}

func TestFormatSieveSetError_WithoutDescription(t *testing.T) {
	err := &jmap.SetError{
		Type: "notFound",
	}
	result := formatSieveSetError(err)
	assert.Equal(t, "notFound", result)
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

func TestRunSieveList(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/get" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "sieve-1", "name": "main", "isActive": true},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runSieveList(nil, nil)
	assert.NoError(t, err)
}

func TestRunSieveActivate(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/set" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runSieveActivate(nil, []string{"sieve-1"})
	assert.NoError(t, err)
}

func TestRunSieveActivate_JSON(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/set" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
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

	err := runSieveActivate(nil, []string{"sieve-1"})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "activated")
}

func TestRunSieveDeactivate(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/set" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runSieveDeactivate(nil, nil)
	assert.NoError(t, err)
}

func TestRunSieveDelete(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/set" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"destroyed": []string{"sieve-1"},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runSieveDelete(nil, []string{"sieve-1"})
	assert.NoError(t, err)
}

func TestRunSieveDelete_NotDestroyed(t *testing.T) {
	desc := "script is active"
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/set" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/set", map[string]any{
					"accountId":    jmaptest.TestAccountID,
					"oldState":     "s1",
					"newState":     "s1",
					"notDestroyed": map[string]any{
						"sieve-1": map[string]any{"type": "scriptIsActive", "description": desc},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	err := runSieveDelete(nil, []string{"sieve-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "script is active")
}

func TestRunSieveGet(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/get" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "sieve-1", "name": "main", "blobId": "blob-1", "isActive": true},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	// When not jsonOutput, it tries to download the blob
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = false

	err := runSieveGet(nil, []string{"sieve-1"})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	// The download handler returns "test-blob-content"
	assert.Equal(t, "test-blob-content", buf.String())
}

func TestRunSieveGet_JSON(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/get" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "sieve-1", "name": "main", "blobId": "blob-1", "isActive": true},
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

	err := runSieveGet(nil, []string{"sieve-1"})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "sieve-1")
}

func TestRunSieveGet_NotFound(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/get" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{},
				}, call.CallID))
			}
		}
		return responses
	})

	err := runSieveGet(nil, []string{"nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "script not found")
}

func TestRunSieveSet_FromFile(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/set" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"new1": map[string]any{"id": "sieve-new", "name": "test-script"},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	tmpDir := t.TempDir()
	scriptFile := tmpDir + "/filter.sieve"
	require.NoError(t, os.WriteFile(scriptFile, []byte("require \"fileinto\";\nfileinto \"INBOX\";\n"), 0644))

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runSieveSet(nil, []string{"test-script", scriptFile})
	assert.NoError(t, err)
}

func TestRunSieveSet_JSON(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/set" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"created": map[string]any{
						"new1": map[string]any{"id": "sieve-new", "name": "test-script"},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	tmpDir := t.TempDir()
	scriptFile := tmpDir + "/filter.sieve"
	require.NoError(t, os.WriteFile(scriptFile, []byte("keep;\n"), 0644))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := runSieveSet(nil, []string{"test-script", scriptFile})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "sieve-new")
}

func TestRunSieveSet_NotCreated(t *testing.T) {
	desc := "invalid script"
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/set" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/set", map[string]any{
					"accountId":  jmaptest.TestAccountID,
					"oldState":   "s1",
					"newState":   "s1",
					"notCreated": map[string]any{
						"new1": map[string]any{"type": "invalidScript", "description": desc},
					},
				}, call.CallID))
			}
		}
		return responses
	})

	tmpDir := t.TempDir()
	scriptFile := tmpDir + "/bad.sieve"
	require.NoError(t, os.WriteFile(scriptFile, []byte("bad script\n"), 0644))

	err := runSieveSet(nil, []string{"bad-script", scriptFile})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid script")
}

func TestRunSieveSet_FileNotFound(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	err := runSieveSet(nil, []string{"test", "/nonexistent/file.sieve"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestRunSieveValidate_FromFile(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/validate" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/validate", map[string]any{
					"accountId": jmaptest.TestAccountID,
				}, call.CallID))
			}
		}
		return responses
	})

	tmpDir := t.TempDir()
	scriptFile := tmpDir + "/valid.sieve"
	require.NoError(t, os.WriteFile(scriptFile, []byte("keep;\n"), 0644))

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runSieveValidate(nil, []string{scriptFile})
	assert.NoError(t, err)
}

func TestRunSieveValidate_JSON(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/validate" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/validate", map[string]any{
					"accountId": jmaptest.TestAccountID,
				}, call.CallID))
			}
		}
		return responses
	})

	tmpDir := t.TempDir()
	scriptFile := tmpDir + "/valid.sieve"
	require.NoError(t, os.WriteFile(scriptFile, []byte("keep;\n"), 0644))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := runSieveValidate(nil, []string{scriptFile})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "valid")
}

func TestRunSieveValidate_Error(t *testing.T) {
	desc := "syntax error on line 1"
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "SieveScript/validate" {
				responses = append(responses, jmaptest.MethodResponse("SieveScript/validate", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"error":     map[string]any{"type": "invalidScript", "description": desc},
				}, call.CallID))
			}
		}
		return responses
	})

	tmpDir := t.TempDir()
	scriptFile := tmpDir + "/bad.sieve"
	require.NoError(t, os.WriteFile(scriptFile, []byte("bad\n"), 0644))

	err := runSieveValidate(nil, []string{scriptFile})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error")
}

func TestRunSieveValidate_FileNotFound(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	err := runSieveValidate(nil, []string{"/nonexistent/file.sieve"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open file")
}
