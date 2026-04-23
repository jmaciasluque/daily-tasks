package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// ErrRemoteNotFound is returned when the remote file does not exist (HTTP 404).
var ErrRemoteNotFound = errors.New("remote file not found")

// ErrWebDAVHandledByDesktopClient is returned when the local data file lives
// inside a Nextcloud desktop-client sync folder, so the CLI should not also
// push to WebDAV — doing both would race and create "conflicted copy" files.
var ErrWebDAVHandledByDesktopClient = errors.New("local data file is inside a Nextcloud sync folder; desktop client handles sync")

// LocalPathInNextcloudSyncFolder reports whether dataPath resolves to a
// location inside the user's Nextcloud desktop-client sync folder (~/Nextcloud).
// When true, the desktop client already propagates writes to the server, and
// the CLI should not perform its own WebDAV PUTs against the same file.
func LocalPathInNextcloudSyncFolder(dataPath string) bool {
	if dataPath == "" {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	abs, err := filepath.Abs(dataPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(filepath.Join(home, "Nextcloud"), abs)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, "..")
}

// WebDAVSettings contains the configuration for WebDAV sync
type WebDAVSettings struct {
	URL  string
	User string
	Pass string
}

func buildWebDAVURL(cfg NextcloudConfig) string {
	base := strings.TrimRight(cfg.ServerURL, "/")
	path := cfg.RemotePath
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func buildHistoryWebDAVURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw + ".history.json"
	}
	ext := path.Ext(parsed.Path)
	if ext == "" {
		parsed.Path += ".history.json"
	} else {
		base := strings.TrimSuffix(parsed.Path, ext)
		parsed.Path = base + ".history" + ext
	}
	return parsed.String()
}

// LoadWebDAVSettings loads WebDAV configuration from the persisted backend config.
func LoadWebDAVSettings() (WebDAVSettings, error) {
	cfg, _, err := LoadEffectiveAppConfig()
	if err != nil {
		return WebDAVSettings{}, err
	}
	if cfg.Backend != BackendNextcloud || !nextcloudConfigComplete(cfg.Nextcloud) {
		return WebDAVSettings{}, ErrNextcloudNotConfigured
	}
	return WebDAVSettings{
		URL:  buildWebDAVURL(cfg.Nextcloud),
		User: cfg.Nextcloud.LoginName,
		Pass: cfg.Nextcloud.AppPassword,
	}, nil
}

// FetchRemoteData fetches the data from the remote WebDAV server
func FetchRemoteData(settings WebDAVSettings) (Data, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, settings.URL, nil)
	if err != nil {
		return Data{}, err
	}
	req.SetBasicAuth(settings.User, settings.Pass)
	resp, err := client.Do(req)
	if err != nil {
		return Data{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Data{}, ErrRemoteNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Data{}, fmt.Errorf("remote status %d", resp.StatusCode)
	}
	var data Data
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return Data{}, err
	}
	return data, nil
}

// PushRemoteData pushes the data to the remote WebDAV server.
// The caller is expected to have set data.LastModified (normally via SaveData).
// This function no longer restamps it: if it did, the bytes pushed via WebDAV
// would differ from the bytes on disk, guaranteeing a conflict whenever a
// Nextcloud desktop client is also syncing the same file.
func PushRemoteData(settings WebDAVSettings, data Data) error {
	if data.LastModified == 0 {
		data.LastModified = time.Now().UnixMilli()
	}
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodPut, settings.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(settings.User, settings.Pass)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("push failed with status %d", resp.StatusCode)
	}
	return nil
}

func FetchRemoteHistory(settings WebDAVSettings) (History, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, buildHistoryWebDAVURL(settings.URL), nil)
	if err != nil {
		return History{}, err
	}
	req.SetBasicAuth(settings.User, settings.Pass)
	resp, err := client.Do(req)
	if err != nil {
		return History{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return History{}, ErrRemoteNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return History{}, fmt.Errorf("remote history status %d", resp.StatusCode)
	}
	var history History
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return History{}, err
	}
	if history.Version == 0 {
		history.Version = historyVersion
	}
	sortHistory(&history)
	return history, nil
}

