//go:build integration

package integration

import (
	"fmt"
	"os"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/identity"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	"git.sr.ht/~rockorager/go-jmap/mail/thread"
	"git.sr.ht/~rockorager/go-jmap/mail/vacationresponse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/maskedemail"

	_ "git.sr.ht/~rockorager/go-jmap/mail/emailsubmission"
	_ "git.sr.ht/~rockorager/go-jmap/mail/searchsnippet"
	_ "github.com/vicyap/fastmail-cli/internal/sieve"
)

const expectedEmail = "vicyap_fastmail_cli_test@fastmail.com"

func newClient(t *testing.T) *jmap.Client {
	t.Helper()
	token := os.Getenv("FASTMAIL_TEST_TOKEN")
	if token == "" {
		t.Skip("FASTMAIL_TEST_TOKEN not set")
	}

	client := &jmap.Client{
		SessionEndpoint: "https://api.fastmail.com/jmap/session",
	}
	client.WithAccessToken(token)
	require.NoError(t, client.Authenticate())

	// Safety check: refuse to run against the wrong account
	if client.Session.Username != expectedEmail {
		t.Fatalf("Safety check failed: expected account %s, got %s. Refusing to run integration tests against an unknown account.", expectedEmail, client.Session.Username)
	}

	return client
}

func accountID(c *jmap.Client) jmap.ID {
	return c.Session.PrimaryAccounts[mail.URI]
}

func TestSession(t *testing.T) {
	c := newClient(t)

	assert.Equal(t, expectedEmail, c.Session.Username)
	assert.NotEmpty(t, c.Session.APIURL)
	assert.NotEmpty(t, c.Session.PrimaryAccounts[mail.URI])
}

func TestMailboxList(t *testing.T) {
	c := newClient(t)

	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{
		Account: accountID(c),
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	var mailboxes []*mailbox.Mailbox
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*mailbox.GetResponse); ok {
			mailboxes = r.List
		}
	}

	require.NotEmpty(t, mailboxes, "account should have at least one mailbox")

	// Should have an Inbox
	var hasInbox bool
	for _, mbox := range mailboxes {
		if mbox.Role == mailbox.RoleInbox {
			hasInbox = true
			break
		}
	}
	assert.True(t, hasInbox, "account should have an Inbox")
}

func TestIdentityList(t *testing.T) {
	c := newClient(t)

	req := &jmap.Request{}
	req.Invoke(&identity.Get{
		Account: accountID(c),
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*identity.GetResponse); ok {
			require.NotEmpty(t, r.List, "account should have at least one identity")
			assert.NotEmpty(t, r.List[0].Email)
		}
	}
}

func TestEmailQueryAndGet(t *testing.T) {
	c := newClient(t)

	req := &jmap.Request{}
	queryCallID := req.Invoke(&email.Query{
		Account: accountID(c),
		Sort: []*email.SortComparator{
			{Property: "receivedAt", IsAscending: false},
		},
		Limit: 5,
	})

	req.Invoke(&email.Get{
		Account:    accountID(c),
		Properties: []string{"id", "subject", "from", "receivedAt", "preview"},
		ReferenceIDs: &jmap.ResultReference{
			ResultOf: queryCallID,
			Name:     "Email/query",
			Path:     "/ids",
		},
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	for _, inv := range resp.Responses {
		switch inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error: %v", inv.Args)
		}
	}
	// Just verifying the query doesn't error -- inbox may be empty
}

func TestMailboxCreateAndDelete(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)

	// Create
	req := &jmap.Request{}
	req.Invoke(&mailbox.Set{
		Account: aid,
		Create: map[jmap.ID]*mailbox.Mailbox{
			"test": {Name: "fm-integration-test"},
		},
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	var createdID jmap.ID
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*mailbox.SetResponse); ok {
			require.Contains(t, r.Created, jmap.ID("test"))
			createdID = r.Created["test"].ID
		}
	}
	require.NotEmpty(t, createdID)

	// Delete
	req2 := &jmap.Request{}
	req2.Invoke(&mailbox.Set{
		Account:               aid,
		Destroy:               []jmap.ID{createdID},
		OnDestroyRemoveEmails: true,
	})

	resp2, err := c.Do(req2)
	require.NoError(t, err)

	for _, inv := range resp2.Responses {
		if r, ok := inv.Args.(*mailbox.SetResponse); ok {
			assert.Contains(t, r.Destroyed, createdID)
		}
	}
}

