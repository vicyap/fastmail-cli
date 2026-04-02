//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/emailsubmission"
	"git.sr.ht/~rockorager/go-jmap/mail/identity"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	"git.sr.ht/~rockorager/go-jmap/mail/thread"
	"git.sr.ht/~rockorager/go-jmap/mail/vacationresponse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/maskedemail"
	"github.com/vicyap/fastmail-cli/internal/searchsnippet"
	"github.com/vicyap/fastmail-cli/internal/sieve"

	_ "git.sr.ht/~rockorager/go-jmap/mail/searchsnippet"
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

// helperFindMailboxByRole returns the ID of the first mailbox with the given role.
func helperFindMailboxByRole(t *testing.T, c *jmap.Client, aid jmap.ID, role mailbox.Role) jmap.ID {
	t.Helper()
	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{Account: aid})
	resp, err := c.Do(req)
	require.NoError(t, err)
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*mailbox.GetResponse); ok {
			for _, mbox := range r.List {
				if mbox.Role == role {
					return mbox.ID
				}
			}
		}
	}
	t.Fatalf("mailbox with role %q not found", role)
	return ""
}

// helperGetIdentityID returns the first identity's ID.
func helperGetIdentityID(t *testing.T, c *jmap.Client, aid jmap.ID) jmap.ID {
	t.Helper()
	req := &jmap.Request{}
	req.Invoke(&identity.Get{Account: aid})
	resp, err := c.Do(req)
	require.NoError(t, err)
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*identity.GetResponse); ok {
			require.NotEmpty(t, r.List, "account should have at least one identity")
			return r.List[0].ID
		}
	}
	t.Fatal("no identity found")
	return ""
}

// helperCreateDraftEmail creates a draft email in the given mailbox and returns its ID.
func helperCreateDraftEmail(t *testing.T, c *jmap.Client, aid jmap.ID, mailboxID jmap.ID, subject string, bodyText string) jmap.ID {
	t.Helper()
	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: aid,
		Create: map[jmap.ID]*email.Email{
			"draft": {
				MailboxIDs: map[jmap.ID]bool{mailboxID: true},
				To:         []*mail.Address{{Email: expectedEmail}},
				From:       []*mail.Address{{Email: expectedEmail}},
				Subject:    subject,
				Keywords:   map[string]bool{"$draft": true},
				BodyValues: map[string]*email.BodyValue{
					"body": {Value: bodyText},
				},
				TextBody: []*email.BodyPart{
					{PartID: "body", Type: "text/plain"},
				},
			},
		},
	})
	resp, err := c.Do(req)
	require.NoError(t, err)
	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error creating draft: %v", r)
		case *email.SetResponse:
			require.Contains(t, r.Created, jmap.ID("draft"))
			return r.Created["draft"].ID
		}
	}
	t.Fatal("no email created")
	return ""
}

// helperDestroyEmail destroys the given email by ID.
func helperDestroyEmail(t *testing.T, c *jmap.Client, aid jmap.ID, emailID jmap.ID) {
	t.Helper()
	req := &jmap.Request{}
	req.Invoke(&email.Set{
		Account: aid,
		Destroy: []jmap.ID{emailID},
	})
	_, err := c.Do(req)
	if err != nil {
		t.Logf("warning: failed to destroy email %s: %v", emailID, err)
	}
}

func TestEmailSearchWithFilters(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)
	inboxID := helperFindMailboxByRole(t, c, aid, mailbox.RoleInbox)

	uniqueID := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	subject := fmt.Sprintf("[fm-test-search] %s", uniqueID)
	bodyText := "This is a searchable test email body for filter integration test."

	emailID := helperCreateDraftEmail(t, c, aid, inboxID, subject, bodyText)
	defer helperDestroyEmail(t, c, aid, emailID)

	// Wait for the search index to catch up.
	time.Sleep(2 * time.Second)

	// Search by text filter.
	req := &jmap.Request{}
	req.Invoke(&email.Query{
		Account: aid,
		Filter: &email.FilterCondition{
			Text: uniqueID,
		},
		Limit: 5,
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	var foundIDs []jmap.ID
	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error: %v", r)
		case *email.QueryResponse:
			foundIDs = r.IDs
		}
	}
	require.NotEmpty(t, foundIDs, "expected to find the test email via text search")
	assert.Contains(t, foundIDs, emailID)
}