func PushRemoteHistory(settings WebDAVSettings, history History) error {
	history.Version = historyVersion
	if history.UpdatedAt == 0 {
		history.UpdatedAt = time.Now().UnixMilli()
	}
	sortHistory(&history)

	body, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodPut, buildHistoryWebDAVURL(settings.URL), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(settings.User, settings.Pass)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("history push failed with status %d", resp.StatusCode)
	}
	return nil
}

func PushRemoteState(settings WebDAVSettings, data Data, history History) error {
	if err := PushRemoteData(settings, data); err != nil {
		return err
	}
	return PushRemoteHistory(settings, HistoryWithCurrentSnapshot(history, data, time.Now().UnixMilli()))
}

func SyncRemoteHistory(settings WebDAVSettings, local History, current Data) (History, error) {
	remote, err := FetchRemoteHistory(settings)
	if err != nil && !errors.Is(err, ErrRemoteNotFound) {
		return History{}, err
	}
	if errors.Is(err, ErrRemoteNotFound) {
		remote = History{Version: historyVersion}
	}

	merged := HistoryWithCurrentSnapshot(MergeHistories(local, remote), current, time.Now().UnixMilli())

	if errors.Is(err, ErrRemoteNotFound) {
		if len(merged.Days) > 0 || len(merged.Events) > 0 {
			if pushErr := PushRemoteHistory(settings, merged); pushErr != nil {
				return History{}, pushErr
			}
		}
		return merged, nil
	}

	if !HistoryContentEqual(merged, remote) {
		if pushErr := PushRemoteHistory(settings, merged); pushErr != nil {
			return History{}, pushErr
		}
	}
	return merged, nil
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Data     Data
	Action   string // "pulled", "pushed", "in_sync", "error"
	Message  string
	Conflict bool
}

type SyncStateResult struct {
	Data    Data
	History History
	Action  string
	Message string
}

// SyncWithRemote performs a bi-directional sync with conflict detection
func SyncWithRemote(settings WebDAVSettings, local Data) SyncResult {
	remote, err := FetchRemoteData(settings)
	if err != nil {
		// If remote doesn't exist, push local
		if errors.Is(err, ErrRemoteNotFound) {
			if pushErr := PushRemoteData(settings, local); pushErr != nil {
				return SyncResult{Data: local, Action: "error", Message: fmt.Sprintf("Push failed: %s", pushErr)}
			}
			return SyncResult{Data: local, Action: "pushed", Message: "Created remote file"}
		}
		return SyncResult{Data: local, Action: "error", Message: err.Error()}
	}

	remote = NormalizeData(remote)
	local = NormalizeData(local)

	// Never overwrite remote tasks with an empty local state (e.g. fresh install)
	if len(local.Tasks) == 0 && len(remote.Tasks) > 0 {
		return SyncResult{Data: remote, Action: "pulled", Message: "Pulled remote data"}
	}

	// Check for conflicts using LastModified
	if remote.LastModified > local.LastModified {
		// Remote is newer, pull it
		return SyncResult{Data: remote, Action: "pulled", Message: "Pulled newer remote data"}
	} else if local.LastModified > remote.LastModified {
		// Local is newer, push it
		if pushErr := PushRemoteData(settings, local); pushErr != nil {
			return SyncResult{Data: local, Action: "error", Message: fmt.Sprintf("Push failed: %s", pushErr)}
		}
		return SyncResult{Data: local, Action: "pushed", Message: "Pushed local changes"}
	}

	// Same timestamp, no action needed
	return SyncResult{Data: local, Action: "in_sync", Message: "Already in sync"}
}

func SyncStateWithRemote(settings WebDAVSettings, local Data, history History) SyncStateResult {
	result := SyncWithRemote(settings, local)
	if result.Action == "error" {
		return SyncStateResult{
			Data:    result.Data,
			History: history,
			Action:  result.Action,
			Message: result.Message,
		}
	}

	mergedHistory, err := SyncRemoteHistory(settings, history, NormalizeData(result.Data))
	if err != nil {
		return SyncStateResult{
			Data:    result.Data,
			History: history,
			Action:  "error",
			Message: err.Error(),
		}
	}

	return SyncStateResult{
		Data:    NormalizeData(result.Data),
		History: mergedHistory,
		Action:  result.Action,
		Message: result.Message,
	}
}

// HasWebDAVConfig returns true if WebDAV is configured
func HasWebDAVConfig() bool {
	_, err := LoadWebDAVSettings()
	return err == nil && !errors.Is(err, ErrNextcloudNotConfigured)
}
