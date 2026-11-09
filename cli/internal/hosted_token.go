package internal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func DefaultHostedTokenPath() (string, error) {
	if envPath := strings.TrimSpace(os.Getenv("DAILY_TASKS_TOKEN")); envPath != "" {
		return envPath, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "daily-tasks", "token"), nil
}

func SaveHostedToken(path, token string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(token)+"\n"), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func LoadHostedToken(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func DeleteHostedToken(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