func TestEmailReadFullBody(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)
	inboxID := helperFindMailboxByRole(t, c, aid, mailbox.RoleInbox)

	subject := fmt.Sprintf("[fm-test-readbody] %d", os.Getpid())
	bodyText := "Integration test body with known content: cranberry-phoenix-42."

	emailID := helperCreateDraftEmail(t, c, aid, inboxID, subject, bodyText)
	defer helperDestroyEmail(t, c, aid, emailID)

	// Fetch with FetchTextBodyValues.
	req := &jmap.Request{}
	req.Invoke(&email.Get{
		Account:             aid,
		IDs:                 []jmap.ID{emailID},
		Properties:          []string{"subject", "bodyValues", "textBody"},
		FetchTextBodyValues: true,
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error: %v", r)
		case *email.GetResponse:
			require.NotEmpty(t, r.List)
			foundEmail := r.List[0]
			require.NotEmpty(t, foundEmail.BodyValues, "bodyValues should not be empty")
			// Check that at least one body value contains our known text.
			found := false
			for _, bv := range foundEmail.BodyValues {
				if strings.Contains(bv.Value, "cranberry-phoenix-42") {
					found = true
					break
				}
			}
			assert.True(t, found, "expected body to contain known text 'cranberry-phoenix-42'")
		}
	}
}

func TestEmailMoveBetweenMailboxes(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)
	inboxID := helperFindMailboxByRole(t, c, aid, mailbox.RoleInbox)

	// Create a test mailbox.
	req := &jmap.Request{}
	req.Invoke(&mailbox.Set{
		Account: aid,
		Create: map[jmap.ID]*mailbox.Mailbox{
			"testmbox": {Name: fmt.Sprintf("fm-test-move-%d", os.Getpid())},
		},
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	var testMailboxID jmap.ID
	for _, inv := range resp.Responses {
		if r, ok := inv.Args.(*mailbox.SetResponse); ok {
			require.Contains(t, r.Created, jmap.ID("testmbox"))
			testMailboxID = r.Created["testmbox"].ID
		}
	}
	require.NotEmpty(t, testMailboxID)
	defer func() {
		req := &jmap.Request{}
		req.Invoke(&mailbox.Set{
			Account:               aid,
			Destroy:               []jmap.ID{testMailboxID},
			OnDestroyRemoveEmails: true,
		})
		_, _ = c.Do(req)
	}()

	// Create email in Inbox.
	subject := fmt.Sprintf("[fm-test-move] %d", os.Getpid())
	emailID := helperCreateDraftEmail(t, c, aid, inboxID, subject, "Move test email.")
	defer helperDestroyEmail(t, c, aid, emailID)

	// Move email to test mailbox: remove from inbox, add to test mailbox.
	moveReq := &jmap.Request{}
	moveReq.Invoke(&email.Set{
		Account: aid,
		Update: map[jmap.ID]jmap.Patch{
			emailID: {
				"mailboxIds": map[jmap.ID]bool{testMailboxID: true},
			},
		},
	})

	moveResp, err := c.Do(moveReq)
	require.NoError(t, err)

	for _, inv := range moveResp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error moving email: %v", r)
		case *email.SetResponse:
			require.Contains(t, r.Updated, emailID)
		}
	}

	// Verify the email is in the new mailbox.
	getReq := &jmap.Request{}
	getReq.Invoke(&email.Get{
		Account:    aid,
		IDs:        []jmap.ID{emailID},
		Properties: []string{"mailboxIds"},
	})

	getResp, err := c.Do(getReq)
	require.NoError(t, err)

	for _, inv := range getResp.Responses {
		if r, ok := inv.Args.(*email.GetResponse); ok {
			require.NotEmpty(t, r.List)
			assert.True(t, r.List[0].MailboxIDs[testMailboxID], "email should be in the test mailbox")
			assert.False(t, r.List[0].MailboxIDs[inboxID], "email should not be in inbox")
		}
	}
}

