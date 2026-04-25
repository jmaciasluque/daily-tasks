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

// ErrEtagMismatch is returned when a conditional PUT fails because the remote
// resource changed under us (HTTP 412 Precondition Failed). Callers can react
// by re-fetching and merging or by surfacing the conflict to the user.
var ErrEtagMismatch = errors.New("remote etag changed since last fetch")

// IfNoneMatchAny is the value used as the ifMatch argument when the caller
// wants the PUT to succeed only if the resource does not yet exist (i.e.
// translated by the WebDAV layer to an If-None-Match: * header).
const IfNoneMatchAny = "*"

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

// FetchRemoteData fetches the data from the remote WebDAV server and returns
// the response ETag alongside the parsed body. The etag is opaque and should
// be passed back verbatim to PushRemoteData as ifMatch on a subsequent PUT
// to detect concurrent writers (412 → ErrEtagMismatch).
func FetchRemoteData(settings WebDAVSettings) (Data, string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, settings.URL, nil)
	if err != nil {
		return Data{}, "", err
	}
	req.SetBasicAuth(settings.User, settings.Pass)
	resp, err := client.Do(req)
	if err != nil {
		return Data{}, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Data{}, "", ErrRemoteNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Data{}, "", fmt.Errorf("remote status %d", resp.StatusCode)
	}
	var data Data
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return Data{}, "", err
	}
	return data, resp.Header.Get("ETag"), nil
}

// PushRemoteData pushes data to the remote WebDAV server. ifMatch carries the
// ETag from a prior FetchRemoteData call: pass "" to push unconditionally,
// IfNoneMatchAny to require the resource not yet exist (initial create), or
// any other value to require the resource still match that ETag. On a 412
// Precondition Failed response this returns ErrEtagMismatch so callers can
// refetch and merge instead of silently clobbering a concurrent writer.
//
// The caller is expected to have set data.LastModified (normally via
// SaveData). This function does not restamp it: doing so would make the
// bytes pushed differ from the bytes on disk, which would itself cause
// content-level conflicts with any Nextcloud desktop client syncing the
// same file.
func PushRemoteData(settings WebDAVSettings, data Data, ifMatch string) error {
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
	setIfMatch(req, ifMatch)
	req.SetBasicAuth(settings.User, settings.Pass)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPreconditionFailed {
		return ErrEtagMismatch
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("push failed with status %d", resp.StatusCode)
	}
	return nil
}

// setIfMatch translates an ifMatch argument into the corresponding precondition
// header. The IfNoneMatchAny sentinel maps to If-None-Match: * (only allow
// create); any other non-empty value goes onto If-Match verbatim.
func setIfMatch(req *http.Request, ifMatch string) {
	switch ifMatch {
	case "":
		// no precondition
	case IfNoneMatchAny:
		req.Header.Set("If-None-Match", "*")
	default:
		req.Header.Set("If-Match", ifMatch)
	}
}

// FetchRemoteHistory returns the parsed history payload along with the ETag
// for the resource (empty if the server did not provide one). Pass the etag
// back to PushRemoteHistory as ifMatch on a subsequent PUT to detect
// concurrent writers.
func FetchRemoteHistory(settings WebDAVSettings) (History, string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, buildHistoryWebDAVURL(settings.URL), nil)
	if err != nil {
		return History{}, "", err
	}
	req.SetBasicAuth(settings.User, settings.Pass)
	resp, err := client.Do(req)
	if err != nil {
		return History{}, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return History{}, "", ErrRemoteNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return History{}, "", fmt.Errorf("remote history status %d", resp.StatusCode)
	}
	var history History
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return History{}, "", err
	}
	if history.Version == 0 {
		history.Version = historyVersion
	}
	sortHistory(&history)
	return history, resp.Header.Get("ETag"), nil
}

// PushRemoteHistory pushes the history payload. The ifMatch argument follows
// the same conventions as PushRemoteData: "" for unconditional, IfNoneMatchAny
// for create-only, otherwise an opaque ETag from a prior fetch.
// Returns ErrEtagMismatch on 412.
func PushRemoteHistory(settings WebDAVSettings, history History, ifMatch string) error {
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
	setIfMatch(req, ifMatch)
	req.SetBasicAuth(settings.User, settings.Pass)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPreconditionFailed {
		return ErrEtagMismatch
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("history push failed with status %d", resp.StatusCode)
	}
	return nil
}

// PushRemoteState pushes both data and history unconditionally — used by the
// dedicated "push" command (`daily-tasks push`, the TUI push action). It does
// not check the remote etag; it deliberately overwrites whatever is on the
// server. Sync flows go through SyncWithRemote / SyncStateWithRemote, which
// fetch first and use If-Match to avoid clobbering concurrent writers.
func PushRemoteState(settings WebDAVSettings, data Data, history History) error {
	if err := PushRemoteData(settings, data, ""); err != nil {
		return err
	}
	return PushRemoteHistory(settings, HistoryWithCurrentSnapshot(history, data, time.Now().UnixMilli()), "")
}

