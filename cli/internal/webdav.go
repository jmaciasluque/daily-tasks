package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// WebDAVSettings contains the configuration for WebDAV sync
type WebDAVSettings struct {
	URL  string
	User string
	Pass string
}

// LoadWebDAVSettings loads WebDAV configuration from environment variables
func LoadWebDAVSettings() (WebDAVSettings, error) {
	url := strings.TrimSpace(os.Getenv("DAILY_TASKS_WEBDAV_URL"))
	user := os.Getenv("DAILY_TASKS_WEBDAV_USER")
	pass := os.Getenv("DAILY_TASKS_WEBDAV_PASS")
	if url == "" || user == "" || pass == "" {
		return WebDAVSettings{}, errors.New("set DAILY_TASKS_WEBDAV_URL, DAILY_TASKS_WEBDAV_USER, DAILY_TASKS_WEBDAV_PASS")
	}
	return WebDAVSettings{URL: url, User: user, Pass: pass}, nil
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
		return Data{}, errors.New("remote file not found")
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
	data.LastModified = time.Now().Unix()
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

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Data     Data
	Action   string // "pulled", "pushed", "conflict", "error"
	Message  string
	Conflict bool
}

// SyncWithRemote performs a bi-directional sync with conflict detection
func SyncWithRemote(settings WebDAVSettings, local Data) SyncResult {
	remote, err := FetchRemoteData(settings)
	if err != nil {
		// If remote doesn't exist, push local
		if strings.Contains(err.Error(), "not found") {
			if pushErr := PushRemoteData(settings, local); pushErr != nil {
				return SyncResult{Data: local, Action: "error", Message: fmt.Sprintf("Push failed: %s", pushErr)}
			}
			return SyncResult{Data: local, Action: "pushed", Message: "Created remote file"}
		}
		return SyncResult{Data: local, Action: "error", Message: err.Error()}
	}

	remote = NormalizeData(remote)

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
	return SyncResult{Data: local, Action: "pulled", Message: "Already in sync"}
}

// HasWebDAVConfig returns true if WebDAV is configured
func HasWebDAVConfig() bool {
	_, err := LoadWebDAVSettings()
	return err == nil
}
