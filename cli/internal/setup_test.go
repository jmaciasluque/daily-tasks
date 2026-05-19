package internal

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSetupModelSavesLocalConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := newSetupModel(path)

	m, _ = updateSetupModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != setupStepConfirm {
		t.Fatalf("expected confirm step, got %v", m.step)
	}

	var cmd tea.Cmd
	m, cmd = updateSetupModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	runSetupCommand(t, &m, cmd)

	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if cfg.Backend != BackendLocal {
		t.Fatalf("expected local backend, got %q", cfg.Backend)
	}
	if !IsBackendConfigured(cfg) {
		t.Fatal("expected local backend to be configured")
	}
}

func TestSetupModelSavesNextcloudConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m := newSetupModel(path)
	m.backendChoice = 1

	m, _ = updateSetupModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != setupStepServerURL {
		t.Fatalf("expected server URL step, got %v", m.step)
	}

	m.serverInput.SetValue("https://cloud.example.com/apps/files/")
	m, _ = updateSetupModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m.usernameInput.SetValue("daily-user")
	m, _ = updateSetupModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m.passwordInput.SetValue("app-pass")
	m, _ = updateSetupModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != setupStepConfirm {
		t.Fatalf("expected confirm step, got %v", m.step)
	}

	var cmd tea.Cmd
	m, cmd = updateSetupModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	runSetupCommand(t, &m, cmd)

	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if cfg.Backend != BackendNextcloud {
		t.Fatalf("expected nextcloud backend, got %q", cfg.Backend)
	}
	if cfg.Nextcloud.ServerURL != "https://cloud.example.com" {
		t.Fatalf("expected normalized server URL, got %q", cfg.Nextcloud.ServerURL)
	}
	if cfg.Nextcloud.LoginName != "daily-user" {
		t.Fatalf("expected login name to be saved, got %q", cfg.Nextcloud.LoginName)
	}
	if cfg.Nextcloud.AppPassword != "app-pass" {
		t.Fatal("expected app password to be saved")
	}
	if cfg.Nextcloud.RemotePath != "/remote.php/dav/files/daily-user/.daily-tasks.json" {
		t.Fatalf("expected default remote path, got %q", cfg.Nextcloud.RemotePath)
	}
	if !IsBackendConfigured(cfg) {
		t.Fatal("expected nextcloud backend to be configured")
	}
}

func TestSetupModelRequiresServerURL(t *testing.T) {
	m := newSetupModel(filepath.Join(t.TempDir(), "config.json"))
	m.backendChoice = 1

	m, _ = updateSetupModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = updateSetupModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.step != setupStepServerURL {
		t.Fatalf("expected to stay on server URL step, got %v", m.step)
	}
	if !strings.Contains(m.errMsg, "server URL") {
		t.Fatalf("expected server URL error, got %q", m.errMsg)
	}
}

func updateSetupModel(t *testing.T, m setupModel, msg tea.Msg) (setupModel, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	next, ok := updated.(setupModel)
	if !ok {
		t.Fatalf("expected setupModel, got %T", updated)
	}
	return next, cmd
}

func runSetupCommand(t *testing.T, m *setupModel, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected setup command")
	}
	msg := cmd()
	updated, _ := updateSetupModel(t, *m, msg)
	*m = updated
	if !m.saved {
		t.Fatalf("expected model to be saved, err=%q", m.errMsg)
	}
}
