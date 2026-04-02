package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "", cfg.DefaultIdentity)
	assert.Equal(t, "", cfg.DefaultMailbox)
	assert.Equal(t, "", cfg.Pager)
	assert.Nil(t, cfg.Color)
}

func TestLoad_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	dir := filepath.Join(tmpDir, configDirName)
	require.NoError(t, os.MkdirAll(dir, 0755))

	configContent := `
default_identity = "ident-2"
default_mailbox = "Archive"
pager = "less -R"
color = false
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, configFileName), []byte(configContent), 0644))

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "ident-2", cfg.DefaultIdentity)
	assert.Equal(t, "Archive", cfg.DefaultMailbox)
	assert.Equal(t, "less -R", cfg.Pager)
	require.NotNil(t, cfg.Color)
	assert.False(t, *cfg.Color)
}

func TestDefaultMailboxOrInbox(t *testing.T) {
	cfg := &Config{}
	assert.Equal(t, "Inbox", cfg.DefaultMailboxOrInbox())

	cfg.DefaultMailbox = "Archive"
	assert.Equal(t, "Archive", cfg.DefaultMailboxOrInbox())
}

func TestColorEnabled(t *testing.T) {
	cfg := &Config{}
	assert.True(t, cfg.ColorEnabled())

	f := false
	cfg.Color = &f
	assert.False(t, cfg.ColorEnabled())

	tr := true
	cfg.Color = &tr
	assert.True(t, cfg.ColorEnabled())
}
