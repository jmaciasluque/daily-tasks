package internal

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestLoadWebDAVSettings(t *testing.T) {
	// Point DAILY_TASKS_CONFIG to an empty temp dir so the real user config
	// is never picked up. Tests exercise the legacy env-var fallback path.
	isolateConfig := func(t *testing.T) {
		t.Helper()
		tmpCfg := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("DAILY_TASKS_CONFIG", tmpCfg)
	}

	t.Run("legacy env vars", func(t *testing.T) {
		isolateConfig(t)
		t.Setenv("DAILY_TASKS_WEBDAV_URL", "https://example.com/dav")
		t.Setenv("DAILY_TASKS_WEBDAV_USER", "testuser")
		t.Setenv("DAILY_TASKS_WEBDAV_PASS", "testpass")

		settings, err := LoadWebDAVSettings()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings.User != "testuser" {
			t.Errorf("expected User 'testuser', got '%s'", settings.User)
		}
		if settings.Pass != "testpass" {
			t.Errorf("expected Pass 'testpass', got '%s'", settings.Pass)
		}
	})

	t.Run("missing URL", func(t *testing.T) {
		isolateConfig(t)
		t.Setenv("DAILY_TASKS_WEBDAV_USER", "testuser")
		t.Setenv("DAILY_TASKS_WEBDAV_PASS", "testpass")

		_, err := LoadWebDAVSettings()
		if err == nil {
			t.Error("expected error for missing URL")
		}
	})

	t.Run("missing user", func(t *testing.T) {
		isolateConfig(t)
		t.Setenv("DAILY_TASKS_WEBDAV_URL", "https://example.com/dav")
		t.Setenv("DAILY_TASKS_WEBDAV_PASS", "testpass")

		_, err := LoadWebDAVSettings()
		if err == nil {
			t.Error("expected error for missing user")
		}
	})

	t.Run("missing password", func(t *testing.T) {
		isolateConfig(t)
		t.Setenv("DAILY_TASKS_WEBDAV_URL", "https://example.com/dav")
		t.Setenv("DAILY_TASKS_WEBDAV_USER", "testuser")

		_, err := LoadWebDAVSettings()
		if err == nil {
			t.Error("expected error for missing password")
		}
	})
}

func TestFetchRemoteData(t *testing.T) {
	t.Run("successful fetch", func(t *testing.T) {
		data := Data{
			LastReset:    "2026-01-23",
			NextID:       5,
			Tasks:        []Task{{ID: 1, Title: "Test", Status: "todo", Order: 1}},
			ThemeIndex:   2,
			LastModified: 1234567890,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}

			user, pass, ok := r.BasicAuth()
			if !ok || user != "testuser" || pass != "testpass" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			json.NewEncoder(w).Encode(data)
		}))
		defer server.Close()

		settings := WebDAVSettings{
			URL:  server.URL,
			User: "testuser",
			Pass: "testpass",
		}

		result, err := FetchRemoteData(settings)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.NextID != 5 {
			t.Errorf("expected NextID=5, got %d", result.NextID)
		}
		if len(result.Tasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(result.Tasks))
		}
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "user", Pass: "pass"}
		_, err := FetchRemoteData(settings)
		if !errors.Is(err, ErrRemoteNotFound) {
			t.Errorf("expected ErrRemoteNotFound, got %v", err)
		}
	})

	t.Run("server error is not ErrRemoteNotFound", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "user", Pass: "pass"}
		_, err := FetchRemoteData(settings)
		if err == nil {
			t.Fatal("expected error for 500")
		}
		if errors.Is(err, ErrRemoteNotFound) {
			t.Error("500 error should not match ErrRemoteNotFound")
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "user", Pass: "pass"}
		_, err := FetchRemoteData(settings)
		if err == nil {
			t.Error("expected error for 500")
		}
		if errors.Is(err, ErrRemoteNotFound) {
			t.Error("500 error should not match ErrRemoteNotFound")
		}
	})
}