func TestEmailFlagAndUnflag(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)
	inboxID := helperFindMailboxByRole(t, c, aid, mailbox.RoleInbox)

	subject := fmt.Sprintf("[fm-test-flag] %d", os.Getpid())
	emailID := helperCreateDraftEmail(t, c, aid, inboxID, subject, "Flag test email.")
	defer helperDestroyEmail(t, c, aid, emailID)

	// Set $flagged to true.
	flagReq := &jmap.Request{}
	flagReq.Invoke(&email.Set{
		Account: aid,
		Update: map[jmap.ID]jmap.Patch{
			emailID: {
				"keywords/$flagged": true,
			},
		},
	})

	flagResp, err := c.Do(flagReq)
	require.NoError(t, err)
	for _, inv := range flagResp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error flagging email: %v", r)
		case *email.SetResponse:
			require.Contains(t, r.Updated, emailID)
		}
	}

	// Verify flagged.
	getReq := &jmap.Request{}
	getReq.Invoke(&email.Get{
		Account:    aid,
		IDs:        []jmap.ID{emailID},
		Properties: []string{"keywords"},
	})

	getResp, err := c.Do(getReq)
	require.NoError(t, err)
	for _, inv := range getResp.Responses {
		if r, ok := inv.Args.(*email.GetResponse); ok {
			require.NotEmpty(t, r.List)
			assert.True(t, r.List[0].Keywords["$flagged"], "email should be flagged")
		}
	}

	// Remove $flagged (set to nil to remove the keyword).
	unflagReq := &jmap.Request{}
	unflagReq.Invoke(&email.Set{
		Account: aid,
		Update: map[jmap.ID]jmap.Patch{
			emailID: {
				"keywords/$flagged": nil,
			},
		},
	})

	unflagResp, err := c.Do(unflagReq)
	require.NoError(t, err)
	for _, inv := range unflagResp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error unflagging email: %v", r)
		case *email.SetResponse:
			require.Contains(t, r.Updated, emailID)
		}
	}

	// Verify unflagged.
	getReq2 := &jmap.Request{}
	getReq2.Invoke(&email.Get{
		Account:    aid,
		IDs:        []jmap.ID{emailID},
		Properties: []string{"keywords"},
	})

	getResp2, err := c.Do(getReq2)
	require.NoError(t, err)
	for _, inv := range getResp2.Responses {
		if r, ok := inv.Args.(*email.GetResponse); ok {
			require.NotEmpty(t, r.List)
			assert.False(t, r.List[0].Keywords["$flagged"], "email should not be flagged")
		}
	}
}

func TestEmailMarkReadUnread(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)
	inboxID := helperFindMailboxByRole(t, c, aid, mailbox.RoleInbox)

	subject := fmt.Sprintf("[fm-test-seen] %d", os.Getpid())
	emailID := helperCreateDraftEmail(t, c, aid, inboxID, subject, "Read/unread test email.")
	defer helperDestroyEmail(t, c, aid, emailID)

	// Mark as read ($seen = true).
	readReq := &jmap.Request{}
	readReq.Invoke(&email.Set{
		Account: aid,
		Update: map[jmap.ID]jmap.Patch{
			emailID: {
				"keywords/$seen": true,
			},
		},
	})

	readResp, err := c.Do(readReq)
	require.NoError(t, err)
	for _, inv := range readResp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error marking read: %v", r)
		case *email.SetResponse:
			require.Contains(t, r.Updated, emailID)
		}
	}

	// Verify read.
	getReq := &jmap.Request{}
	getReq.Invoke(&email.Get{
		Account:    aid,
		IDs:        []jmap.ID{emailID},
		Properties: []string{"keywords"},
	})

	getResp, err := c.Do(getReq)
	require.NoError(t, err)
	for _, inv := range getResp.Responses {
		if r, ok := inv.Args.(*email.GetResponse); ok {
			require.NotEmpty(t, r.List)
			assert.True(t, r.List[0].Keywords["$seen"], "email should be marked as read")
		}
	}

	// Mark as unread ($seen = nil).
	unreadReq := &jmap.Request{}
	unreadReq.Invoke(&email.Set{
		Account: aid,
		Update: map[jmap.ID]jmap.Patch{
			emailID: {
				"keywords/$seen": nil,
			},
		},
	})

	unreadResp, err := c.Do(unreadReq)
	require.NoError(t, err)
	for _, inv := range unreadResp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error marking unread: %v", r)
		case *email.SetResponse:
			require.Contains(t, r.Updated, emailID)
		}
	}

	// Verify unread.
	getReq2 := &jmap.Request{}
	getReq2.Invoke(&email.Get{
		Account:    aid,
		IDs:        []jmap.ID{emailID},
		Properties: []string{"keywords"},
	})

	getResp2, err := c.Do(getReq2)
	require.NoError(t, err)
	for _, inv := range getResp2.Responses {
		if r, ok := inv.Args.(*email.GetResponse); ok {
			require.NotEmpty(t, r.List)
			assert.False(t, r.List[0].Keywords["$seen"], "email should be marked as unread")
		}
	}
}

