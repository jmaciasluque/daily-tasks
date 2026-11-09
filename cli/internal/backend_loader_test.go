package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRemoteBackendReturnsHostedBackendWithConfiguredToken(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	tokenPath := filepath.Join(dir, "token")
	t.Setenv("DAILY_TASKS_CONFIG", configPath)
	t.Setenv("DAILY_TASKS_TOKEN", tokenPath)

	if err := SaveAppConfig(configPath, AppConfig{
		Backend: BackendHosted,
		Hosted:  HostedConfig{APIURL: "https://api.example.com/"},
	}); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}
	if err := SaveHostedToken(tokenPath, "jwt-token"); err != nil {
		t.Fatalf("SaveHostedToken: %v", err)
	}

	backend, err := LoadRemoteBackend()
	if err != nil {
		t.Fatalf("LoadRemoteBackend: %v", err)
	}
	hosted, ok := backend.(*HostedBackend)
	if !ok {
		t.Fatalf("backend type = %T, want *HostedBackend", backend)
	}
	if hosted.APIURL != "https://api.example.com" {
		t.Fatalf("APIURL = %q", hosted.APIURL)
	}
	if hosted.Token != "jwt-token" {
		t.Fatalf("Token = %q", hosted.Token)
	}
}

func TestLoadRemoteBackendHostedWithoutTokenReturnsHelpfulError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	tokenPath := filepath.Join(dir, "missing-token")
	t.Setenv("DAILY_TASKS_CONFIG", configPath)
	t.Setenv("DAILY_TASKS_TOKEN", tokenPath)

	if err := SaveAppConfig(configPath, AppConfig{Backend: BackendHosted}); err != nil {
		t.Fatalf("SaveAppConfig: %v", err)
	}

	_, err := LoadRemoteBackend()
	if err == nil {
		t.Fatal("expected error")
	}
	if !os.IsNotExist(err) && err != ErrHostedTokenMissing {
		t.Fatalf("error = %v, want missing-token error", err)
	}
}
