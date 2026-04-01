package jmaptest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"

	// Register all mail methods so response deserialization works.
	_ "git.sr.ht/~rockorager/go-jmap/mail/email"
	_ "git.sr.ht/~rockorager/go-jmap/mail/emailsubmission"
	_ "git.sr.ht/~rockorager/go-jmap/mail/identity"
	_ "git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	_ "git.sr.ht/~rockorager/go-jmap/mail/searchsnippet"
	_ "git.sr.ht/~rockorager/go-jmap/mail/thread"
	_ "git.sr.ht/~rockorager/go-jmap/mail/vacationresponse"
	_ "github.com/vicyap/fastmail-cli/internal/maskedemail"
	_ "github.com/vicyap/fastmail-cli/internal/sieve"
)

const (
	TestAccountID = "u12345"
	TestUsername   = "test@fastmail.com"
)

// SessionJSON returns a valid JMAP session response for testing.
func SessionJSON(apiURL string) string {
	return `{
		"capabilities": {
			"urn:ietf:params:jmap:core": {
				"maxSizeUpload": 50000000,
				"maxConcurrentUpload": 4,
				"maxSizeRequest": 10000000,
				"maxConcurrentRequests": 4,
				"maxCallsInRequest": 16,
				"maxObjectsInGet": 500,
				"maxObjectsInSet": 500,
				"collationAlgorithms": []
			},
			"urn:ietf:params:jmap:mail": {
				"maxMailboxesPerEmail": 1000,
				"maxMailboxDepth": 10,
				"maxSizeMailboxName": 490,
				"maxSizeAttachmentsPerEmail": 50000000,
				"emailQuerySortOptions": ["receivedAt", "sentAt", "size", "from", "to", "subject"],
				"mayCreateTopLevelMailbox": true
			},
			"urn:ietf:params:jmap:submission": {},
			"urn:ietf:params:jmap:vacationresponse": {},
			"urn:ietf:params:jmap:sieve": {},
			"https://www.fastmail.com/dev/maskedemail": {}
		},
		"accounts": {
			"` + TestAccountID + `": {
				"name": "` + TestUsername + `",
				"isPersonal": true,
				"isReadOnly": false,
				"accountCapabilities": {
					"urn:ietf:params:jmap:core": {},
					"urn:ietf:params:jmap:mail": {},
					"urn:ietf:params:jmap:submission": {},
					"https://www.fastmail.com/dev/maskedemail": {}
				}
			}
		},
		"primaryAccounts": {
			"urn:ietf:params:jmap:core": "` + TestAccountID + `",
			"urn:ietf:params:jmap:mail": "` + TestAccountID + `",
			"urn:ietf:params:jmap:submission": "` + TestAccountID + `",
			"urn:ietf:params:jmap:vacationresponse": "` + TestAccountID + `",
			"urn:ietf:params:jmap:sieve": "` + TestAccountID + `",
			"https://www.fastmail.com/dev/maskedemail": "` + TestAccountID + `"
		},
		"username": "` + TestUsername + `",
		"apiUrl": "` + apiURL + `/api",
		"downloadUrl": "` + apiURL + `/download/{accountId}/{blobId}/{name}?type={type}",
		"uploadUrl": "` + apiURL + `/upload/{accountId}/",
		"eventSourceUrl": "` + apiURL + `/event/",
		"state": "test-state-1"
	}`
}

// APIHandler is a function that handles JMAP API requests.
// It receives the parsed request and returns method responses.
type APIHandler func(t *testing.T, req *RawRequest) []RawInvocation

// RawRequest is the raw JMAP request body.
type RawRequest struct {
	Using []string          `json:"using"`
	Calls []json.RawMessage `json:"methodCalls"`
}

// RawInvocation is a raw JMAP method response triple.
type RawInvocation [3]any

// NewServer creates a test HTTP server that serves JMAP session and API endpoints.
func NewServer(t *testing.T, handler APIHandler) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	var server *httptest.Server

	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(SessionJSON(server.URL)))
	})

	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var req RawRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}

		responses := handler(t, &req)

		resp := map[string]any{
			"methodResponses": responses,
			"sessionState":    "test-state-1",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/upload/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		resp := map[string]any{
			"accountId": TestAccountID,
			"blobId":    "blob-upload-1",
			"type":      r.Header.Get("Content-Type"),
			"size":      len(body),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("test-blob-content"))
	})

	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// NewClient creates a go-jmap Client connected to the test server.
func NewClient(t *testing.T, handler APIHandler) *jmap.Client {
	t.Helper()
	server := NewServer(t, handler)

	client := &jmap.Client{
		SessionEndpoint: server.URL + "/session",
		HttpClient:      http.DefaultClient,
	}

	if err := client.Authenticate(); err != nil {
		t.Fatalf("failed to authenticate test client: %v", err)
	}

	return client
}

// MailAccountID returns the test account ID for mail operations.
func MailAccountID(client *jmap.Client) jmap.ID {
	return client.Session.PrimaryAccounts[mail.URI]
}

// ParseMethodCall parses a raw method call from a JMAP request.
func ParseMethodCall(t *testing.T, raw json.RawMessage) (name string, args map[string]any, callID string) {
	t.Helper()
	var triple [3]json.RawMessage
	if err := json.Unmarshal(raw, &triple); err != nil {
		t.Fatalf("failed to unmarshal method call: %v", err)
	}

	if err := json.Unmarshal(triple[0], &name); err != nil {
		t.Fatalf("failed to unmarshal method name: %v", err)
	}

	if err := json.Unmarshal(triple[1], &args); err != nil {
		t.Fatalf("failed to unmarshal method args: %v", err)
	}

	if err := json.Unmarshal(triple[2], &callID); err != nil {
		t.Fatalf("failed to unmarshal call ID: %v", err)
	}

	return name, args, callID
}

// MethodResponse creates a JMAP method response triple.
func MethodResponse(name string, args any, callID string) RawInvocation {
	return RawInvocation{name, args, callID}
}

// ParseCallArgs unmarshals a raw method call's arguments into the provided map
// and returns them along with the method name and callID.
func ParseCalls(t *testing.T, req *RawRequest) []struct {
	Name   string
	Args   map[string]any
	CallID string
} {
	t.Helper()
	var calls []struct {
		Name   string
		Args   map[string]any
		CallID string
	}
	for _, raw := range req.Calls {
		name, args, callID := ParseMethodCall(t, raw)
		calls = append(calls, struct {
			Name   string
			Args   map[string]any
			CallID string
		}{name, args, callID})
	}
	return calls
}

// HasCapability checks if the request uses a specific capability.
func HasCapability(req *RawRequest, uri string) bool {
	for _, u := range req.Using {
		if strings.Contains(u, uri) {
			return true
		}
	}
	return false
}