// SyncRemoteHistory fetches the remote history, merges it with the supplied
// local history (and current snapshot), and pushes the merged result back if
// it differs from what the server has. Pushes are conditional on the ETag
// captured at fetch time; on ErrEtagMismatch the function refetches and
// retries once, picking up whatever a concurrent writer just deposited.
func SyncRemoteHistory(settings WebDAVSettings, local History, current Data) (History, error) {
	merged, err := mergeAndPushHistory(settings, local, current)
	if err == nil || !errors.Is(err, ErrEtagMismatch) {
		return merged, err
	}
	// Etag changed mid-sync: another writer pushed between our fetch and
	// our PUT. Retry once with the latest remote state folded in.
	return mergeAndPushHistory(settings, local, current)
}

func mergeAndPushHistory(settings WebDAVSettings, local History, current Data) (History, error) {
	remote, etag, err := FetchRemoteHistory(settings)
	notFound := errors.Is(err, ErrRemoteNotFound)
	if err != nil && !notFound {
		return History{}, err
	}
	if notFound {
		remote = History{Version: historyVersion}
	}

	merged := HistoryWithCurrentSnapshot(MergeHistories(local, remote), current, time.Now().UnixMilli())

	if notFound {
		if len(merged.Days) > 0 || len(merged.Events) > 0 {
			if pushErr := PushRemoteHistory(settings, merged, IfNoneMatchAny); pushErr != nil {
				return History{}, pushErr
			}
		}
		return merged, nil
	}

	if !HistoryContentEqual(merged, remote) {
		if pushErr := PushRemoteHistory(settings, merged, etag); pushErr != nil {
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

// SyncWithRemote performs a bi-directional sync with conflict detection.
// Pushes are conditional on the ETag captured at fetch time, so a concurrent
// writer can be detected (412) instead of silently clobbered. On
// ErrEtagMismatch the function refetches once and re-evaluates: typically the
// remote is now newer, so the result becomes a pull rather than another push.
func SyncWithRemote(settings WebDAVSettings, local Data) SyncResult {
	result, retry := syncOnce(settings, local)
	if !retry {
		return result
	}
	result, _ = syncOnce(settings, local)
	return result
}

// syncOnce runs a single sync attempt. The bool return is true when the
// caller should retry (because a concurrent writer changed the etag between
// our fetch and our PUT).
func syncOnce(settings WebDAVSettings, local Data) (SyncResult, bool) {
	remote, etag, err := FetchRemoteData(settings)
	if err != nil {
		// If remote doesn't exist, push local with If-None-Match: *
		// so we don't clobber a file another client just created.
		if errors.Is(err, ErrRemoteNotFound) {
			pushErr := PushRemoteData(settings, local, IfNoneMatchAny)
			if errors.Is(pushErr, ErrEtagMismatch) {
				// Lost the create race — retry path will pull or merge.
				return SyncResult{Data: local, Action: "error", Message: "Remote was created by another writer; retrying"}, true
			}
			if pushErr != nil {
				return SyncResult{Data: local, Action: "error", Message: fmt.Sprintf("Push failed: %s", pushErr)}, false
			}
			return SyncResult{Data: local, Action: "pushed", Message: "Created remote file"}, false
		}
		return SyncResult{Data: local, Action: "error", Message: err.Error()}, false
	}

	remote = NormalizeData(remote)
	local = NormalizeData(local)

	// Never overwrite remote tasks with an empty local state (e.g. fresh install)
	if len(local.Tasks) == 0 && len(remote.Tasks) > 0 {
		return SyncResult{Data: remote, Action: "pulled", Message: "Pulled remote data"}, false
	}

	// Check for conflicts using LastModified
	if remote.LastModified > local.LastModified {
		// Remote is newer, pull it
		return SyncResult{Data: remote, Action: "pulled", Message: "Pulled newer remote data"}, false
	} else if local.LastModified > remote.LastModified {
		// Local is newer, push it conditionally on the etag we just saw.
		pushErr := PushRemoteData(settings, local, etag)
		if errors.Is(pushErr, ErrEtagMismatch) {
			return SyncResult{Data: local, Action: "error", Message: "Remote changed during sync; retrying"}, true
		}
		if pushErr != nil {
			return SyncResult{Data: local, Action: "error", Message: fmt.Sprintf("Push failed: %s", pushErr)}, false
		}
		return SyncResult{Data: local, Action: "pushed", Message: "Pushed local changes"}, false
	}

	// Same timestamp, no action needed
	return SyncResult{Data: local, Action: "in_sync", Message: "Already in sync"}, false
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
