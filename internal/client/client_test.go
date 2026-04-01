package client

import (
	"os"
	"testing"

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
