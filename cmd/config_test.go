package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vicyap/fastmail-cli/internal/config"
)

func TestConfigInit(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	err := runConfigInit(nil, nil)
	require.NoError(t, err)

	path, err := config.Path()
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "fm configuration")
	assert.Contains(t, string(content), "default_identity")
	assert.Contains(t, string(content), "default_mailbox")
	assert.Contains(t, string(content), "pager")
	assert.Contains(t, string(content), "color")
}

func TestConfigInit_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create the file first
	dir := filepath.Join(tmpDir, "fm")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte("existing"), 0644))

	err := runConfigInit(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestConfigShow(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create a config
	dir := filepath.Join(tmpDir, "fm")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`
default_identity = "ident-1"
default_mailbox = "Archive"
`), 0644))

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "ident-1", cfg.DefaultIdentity)
	assert.Equal(t, "Archive", cfg.DefaultMailbox)
}