func TestEmailSendToSelfAndVerify(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)
	draftsID := helperFindMailboxByRole(t, c, aid, mailbox.RoleDrafts)
	identityID := helperGetIdentityID(t, c, aid)

	uniqueID := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	subject := fmt.Sprintf("[fm-test-send] %s", uniqueID)

	// Create draft.
	createReq := &jmap.Request{}
	createReq.Invoke(&email.Set{
		Account: aid,
		Create: map[jmap.ID]*email.Email{
			"draft": {
				MailboxIDs: map[jmap.ID]bool{draftsID: true},
				From:       []*mail.Address{{Email: expectedEmail}},
				To:         []*mail.Address{{Email: expectedEmail}},
				Subject:    subject,
				Keywords:   map[string]bool{"$draft": true},
				BodyValues: map[string]*email.BodyValue{
					"body": {Value: "Send-to-self integration test."},
				},
				TextBody: []*email.BodyPart{
					{PartID: "body", Type: "text/plain"},
				},
			},
		},
	})

	createResp, err := c.Do(createReq)
	require.NoError(t, err)

	var draftEmailID jmap.ID
	for _, inv := range createResp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error creating draft: %v", r)
		case *email.SetResponse:
			require.Contains(t, r.Created, jmap.ID("draft"))
			draftEmailID = r.Created["draft"].ID
		}
	}
	require.NotEmpty(t, draftEmailID)
	t.Logf("Created draft: %s", draftEmailID)

	// Submit via EmailSubmission/set.
	submitReq := &jmap.Request{}
	submitReq.Invoke(&emailsubmission.Set{
		Account: aid,
		Create: map[jmap.ID]*emailsubmission.EmailSubmission{
			"sub": {
				IdentityID: identityID,
				EmailID:    draftEmailID,
			},
		},
		OnSuccessUpdateEmail: map[jmap.ID]jmap.Patch{
			"#sub": {
				"mailboxIds/" + string(draftsID): nil,
				"keywords/$draft":                nil,
			},
		},
	})

	submitResp, err := c.Do(submitReq)
	require.NoError(t, err)
	for _, inv := range submitResp.Responses {
		if r, ok := inv.Args.(*jmap.MethodError); ok {
			t.Fatalf("JMAP error submitting email: %v", r)
		}
		if r, ok := inv.Args.(*emailsubmission.SetResponse); ok {
			require.Contains(t, r.Created, jmap.ID("sub"), "submission should be created")
		}
	}
	t.Log("Email submitted")

	// Wait for delivery.
	time.Sleep(3 * time.Second)

	// Query for the sent email by subject.
	queryReq := &jmap.Request{}
	queryReq.Invoke(&email.Query{
		Account: aid,
		Filter: &email.FilterCondition{
			Subject: uniqueID,
		},
		Limit: 5,
	})

	queryResp, err := c.Do(queryReq)
	require.NoError(t, err)

	var sentIDs []jmap.ID
	for _, inv := range queryResp.Responses {
		if r, ok := inv.Args.(*email.QueryResponse); ok {
			sentIDs = r.IDs
		}
	}
	require.NotEmpty(t, sentIDs, "expected to find the sent email by subject")
	t.Logf("Found %d email(s) matching subject", len(sentIDs))

	// Clean up all found emails.
	cleanReq := &jmap.Request{}
	cleanReq.Invoke(&email.Set{
		Account: aid,
		Destroy: sentIDs,
	})
	_, err = c.Do(cleanReq)
	if err != nil {
		t.Logf("warning: cleanup failed: %v", err)
	}
}