func TestPushRemoteData(t *testing.T) {
	t.Run("successful push", func(t *testing.T) {
		var receivedData Data

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT, got %s", r.Method)
			}

			user, pass, ok := r.BasicAuth()
			if !ok || user != "testuser" || pass != "testpass" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if ct := r.Header.Get("Content-Type"); ct != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", ct)
			}

			json.NewDecoder(r.Body).Decode(&receivedData)
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		settings := WebDAVSettings{
			URL:  server.URL,
			User: "testuser",
			Pass: "testpass",
		}

		data := Data{
			LastReset: "2026-01-23",
			NextID:    3,
			Tasks:     []Task{{ID: 1, Title: "Push Test", Status: "todo", Order: 1}},
		}

		err := PushRemoteData(settings, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if receivedData.NextID != 3 {
			t.Errorf("expected NextID=3, got %d", receivedData.NextID)
		}
		if receivedData.LastModified == 0 {
			t.Error("expected LastModified to be set")
		}
	})

	t.Run("push failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "user", Pass: "pass"}
		err := PushRemoteData(settings, Data{})
		if err == nil {
			t.Error("expected error for 403")
		}
	})
}

func TestSyncWithRemote(t *testing.T) {
	t.Run("remote newer pulls", func(t *testing.T) {
		remoteData := Data{
			LastReset:    "2026-01-23",
			NextID:       10,
			Tasks:        []Task{{ID: 1, Title: "Remote", Status: "todo", Order: 1}},
			LastModified: 2000000000,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(remoteData)
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "user", Pass: "pass"}
		localData := Data{
			LastReset:    "2026-01-23",
			NextID:       5,
			Tasks:        []Task{{ID: 1, Title: "Local", Status: "todo", Order: 1}},
			LastModified: 1000000000,
		}

		result := SyncWithRemote(settings, localData)
		if result.Action != "pulled" {
			t.Errorf("expected action 'pulled', got '%s'", result.Action)
		}
		if result.Data.NextID != 10 {
			t.Errorf("expected NextID=10 from remote, got %d", result.Data.NextID)
		}
	})

	t.Run("local newer pushes", func(t *testing.T) {
		remoteData := Data{
			LastModified: 1000000000,
		}
		pushed := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				json.NewEncoder(w).Encode(remoteData)
			} else if r.Method == http.MethodPut {
				pushed = true
				w.WriteHeader(http.StatusCreated)
			}
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "user", Pass: "pass"}
		localData := Data{
			NextID:       15,
			LastModified: 2000000000,
		}

		result := SyncWithRemote(settings, localData)
		if result.Action != "pushed" {
			t.Errorf("expected action 'pushed', got '%s'", result.Action)
		}
		if !pushed {
			t.Error("expected push to be called")
		}
	})

	t.Run("remote not found creates", func(t *testing.T) {
		pushed := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusNotFound)
			} else if r.Method == http.MethodPut {
				pushed = true
				w.WriteHeader(http.StatusCreated)
			}
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "user", Pass: "pass"}
		localData := Data{NextID: 5}

		result := SyncWithRemote(settings, localData)
		if result.Action != "pushed" {
			t.Errorf("expected action 'pushed', got '%s'", result.Action)
		}
		if !pushed {
			t.Error("expected push to be called")
		}
	})
}

func TestHasWebDAVConfig(t *testing.T) {
	t.Run("configured", func(t *testing.T) {
		t.Setenv("DAILY_TASKS_CONFIG", filepath.Join(t.TempDir(), "config.json"))
		t.Setenv("DAILY_TASKS_WEBDAV_URL", "https://example.com")
		t.Setenv("DAILY_TASKS_WEBDAV_USER", "user")
		t.Setenv("DAILY_TASKS_WEBDAV_PASS", "pass")

		if !HasWebDAVConfig() {
			t.Error("expected HasWebDAVConfig to return true")
		}
	})

	t.Run("not configured", func(t *testing.T) {
		t.Setenv("DAILY_TASKS_CONFIG", filepath.Join(t.TempDir(), "config.json"))

		if HasWebDAVConfig() {
			t.Error("expected HasWebDAVConfig to return false")
		}
	})
}
