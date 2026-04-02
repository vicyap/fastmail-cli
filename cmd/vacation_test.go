package cmd

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/vacationresponse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/client"
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

func TestPrintVacation_Enabled(t *testing.T) {
	subject := "Out of office"
	body := "I'm on vacation."
	fromDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)

	v := &vacationresponse.VacationResponse{
		IsEnabled: true,
		Subject:   &subject,
		TextBody:  &body,
		FromDate:  &fromDate,
		ToDate:    &toDate,
	}

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = false

	err := printVacation(v)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()
	assert.Contains(t, out, "yes")
	assert.Contains(t, out, "Out of office")
	assert.Contains(t, out, "2026-04-01")
	assert.Contains(t, out, "2026-04-15")
	assert.Contains(t, out, "I'm on vacation.")
}

func TestPrintVacation_Disabled(t *testing.T) {
	v := &vacationresponse.VacationResponse{
		IsEnabled: false,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = false

	err := printVacation(v)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "no")
}

func TestPrintVacation_JSON(t *testing.T) {
	subject := "OOO"
	v := &vacationresponse.VacationResponse{
		IsEnabled: true,
		Subject:   &subject,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldJSON := jsonOutput
	jsonOutput = true

	err := printVacation(v)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "OOO")
}

func TestFormatVacationSetError_WithDescription(t *testing.T) {
	desc := "invalid date range"
	err := &jmap.SetError{
		Type:        "invalidProperties",
		Description: &desc,
	}
	result := formatVacationSetError(err)
	assert.Equal(t, "invalidProperties: invalid date range", result)
}

func TestFormatVacationSetError_WithoutDescription(t *testing.T) {
	err := &jmap.SetError{
		Type: "forbidden",
	}
	result := formatVacationSetError(err)
	assert.Equal(t, "forbidden", result)
}

func TestVacationAccountID(t *testing.T) {
	server := jmaptest.NewServer(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	jmapClient := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}
	require.NoError(t, jmapClient.Authenticate())

	c := &client.Client{JMAP: jmapClient}
	accountID := vacationAccountID(c)
	assert.Equal(t, jmap.ID(jmaptest.TestAccountID), accountID)
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

func TestRunVacationGet_Success(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "VacationResponse/get" {
				responses = append(responses, jmaptest.MethodResponse("VacationResponse/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list": []map[string]any{
						{"id": "singleton", "isEnabled": true, "subject": "OOO", "textBody": "Away"},
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

	err := runVacationGet(nil, nil)
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	jsonOutput = oldJSON

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	assert.Contains(t, buf.String(), "yes")
}

func TestRunVacationGet_NoConfig(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "VacationResponse/get" {
				responses = append(responses, jmaptest.MethodResponse("VacationResponse/get", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"state":     "s1",
					"list":      []map[string]any{},
				}, call.CallID))
			}
		}
		return responses
	})

	err := runVacationGet(nil, nil)
	assert.NoError(t, err)
}

func TestRunVacationSet_Success(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "VacationResponse/set" {
				responses = append(responses, jmaptest.MethodResponse("VacationResponse/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"singleton": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	oldSubject := vacationSubject
	oldBody := vacationBody
	oldHTMLBody := vacationHTMLBody
	oldFrom := vacationFrom
	oldTo := vacationTo
	vacationSubject = "Away"
	vacationBody = "On vacation"
	vacationHTMLBody = "<p>On vacation</p>"
	vacationFrom = "2026-04-01"
	vacationTo = "2026-04-15"
	defer func() {
		vacationSubject = oldSubject
		vacationBody = oldBody
		vacationHTMLBody = oldHTMLBody
		vacationFrom = oldFrom
		vacationTo = oldTo
	}()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runVacationSet(nil, nil)
	assert.NoError(t, err)
}

func TestRunVacationSet_BadFromDate(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	oldFrom := vacationFrom
	vacationFrom = "invalid"
	defer func() { vacationFrom = oldFrom }()

	err := runVacationSet(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --from date")
}

func TestRunVacationSet_BadToDate(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		return nil
	})

	oldFrom := vacationFrom
	oldTo := vacationTo
	vacationFrom = ""
	vacationTo = "invalid"
	defer func() {
		vacationFrom = oldFrom
		vacationTo = oldTo
	}()

	err := runVacationSet(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --to date")
}

func TestRunVacationDisable_Success(t *testing.T) {
	withTestClient(t, func(t *testing.T, req *jmaptest.RawRequest) []jmaptest.RawInvocation {
		calls := jmaptest.ParseCalls(t, req)
		var responses []jmaptest.RawInvocation
		for _, call := range calls {
			if call.Name == "VacationResponse/set" {
				responses = append(responses, jmaptest.MethodResponse("VacationResponse/set", map[string]any{
					"accountId": jmaptest.TestAccountID,
					"oldState":  "s1",
					"newState":  "s2",
					"updated":   map[string]any{"singleton": nil},
				}, call.CallID))
			}
		}
		return responses
	})

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	err := runVacationDisable(nil, nil)
	assert.NoError(t, err)
}
