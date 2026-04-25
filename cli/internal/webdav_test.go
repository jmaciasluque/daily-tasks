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

		result, _, err := FetchRemoteData(settings)
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

	t.Run("returns ETag from response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("ETag", "\"abc-123\"")
			json.NewEncoder(w).Encode(Data{NextID: 1})
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "u", Pass: "p"}
		_, etag, err := FetchRemoteData(settings)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if etag != "\"abc-123\"" {
			t.Errorf("expected etag \"abc-123\", got %q", etag)
		}
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "user", Pass: "pass"}
		_, _, err := FetchRemoteData(settings)
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
		_, _, err := FetchRemoteData(settings)
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
		_, _, err := FetchRemoteData(settings)
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

		err := PushRemoteData(settings, data, "")
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
		err := PushRemoteData(settings, Data{}, "")
		if err == nil {
			t.Error("expected error for 403")
		}
	})

	t.Run("preserves caller-supplied LastModified", func(t *testing.T) {
		var receivedData Data
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&receivedData)
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "u", Pass: "p"}
		data := Data{LastReset: "2026-01-23", NextID: 1, LastModified: 1735689600000}

		if err := PushRemoteData(settings, data, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Push must not restamp; the bytes on the wire must equal the bytes
		// the caller already wrote to disk. Otherwise a Nextcloud desktop
		// client syncing the same file sees a content mismatch and creates
		// a "conflicted copy".
		if receivedData.LastModified != 1735689600000 {
			t.Errorf("expected LastModified preserved at 1735689600000, got %d", receivedData.LastModified)
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

func TestPushRemoteDataConditional(t *testing.T) {
	t.Run("sends If-Match when ifMatch is a concrete etag", func(t *testing.T) {
		var got string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Get("If-Match")
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "u", Pass: "p"}
		if err := PushRemoteData(settings, Data{LastModified: 1}, "\"abc\""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "\"abc\"" {
			t.Errorf("expected If-Match \"abc\", got %q", got)
		}
	})

	t.Run("sends If-None-Match: * for IfNoneMatchAny", func(t *testing.T) {
		var ifMatch, ifNoneMatch string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ifMatch = r.Header.Get("If-Match")
			ifNoneMatch = r.Header.Get("If-None-Match")
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "u", Pass: "p"}
		if err := PushRemoteData(settings, Data{LastModified: 1}, IfNoneMatchAny); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ifMatch != "" {
			t.Errorf("expected no If-Match, got %q", ifMatch)
		}
		if ifNoneMatch != "*" {
			t.Errorf("expected If-None-Match \"*\", got %q", ifNoneMatch)
		}
	})

	t.Run("412 maps to ErrEtagMismatch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusPreconditionFailed)
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "u", Pass: "p"}
		err := PushRemoteData(settings, Data{LastModified: 1}, "\"stale\"")
		if !errors.Is(err, ErrEtagMismatch) {
			t.Errorf("expected ErrEtagMismatch, got %v", err)
		}
	})
}

func TestSyncWithRemote_EtagRetry(t *testing.T) {
	t.Run("412 on push triggers refetch and pulls newer remote", func(t *testing.T) {
		// Two remote states: v1 (older) and v2 (newer than local). The first
		// GET returns v1 with etag "v1". Local has a fresher timestamp than
		// v1, so SyncWithRemote tries to push with If-Match "v1". The server
		// rejects with 412 because between fetch and put a concurrent writer
		// uploaded v2. The retry GETs v2 and pulls it.
		v1 := Data{LastReset: "2026-04-25", NextID: 1, Tasks: []Task{{ID: 1, Title: "v1", Status: "todo", Order: 1}}, LastModified: 1000}
		v2 := Data{LastReset: "2026-04-25", NextID: 1, Tasks: []Task{{ID: 1, Title: "v2", Status: "todo", Order: 1}}, LastModified: 3000}

		var getCount, putCount int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				getCount++
				if getCount == 1 {
					w.Header().Set("ETag", "\"v1\"")
					json.NewEncoder(w).Encode(v1)
				} else {
					w.Header().Set("ETag", "\"v2\"")
					json.NewEncoder(w).Encode(v2)
				}
			case http.MethodPut:
				putCount++
				w.WriteHeader(http.StatusPreconditionFailed)
			}
		}))
		defer server.Close()

		settings := WebDAVSettings{URL: server.URL, User: "u", Pass: "p"}
		local := Data{LastReset: "2026-04-25", NextID: 1, Tasks: []Task{{ID: 1, Title: "local", Status: "todo", Order: 1}}, LastModified: 2000}

		result := SyncWithRemote(settings, local)
		if result.Action != "pulled" {
			t.Errorf("expected action 'pulled' after 412 retry, got '%s' (msg=%q)", result.Action, result.Message)
		}
		if result.Data.Tasks[0].Title != "v2" {
			t.Errorf("expected v2 tasks after retry, got %+v", result.Data.Tasks)
		}
		if getCount != 2 {
			t.Errorf("expected 2 GETs (initial + retry), got %d", getCount)
		}
		if putCount != 1 {
			t.Errorf("expected 1 PUT (the failed conditional one), got %d", putCount)
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

func TestLocalPathInNextcloudSyncFolder(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"empty path", "", false},
		{"inside Nextcloud folder", filepath.Join(fakeHome, "Nextcloud", ".daily-tasks.json"), true},
		{"nested inside Nextcloud", filepath.Join(fakeHome, "Nextcloud", "sub", "file.json"), true},
		{"Nextcloud root itself", filepath.Join(fakeHome, "Nextcloud"), false},
		{"sibling of Nextcloud", filepath.Join(fakeHome, "NextcloudBackup", "file.json"), false},
		{"outside home", "/etc/daily-tasks.json", false},
		{"config dir", filepath.Join(fakeHome, ".config", "daily-tasks", "data.json"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := LocalPathInNextcloudSyncFolder(c.path); got != c.want {
				t.Errorf("LocalPathInNextcloudSyncFolder(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}