func TestEmailReplyHeaders(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)
	inboxID := helperFindMailboxByRole(t, c, aid, mailbox.RoleInbox)

	// Create original email.
	originalSubject := fmt.Sprintf("[fm-test-reply] %d", os.Getpid())
	originalID := helperCreateDraftEmail(t, c, aid, inboxID, originalSubject, "Original message for reply test.")
	defer helperDestroyEmail(t, c, aid, originalID)

	// Get the original email's messageId.
	getReq := &jmap.Request{}
	getReq.Invoke(&email.Get{
		Account:    aid,
		IDs:        []jmap.ID{originalID},
		Properties: []string{"messageId"},
	})

	getResp, err := c.Do(getReq)
	require.NoError(t, err)

	var originalMessageID []string
	for _, inv := range getResp.Responses {
		if r, ok := inv.Args.(*email.GetResponse); ok {
			require.NotEmpty(t, r.List)
			originalMessageID = r.List[0].MessageID
		}
	}
	require.NotEmpty(t, originalMessageID, "original email must have a messageId")

	// Create reply with InReplyTo and References.
	replySubject := fmt.Sprintf("Re: %s", originalSubject)
	replyReq := &jmap.Request{}
	replyReq.Invoke(&email.Set{
		Account: aid,
		Create: map[jmap.ID]*email.Email{
			"reply": {
				MailboxIDs: map[jmap.ID]bool{inboxID: true},
				From:       []*mail.Address{{Email: expectedEmail}},
				To:         []*mail.Address{{Email: expectedEmail}},
				Subject:    replySubject,
				InReplyTo:  originalMessageID,
				References: originalMessageID,
				Keywords:   map[string]bool{"$draft": true},
				BodyValues: map[string]*email.BodyValue{
					"body": {Value: "This is a reply."},
				},
				TextBody: []*email.BodyPart{
					{PartID: "body", Type: "text/plain"},
				},
			},
		},
	})

	replyResp, err := c.Do(replyReq)
	require.NoError(t, err)

	var replyID jmap.ID
	for _, inv := range replyResp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error creating reply: %v", r)
		case *email.SetResponse:
			require.Contains(t, r.Created, jmap.ID("reply"))
			replyID = r.Created["reply"].ID
		}
	}
	require.NotEmpty(t, replyID)
	defer helperDestroyEmail(t, c, aid, replyID)

	// Verify the reply has correct headers.
	verifyReq := &jmap.Request{}
	verifyReq.Invoke(&email.Get{
		Account:    aid,
		IDs:        []jmap.ID{replyID},
		Properties: []string{"inReplyTo", "references"},
	})

	verifyResp, err := c.Do(verifyReq)
	require.NoError(t, err)

	for _, inv := range verifyResp.Responses {
		if r, ok := inv.Args.(*email.GetResponse); ok {
			require.NotEmpty(t, r.List)
			replyEmail := r.List[0]
			assert.Equal(t, originalMessageID, replyEmail.InReplyTo, "InReplyTo should match original messageId")
			assert.Equal(t, originalMessageID, replyEmail.References, "References should match original messageId")
		}
	}
}

func TestAttachmentUploadDownload(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)
	inboxID := helperFindMailboxByRole(t, c, aid, mailbox.RoleInbox)

	// Upload a blob with known content.
	knownContent := "attachment-content-cranberry-42-integration-test"
	uploadResp, err := c.Upload(aid, bytes.NewReader([]byte(knownContent)))
	require.NoError(t, err)
	require.NotEmpty(t, uploadResp.ID)
	t.Logf("Uploaded blob: %s (size: %d)", uploadResp.ID, uploadResp.Size)

	// Create an email referencing the blob as attachment.
	subject := fmt.Sprintf("[fm-test-attachment] %d", os.Getpid())
	createReq := &jmap.Request{}
	createReq.Invoke(&email.Set{
		Account: aid,
		Create: map[jmap.ID]*email.Email{
			"att": {
				MailboxIDs: map[jmap.ID]bool{inboxID: true},
				From:       []*mail.Address{{Email: expectedEmail}},
				To:         []*mail.Address{{Email: expectedEmail}},
				Subject:    subject,
				Keywords:   map[string]bool{"$draft": true},
				BodyValues: map[string]*email.BodyValue{
					"body": {Value: "Email with attachment."},
				},
				TextBody: []*email.BodyPart{
					{PartID: "body", Type: "text/plain"},
				},
				Attachments: []*email.BodyPart{
					{
						BlobID:      uploadResp.ID,
						Type:        "application/octet-stream",
						Name:        "test.bin",
						Disposition: "attachment",
					},
				},
			},
		},
	})

	createResp, err := c.Do(createReq)
	require.NoError(t, err)

	var emailID jmap.ID
	for _, inv := range createResp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error creating email with attachment: %v", r)
		case *email.SetResponse:
			require.Contains(t, r.Created, jmap.ID("att"))
			emailID = r.Created["att"].ID
		}
	}
	require.NotEmpty(t, emailID)
	defer helperDestroyEmail(t, c, aid, emailID)

	// Fetch the email to get the attachment's blobId.
	getReq := &jmap.Request{}
	getReq.Invoke(&email.Get{
		Account:        aid,
		IDs:            []jmap.ID{emailID},
		Properties:     []string{"attachments"},
		BodyProperties: []string{"blobId", "name", "type", "size"},
	})

	getResp, err := c.Do(getReq)
	require.NoError(t, err)

	var attachmentBlobID jmap.ID
	for _, inv := range getResp.Responses {
		if r, ok := inv.Args.(*email.GetResponse); ok {
			require.NotEmpty(t, r.List)
			require.NotEmpty(t, r.List[0].Attachments, "email should have attachments")
			attachmentBlobID = r.List[0].Attachments[0].BlobID
		}
	}
	require.NotEmpty(t, attachmentBlobID)

	// Download the blob and verify content.
	body, err := c.Download(aid, attachmentBlobID)
	require.NoError(t, err)
	defer body.Close()

	downloaded, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, knownContent, string(downloaded), "downloaded content should match uploaded content")
}

