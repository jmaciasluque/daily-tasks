package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds persistent WebDAV credentials stored in the config file.
type Config struct {
	WebDAVURL  string `json:"webdav_url"`
	WebDAVUser string `json:"webdav_user"`
	WebDAVPass string `json:"webdav_pass"`
}

// IsEmpty returns true if any credential field is blank.
func (c Config) IsEmpty() bool {
	return c.WebDAVURL == "" || c.WebDAVUser == "" || c.WebDAVPass == ""
}

// DefaultConfigPath returns ~/.config/daily-tasks/config.json.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "daily-tasks", "config.json"), nil
}

// LoadConfig reads the config file at path. Returns an empty Config if the
// file does not exist (not an error).
func LoadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SaveConfig writes cfg to path as JSON with mode 0600, creating parent dirs.
func SaveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o600)
}
