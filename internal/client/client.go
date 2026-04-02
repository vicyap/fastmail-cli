package client

import (
	"fmt"
	"os"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"github.com/zalando/go-keyring"
)

const (
	SessionEndpoint = "https://api.fastmail.com/jmap/session"
	KeyringService  = "fastmail-cli"
	KeyringUser     = "api-token"
	EnvToken        = "FASTMAIL_API_TOKEN"
)

// Client wraps a go-jmap Client with Fastmail-specific helpers.
type Client struct {
	JMAP *jmap.Client
}

// ResolveToken returns the API token using the resolution order:
// 1. flagToken (--token flag)
// 2. FASTMAIL_API_TOKEN env var
// 3. OS keyring
func ResolveToken(flagToken string) (string, error) {
	if flagToken != "" {
		return flagToken, nil
	}
	if token := os.Getenv(EnvToken); token != "" {
		return token, nil
	}
	token, err := keyring.Get(KeyringService, KeyringUser)
	if err != nil {
		return "", fmt.Errorf("no API token found: set FASTMAIL_API_TOKEN, use --token, or run 'fm auth login'")
	}
	return token, nil
}

// StoreToken saves the API token to the OS keyring.
func StoreToken(token string) error {
	return keyring.Set(KeyringService, KeyringUser, token)
}

// DeleteToken removes the API token from the OS keyring.
func DeleteToken() error {
	return keyring.Delete(KeyringService, KeyringUser)
}

// New creates a new Client, resolves the token, and authenticates.
// It is a variable so tests can replace it with a stub.
var New = newClient

func newClient(flagToken string) (*Client, error) {
	token, err := ResolveToken(flagToken)
	if err != nil {
		return nil, err
	}

	jmapClient := &jmap.Client{
		SessionEndpoint: SessionEndpoint,
	}
	jmapClient.WithAccessToken(token)

	if err := jmapClient.Authenticate(); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	return &Client{JMAP: jmapClient}, nil
}

// MailAccountID returns the primary account ID for mail operations.
func (c *Client) MailAccountID() jmap.ID {
	return c.JMAP.Session.PrimaryAccounts[mail.URI]
}

// Username returns the authenticated username.
func (c *Client) Username() string {
	return c.JMAP.Session.Username
}