func TestMailboxRename(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)

	originalName := fmt.Sprintf("fm-test-rename-%d", os.Getpid())
	renamedName := fmt.Sprintf("fm-test-renamed-%d", os.Getpid())

	// Create mailbox.
	createReq := &jmap.Request{}
	createReq.Invoke(&mailbox.Set{
		Account: aid,
		Create: map[jmap.ID]*mailbox.Mailbox{
			"mbox": {Name: originalName},
		},
	})

	createResp, err := c.Do(createReq)
	require.NoError(t, err)

	var mboxID jmap.ID
	for _, inv := range createResp.Responses {
		if r, ok := inv.Args.(*mailbox.SetResponse); ok {
			require.Contains(t, r.Created, jmap.ID("mbox"))
			mboxID = r.Created["mbox"].ID
		}
	}
	require.NotEmpty(t, mboxID)
	defer func() {
		req := &jmap.Request{}
		req.Invoke(&mailbox.Set{
			Account:               aid,
			Destroy:               []jmap.ID{mboxID},
			OnDestroyRemoveEmails: true,
		})
		_, _ = c.Do(req)
	}()

	// Rename mailbox.
	renameReq := &jmap.Request{}
	renameReq.Invoke(&mailbox.Set{
		Account: aid,
		Update: map[jmap.ID]jmap.Patch{
			mboxID: {
				"name": renamedName,
			},
		},
	})

	renameResp, err := c.Do(renameReq)
	require.NoError(t, err)
	for _, inv := range renameResp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error renaming mailbox: %v", r)
		case *mailbox.SetResponse:
			require.Contains(t, r.Updated, mboxID)
		}
	}

	// Verify name changed.
	getReq := &jmap.Request{}
	getReq.Invoke(&mailbox.Get{Account: aid})

	getResp, err := c.Do(getReq)
	require.NoError(t, err)

	var foundRenamed bool
	for _, inv := range getResp.Responses {
		if r, ok := inv.Args.(*mailbox.GetResponse); ok {
			for _, mbox := range r.List {
				if mbox.ID == mboxID {
					assert.Equal(t, renamedName, mbox.Name, "mailbox name should be updated")
					foundRenamed = true
				}
			}
		}
	}
	assert.True(t, foundRenamed, "renamed mailbox should be found")
}

