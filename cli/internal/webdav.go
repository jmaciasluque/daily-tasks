package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// ErrRemoteNotFound is returned when the remote file does not exist (HTTP 404).
var ErrRemoteNotFound = errors.New("remote file not found")

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

// PushRemoteData pushes the data to the remote WebDAV server
func PushRemoteData(settings WebDAVSettings, data Data) error {
	data.LastModified = time.Now().UnixMilli()
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
	history.UpdatedAt = time.Now().UnixMilli()
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
