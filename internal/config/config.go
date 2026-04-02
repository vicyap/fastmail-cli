package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	configDirName  = "fm"
	configFileName = "config.toml"
)

// Config holds user preferences.
type Config struct {
	DefaultIdentity string `toml:"default_identity"`
	DefaultMailbox  string `toml:"default_mailbox"`
	Pager           string `toml:"pager"`
	Color           *bool  `toml:"color"`
}

// Load reads the config file from XDG config dir.
// Returns a zero Config if the file doesn't exist.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return &Config{}, nil
	}

	cfg := &Config{}
	_, err = toml.DecodeFile(path, cfg)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// Path returns the full path to the config file.
func Path() (string, error) {
	return configPath()
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configDirName, configFileName), nil
}

// DefaultMailbox returns the configured default mailbox, or "Inbox".
func (c *Config) DefaultMailboxOrInbox() string {
	if c.DefaultMailbox != "" {
		return c.DefaultMailbox
	}
	return "Inbox"
}

// ColorEnabled returns whether color output is enabled.
// Defaults to true if not set.
func (c *Config) ColorEnabled() bool {
	if c.Color != nil {
		return *c.Color
	}
	return true
}