func TestMaskedEmailStateTransitions(t *testing.T) {
	c := newClient(t)
	aid := c.Session.PrimaryAccounts[maskedemail.URI]
	if aid == "" {
		t.Skip("masked email capability not available")
	}

	// Create masked email (state=enabled).
	createReq := &jmap.Request{}
	createReq.Invoke(&maskedemail.Set{
		Account: aid,
		Create: map[jmap.ID]*maskedemail.MaskedEmail{
			"test": {
				State:       "enabled",
				ForDomain:   "fm-test-transitions.example",
				Description: "State transition integration test",
			},
		},
	})

	createResp, err := c.Do(createReq)
	require.NoError(t, err)

	var meID jmap.ID
	for _, inv := range createResp.Responses {
		if r, ok := inv.Args.(*maskedemail.SetResponse); ok {
			require.Contains(t, r.Created, jmap.ID("test"))
			meID = r.Created["test"].ID
		}
	}
	require.NotEmpty(t, meID)
	defer func() {
		// Clean up: delete the masked email.
		req := &jmap.Request{}
		req.Invoke(&maskedemail.Set{
			Account: aid,
			Update: map[jmap.ID]jmap.Patch{
				meID: {"state": "deleted"},
			},
		})
		_, _ = c.Do(req)
	}()

	// Disable it.
	disableReq := &jmap.Request{}
	disableReq.Invoke(&maskedemail.Set{
		Account: aid,
		Update: map[jmap.ID]jmap.Patch{
			meID: {"state": "disabled"},
		},
	})

	disableResp, err := c.Do(disableReq)
	require.NoError(t, err)
	for _, inv := range disableResp.Responses {
		if r, ok := inv.Args.(*jmap.MethodError); ok {
			t.Fatalf("JMAP error disabling masked email: %v", r)
		}
	}

	// Verify state=disabled.
	getReq := &jmap.Request{}
	getReq.Invoke(&maskedemail.Get{
		Account: aid,
		IDs:     []jmap.ID{meID},
	})

	getResp, err := c.Do(getReq)
	require.NoError(t, err)
	for _, inv := range getResp.Responses {
		if r, ok := inv.Args.(*maskedemail.GetResponse); ok {
			require.NotEmpty(t, r.List)
			assert.Equal(t, "disabled", r.List[0].State)
		}
	}

	// Re-enable it.
	enableReq := &jmap.Request{}
	enableReq.Invoke(&maskedemail.Set{
		Account: aid,
		Update: map[jmap.ID]jmap.Patch{
			meID: {"state": "enabled"},
		},
	})

	enableResp, err := c.Do(enableReq)
	require.NoError(t, err)
	for _, inv := range enableResp.Responses {
		if r, ok := inv.Args.(*jmap.MethodError); ok {
			t.Fatalf("JMAP error re-enabling masked email: %v", r)
		}
	}

	// Verify state=enabled.
	getReq2 := &jmap.Request{}
	getReq2.Invoke(&maskedemail.Get{
		Account: aid,
		IDs:     []jmap.ID{meID},
	})

	getResp2, err := c.Do(getReq2)
	require.NoError(t, err)
	for _, inv := range getResp2.Responses {
		if r, ok := inv.Args.(*maskedemail.GetResponse); ok {
			require.NotEmpty(t, r.List)
			assert.Equal(t, "enabled", r.List[0].State)
		}
	}
}