func TestMaskedEmailList(t *testing.T) {
	c := newClient(t)
	aid := c.Session.PrimaryAccounts[maskedemail.URI]
	if aid == "" {
		t.Skip("masked email capability not available")
	}

	req := &jmap.Request{}
	req.Invoke(&maskedemail.Get{
		Account: aid,
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	for _, inv := range resp.Responses {
		switch inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error: %v", inv.Args)
		case *maskedemail.GetResponse:
			// Success -- list may be empty
		}
	}
}

func TestMaskedEmailCreateAndDelete(t *testing.T) {
	c := newClient(t)
	aid := c.Session.PrimaryAccounts[maskedemail.URI]
	if aid == "" {
		t.Skip("masked email capability not available")
	}

	// Create
	req := &jmap.Request{}
	req.Invoke(&maskedemail.Set{
		Account: aid,
		Create: map[jmap.ID]*maskedemail.MaskedEmail{
			"test": {
				State:       "enabled",
				ForDomain:   "fm-integration-test.example",
				Description: "Integration test",
			},
		},
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	var createdID jmap.ID
	var createdEmail string
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*maskedemail.SetResponse); ok {
			require.Contains(t, r.Created, jmap.ID("test"))
			createdID = r.Created["test"].ID
			createdEmail = r.Created["test"].Email
		}
	}
	require.NotEmpty(t, createdID)
	require.NotEmpty(t, createdEmail)
	t.Logf("Created masked email: %s (ID: %s)", createdEmail, createdID)

	// Delete (set state to deleted)
	req2 := &jmap.Request{}
	req2.Invoke(&maskedemail.Set{
		Account: aid,
		Update: map[jmap.ID]jmap.Patch{
			createdID: {"state": "deleted"},
		},
	})

	_, err = c.Do(req2)
	require.NoError(t, err)
}

func TestVacationGet(t *testing.T) {
	c := newClient(t)

	vacationAID := c.Session.PrimaryAccounts[vacationresponse.URI]
	if vacationAID == "" {
		// Check if the capability is even advertised
		if _, ok := c.Session.RawCapabilities[vacationresponse.URI]; !ok {
			t.Skip("vacation response capability not available on this account")
		}
		vacationAID = accountID(c)
	}

	req := &jmap.Request{}
	req.Invoke(&vacationresponse.Get{
		Account: vacationAID,
		IDs:     []jmap.ID{"singleton"},
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	for _, inv := range resp.Responses {
		switch inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error: %v", inv.Args)
		case *vacationresponse.GetResponse:
			// Success
		}
	}
}

func TestThreadGet(t *testing.T) {
	c := newClient(t)

	// First query for any email to get a thread ID
	req := &jmap.Request{}
	req.Invoke(&email.Query{
		Account: accountID(c),
		Limit:   1,
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	var emailIDs []jmap.ID
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*email.QueryResponse); ok {
			emailIDs = r.IDs
		}
	}

	if len(emailIDs) == 0 {
		t.Skip("no emails in account to test threads")
	}

	// Get the email's thread ID
	req2 := &jmap.Request{}
	req2.Invoke(&email.Get{
		Account:    accountID(c),
		IDs:        emailIDs[:1],
		Properties: []string{"threadId"},
	})

	resp2, err := c.Do(req2)
	require.NoError(t, err)

	var threadID jmap.ID
	for _, inv := range resp2.Responses {
		if r, ok := inv.Args.(*email.GetResponse); ok {
			require.NotEmpty(t, r.List)
			threadID = r.List[0].ThreadID
		}
	}
	require.NotEmpty(t, threadID)

	// Get the thread
	req3 := &jmap.Request{}
	req3.Invoke(&thread.Get{
		Account: accountID(c),
		IDs:     []jmap.ID{threadID},
	})

	resp3, err := c.Do(req3)
	require.NoError(t, err)

	for _, inv := range resp3.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error: %v", r)
		case *thread.GetResponse:
			require.NotEmpty(t, r.List)
			assert.NotEmpty(t, r.List[0].EmailIDs)
		}
	}
}

func TestSendAndCleanup(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)

	// Get identity
	req := &jmap.Request{}
	req.Invoke(&identity.Get{Account: aid})

	resp, err := c.Do(req)
	require.NoError(t, err)

	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*identity.GetResponse); ok {
			require.NotEmpty(t, r.List)
		}
	}

	// Find drafts mailbox
	req2 := &jmap.Request{}
	req2.Invoke(&mailbox.Get{Account: aid})

	resp2, err := c.Do(req2)
	require.NoError(t, err)

	var draftsID jmap.ID
	for _, inv := range resp2.Responses {
		if r, ok := inv.Args.(*mailbox.GetResponse); ok {
			for _, mbox := range r.List {
				if mbox.Role == mailbox.RoleDrafts {
					draftsID = mbox.ID
				}
			}
		}
	}
	require.NotEmpty(t, draftsID)

	// Send email to self
	testSubject := fmt.Sprintf("[fm-test] integration %d", os.Getpid())

	req3 := &jmap.Request{}
	req3.Invoke(&email.Set{
		Account: aid,
		Create: map[jmap.ID]*email.Email{
			"draft": {
				MailboxIDs: map[jmap.ID]bool{draftsID: true},
				To:         []*mail.Address{{Email: expectedEmail}},
				Subject:    testSubject,
				Keywords:   map[string]bool{"$draft": true},
				BodyValues: map[string]*email.BodyValue{
					"body": {Value: "Integration test email. Safe to delete."},
				},
				TextBody: []*email.BodyPart{
					{PartID: "body", Type: "text/plain"},
				},
			},
		},
	})

	resp3, err := c.Do(req3)
	require.NoError(t, err)

	var createdEmailID jmap.ID
	for _, inv := range resp3.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error creating draft: %v", r)
		case *email.SetResponse:
			require.Contains(t, r.Created, jmap.ID("draft"))
			createdEmailID = r.Created["draft"].ID
		}
	}
	require.NotEmpty(t, createdEmailID)
	t.Logf("Created draft email: %s (subject: %s)", createdEmailID, testSubject)

	// Clean up: destroy the draft
	req4 := &jmap.Request{}
	req4.Invoke(&email.Set{
		Account: aid,
		Destroy: []jmap.ID{createdEmailID},
	})

	resp4, err := c.Do(req4)
	require.NoError(t, err)

	for _, inv := range resp4.Responses {
		if r, ok := inv.Args.(*email.SetResponse); ok {
			assert.Contains(t, r.Destroyed, createdEmailID)
		}
	}
	t.Log("Cleaned up test email")
}
