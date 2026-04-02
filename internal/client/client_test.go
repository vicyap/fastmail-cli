package client

import (
	"os"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"github.com/stretchr/testify/assert"
)

func TestResolveToken_FlagTakesPrecedence(t *testing.T) {
	t.Setenv(EnvToken, "env-token")
	token, err := ResolveToken("flag-token")
	assert.NoError(t, err)
	assert.Equal(t, "flag-token", token)
}

func TestResolveToken_EnvFallback(t *testing.T) {
	t.Setenv(EnvToken, "env-token")
	token, err := ResolveToken("")
	assert.NoError(t, err)
	assert.Equal(t, "env-token", token)
}

func TestResolveToken_NoTokenReturnsError(t *testing.T) {
	// Unset env var
	os.Unsetenv(EnvToken)
	_, err := ResolveToken("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no API token found")
}

func TestClient_MailAccountID(t *testing.T) {
	jmapClient := &jmap.Client{
		Session: &jmap.Session{
			PrimaryAccounts: map[jmap.URI]jmap.ID{
				mail.URI: "account-123",
			},
		},
	}
	c := &Client{JMAP: jmapClient}
	assert.Equal(t, jmap.ID("account-123"), c.MailAccountID())
}

func TestClient_Username(t *testing.T) {
	jmapClient := &jmap.Client{
		Session: &jmap.Session{
			Username: "user@fastmail.com",
		},
	}
	c := &Client{JMAP: jmapClient}
	assert.Equal(t, "user@fastmail.com", c.Username())
}