func TestSieveScriptLifecycle(t *testing.T) {
	c := newClient(t)

	// Check for sieve capability.
	if _, ok := c.Session.RawCapabilities[sieve.URI]; !ok {
		t.Skip("sieve capability not available")
	}

	aid := accountID(c)

	// Upload a simple sieve script blob.
	sieveContent := "require \"fileinto\";\nkeep;"
	uploadResp, err := c.Upload(aid, bytes.NewReader([]byte(sieveContent)))
	require.NoError(t, err)
	require.NotEmpty(t, uploadResp.ID)
	t.Logf("Uploaded sieve script blob: %s", uploadResp.ID)

	scriptName := fmt.Sprintf("fm-test-sieve-%d", os.Getpid())

	// Create a SieveScript.
	createReq := &jmap.Request{}
	createReq.Invoke(&sieve.Set{
		Account: aid,
		Create: map[jmap.ID]*sieve.SieveScript{
			"script": {
				Name:   &scriptName,
				BlobID: uploadResp.ID,
			},
		},
	})

	createResp, err := c.Do(createReq)
	require.NoError(t, err)

	var scriptID jmap.ID
	for _, inv := range createResp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error creating sieve script: %v", r)
		case *sieve.SetResponse:
			require.Contains(t, r.Created, jmap.ID("script"))
			scriptID = r.Created["script"].ID
		}
	}
	require.NotEmpty(t, scriptID)
	t.Logf("Created sieve script: %s", scriptID)
	defer func() {
		// Deactivate then destroy.
		deactivateDestroy := true
		req := &jmap.Request{}
		req.Invoke(&sieve.Set{
			Account:                   aid,
			OnSuccessDeactivateScript: &deactivateDestroy,
			Destroy:                   []jmap.ID{scriptID},
		})
		_, _ = c.Do(req)
	}()

	// List scripts to verify it exists.
	listReq := &jmap.Request{}
	listReq.Invoke(&sieve.Get{Account: aid})

	listResp, err := c.Do(listReq)
	require.NoError(t, err)

	var foundScript bool
	for _, inv := range listResp.Responses {
		if r, ok := inv.Args.(*sieve.GetResponse); ok {
			for _, script := range r.List {
				if script.ID == scriptID {
					foundScript = true
					assert.Equal(t, scriptName, *script.Name)
				}
			}
		}
	}
	assert.True(t, foundScript, "created script should appear in list")

	// Activate it.
	activateReq := &jmap.Request{}
	scriptIDRef := scriptID
	activateReq.Invoke(&sieve.Set{
		Account:                 aid,
		OnSuccessActivateScript: &scriptIDRef,
	})

	activateResp, err := c.Do(activateReq)
	require.NoError(t, err)
	for _, inv := range activateResp.Responses {
		if r, ok := inv.Args.(*jmap.MethodError); ok {
			t.Fatalf("JMAP error activating sieve script: %v", r)
		}
	}

	// Verify it is active.
	verifyReq := &jmap.Request{}
	verifyReq.Invoke(&sieve.Get{
		Account: aid,
		IDs:     []jmap.ID{scriptID},
	})

	verifyResp, err := c.Do(verifyReq)
	require.NoError(t, err)
	for _, inv := range verifyResp.Responses {
		if r, ok := inv.Args.(*sieve.GetResponse); ok {
			require.NotEmpty(t, r.List)
			assert.True(t, r.List[0].IsActive, "script should be active after activation")
		}
	}

	// Deactivate it.
	deactivate := true
	deactivateReq := &jmap.Request{}
	deactivateReq.Invoke(&sieve.Set{
		Account:                   aid,
		OnSuccessDeactivateScript: &deactivate,
	})

	deactivateResp, err := c.Do(deactivateReq)
	require.NoError(t, err)
	for _, inv := range deactivateResp.Responses {
		if r, ok := inv.Args.(*jmap.MethodError); ok {
			t.Fatalf("JMAP error deactivating sieve script: %v", r)
		}
	}

	// Verify it is inactive.
	verifyReq2 := &jmap.Request{}
	verifyReq2.Invoke(&sieve.Get{
		Account: aid,
		IDs:     []jmap.ID{scriptID},
	})

	verifyResp2, err := c.Do(verifyReq2)
	require.NoError(t, err)
	for _, inv := range verifyResp2.Responses {
		if r, ok := inv.Args.(*sieve.GetResponse); ok {
			require.NotEmpty(t, r.List)
			assert.False(t, r.List[0].IsActive, "script should be inactive after deactivation")
		}
	}
}

func TestSearchSnippets(t *testing.T) {
	c := newClient(t)
	aid := accountID(c)
	inboxID := helperFindMailboxByRole(t, c, aid, mailbox.RoleInbox)

	uniqueWord := fmt.Sprintf("zephyrsnippet%d", os.Getpid())
	subject := fmt.Sprintf("[fm-test-snippet] %d", os.Getpid())
	bodyText := fmt.Sprintf("This email contains the unique word %s for snippet testing.", uniqueWord)

	emailID := helperCreateDraftEmail(t, c, aid, inboxID, subject, bodyText)
	defer helperDestroyEmail(t, c, aid, emailID)

	// Wait for the search index to catch up.
	time.Sleep(2 * time.Second)

	// Use SearchSnippet/get to search for the unique word.
	req := &jmap.Request{}
	req.Invoke(&searchsnippet.Get{
		Account:  aid,
		Filter:   &email.FilterCondition{Text: uniqueWord},
		EmailIDs: []jmap.ID{emailID},
	})

	resp, err := c.Do(req)
	require.NoError(t, err)

	var foundSnippet bool
	for _, inv := range resp.Responses {
		switch r := inv.Args.(type) {
		case *jmap.MethodError:
			t.Fatalf("JMAP error: %v", r)
		case *searchsnippet.GetResponse:
			for _, snippet := range r.List {
				if snippet.Email == emailID {
					foundSnippet = true
					// The preview should contain our unique word (possibly with HTML markup).
					if snippet.Preview != "" {
						assert.Contains(t, snippet.Preview, uniqueWord,
							"snippet preview should contain the search term")
					}
					t.Logf("Snippet subject: %q, preview: %q", snippet.Subject, snippet.Preview)
				}
			}
		}
	}
	assert.True(t, foundSnippet, "should find a search snippet for the test email")
}
