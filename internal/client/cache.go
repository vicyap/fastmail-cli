package client

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheDirName  = "fastmail-cli"
	cacheFileName = "session.json"
	cacheTTL      = 15 * time.Minute
)

type cachedSession struct {
	Data      json.RawMessage `json:"data"`
	CachedAt  time.Time       `json:"cachedAt"`
	SessionState string       `json:"sessionState"`
}

func cacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheDirName), nil
}

func cachePath() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheFileName), nil
}

// SaveSessionCache writes the JMAP session data to the XDG cache directory.
func SaveSessionCache(data []byte, sessionState string) error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	cached := cachedSession{
		Data:         data,
		CachedAt:     time.Now(),
		SessionState: sessionState,
	}

	encoded, err := json.Marshal(cached)
	if err != nil {
		return err
	}

	path, err := cachePath()
	if err != nil {
		return err
	}

	return os.WriteFile(path, encoded, 0600)
}

// LoadSessionCache reads the cached JMAP session if it exists and is fresh.
// Returns nil if the cache is missing, expired, or corrupt.
func LoadSessionCache() ([]byte, string, error) {
	path, err := cachePath()
	if err != nil {
		return nil, "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	var cached cachedSession
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, "", err
	}

	if time.Since(cached.CachedAt) > cacheTTL {
		return nil, "", nil
	}

	return cached.Data, cached.SessionState, nil
}

// InvalidateSessionCache removes the cached session file.
func InvalidateSessionCache() error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
