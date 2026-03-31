package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadAppConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := AppConfig{
		Backend: BackendNextcloud,
		Nextcloud: NextcloudConfig{
			ServerURL:   "https://cloud.example.com/",
			LoginName:   "user",
			AppPassword: "app-pass",
		},
	}

	if err := SaveAppConfig(path, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Backend != BackendNextcloud {
		t.Fatalf("expected backend %q, got %q", BackendNextcloud, loaded.Backend)
	}
	if loaded.Nextcloud.ServerURL != "https://cloud.example.com" {
		t.Fatalf("expected normalized server URL, got %q", loaded.Nextcloud.ServerURL)
	}
	if loaded.Nextcloud.RemotePath != "/remote.php/dav/files/user/.daily-tasks.json" {
		t.Fatalf("expected default remote path, got %q", loaded.Nextcloud.RemotePath)
	}
}

func TestNormalizeAppConfigClearsLocalNextcloudState(t *testing.T) {
	cfg := NormalizeAppConfig(AppConfig{
		Backend: BackendLocal,
		Nextcloud: NextcloudConfig{
			ServerURL:   "https://cloud.example.com",
			LoginName:   "user",
			AppPassword: "secret",
			RemotePath:  "/remote.php/dav/files/user/.daily-tasks.json",
		},
	})

	if cfg.Nextcloud.ServerURL != "" || cfg.Nextcloud.LoginName != "" || cfg.Nextcloud.AppPassword != "" || cfg.Nextcloud.RemotePath != "" {
		t.Fatalf("expected local backend to clear Nextcloud config, got %+v", cfg.Nextcloud)
	}
}

func TestLoadLegacyWebDAVConfigFromEnv(t *testing.T) {
	t.Setenv("DAILY_TASKS_WEBDAV_URL", "https://cloud.example.com/remote.php/dav/files/user/.daily-tasks.json")
	t.Setenv("DAILY_TASKS_WEBDAV_USER", "user")
	t.Setenv("DAILY_TASKS_WEBDAV_PASS", "app-pass")

	cfg, ok := LoadLegacyWebDAVConfigFromEnv()
	if !ok {
		t.Fatal("expected env config to be detected")
	}
	if cfg.Backend != BackendNextcloud {
		t.Fatalf("expected backend %q, got %q", BackendNextcloud, cfg.Backend)
	}
	if cfg.Nextcloud.ServerURL != "https://cloud.example.com" {
		t.Fatalf("expected server URL to be parsed, got %q", cfg.Nextcloud.ServerURL)
	}
	if cfg.Nextcloud.RemotePath != "/remote.php/dav/files/user/.daily-tasks.json" {
		t.Fatalf("expected remote path to be parsed, got %q", cfg.Nextcloud.RemotePath)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	t.Run("respects env override", func(t *testing.T) {
		t.Setenv("DAILY_TASKS_CONFIG", "/tmp/daily-tasks-config.json")
		path, err := DefaultConfigPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "/tmp/daily-tasks-config.json" {
			t.Fatalf("expected env config path, got %q", path)
		}
	})

	t.Run("uses user config dir by default", func(t *testing.T) {
		t.Setenv("DAILY_TASKS_CONFIG", "")
		path, err := DefaultConfigPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		configDir, err := os.UserConfigDir()
		if err != nil {
			t.Fatalf("failed to read user config dir: %v", err)
		}
		expected := filepath.Join(configDir, "daily-tasks", "config.json")
		if path != expected {
			t.Fatalf("expected %q, got %q", expected, path)
		}
	})
}
