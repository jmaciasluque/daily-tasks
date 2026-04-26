package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// FetchRemoteData fetches the data blob via the supplied backend and
// returns the parsed body alongside its concurrency token. Pass the token
// back to PushRemoteData as ifMatch to detect concurrent writers.
func FetchRemoteData(b Backend) (Data, string, error) {
	blob, err := b.Fetch(KeyData)
	if err != nil {
		return Data{}, "", err
	}
	var data Data
	if err := json.Unmarshal(blob.Bytes, &data); err != nil {
		return Data{}, "", err
	}
	return data, blob.Etag, nil
}

// PushRemoteData encodes and writes data via the backend. ifMatch follows
// the Backend.Push contract ("" unconditional, IfNoneMatchAny create-only,
// otherwise an etag from a prior fetch). Returns ErrEtagMismatch on 412.
//
// The caller is expected to have set data.LastModified (normally via
// SaveData). This function does not restamp it: doing so would make the
// bytes pushed differ from the bytes on disk, which would cause
// content-level conflicts with any Nextcloud desktop client syncing the
// same file.
func PushRemoteData(b Backend, data Data, ifMatch string) error {
	if data.LastModified == 0 {
		data.LastModified = time.Now().UnixMilli()
	}
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return b.Push(KeyData, body, ifMatch)
}

// FetchRemoteHistory fetches the history blob and returns the parsed,
// sorted, version-normalized payload alongside its concurrency token.
func FetchRemoteHistory(b Backend) (History, string, error) {
	blob, err := b.Fetch(KeyHistory)
	if err != nil {
		return History{}, "", err
	}
	var history History
	if err := json.Unmarshal(blob.Bytes, &history); err != nil {
		return History{}, "", err
	}
	if history.Version == 0 {
		history.Version = historyVersion
	}
	sortHistory(&history)
	return history, blob.Etag, nil
}

// PushRemoteHistory normalizes (version, updated_at, sort) and writes the
// history payload. ifMatch follows the Backend.Push contract.
// Returns ErrEtagMismatch on 412.
func PushRemoteHistory(b Backend, history History, ifMatch string) error {
	history.Version = historyVersion
	if history.UpdatedAt == 0 {
		history.UpdatedAt = time.Now().UnixMilli()
	}
	sortHistory(&history)

	body, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return b.Push(KeyHistory, body, ifMatch)
}

// PushRemoteState pushes both data and history unconditionally — used by
// the dedicated "push" command (`daily-tasks push`, the TUI push action).
// It does not check the remote etag; it deliberately overwrites whatever
// is on the server. Sync flows go through SyncWithRemote /
// SyncStateWithRemote, which fetch first and use If-Match to avoid
// clobbering concurrent writers.
func PushRemoteState(b Backend, data Data, history History) error {
	if err := PushRemoteData(b, data, ""); err != nil {
		return err
	}
	return PushRemoteHistory(b, HistoryWithCurrentSnapshot(history, data, time.Now().UnixMilli()), "")
}

// SyncRemoteHistory fetches the remote history, merges it with the
// supplied local history (and current snapshot), and pushes the merged
// result back if it differs from what the server has. Pushes are
// conditional on the ETag captured at fetch time; on ErrEtagMismatch the
// function refetches and retries once, picking up whatever a concurrent
// writer just deposited.
func SyncRemoteHistory(b Backend, local History, current Data) (History, error) {
	merged, err := mergeAndPushHistory(b, local, current)
	if err == nil || !errors.Is(err, ErrEtagMismatch) {
		return merged, err
	}
	// Etag changed mid-sync: another writer pushed between our fetch and
	// our PUT. Retry once with the latest remote state folded in.
	return mergeAndPushHistory(b, local, current)
}

func mergeAndPushHistory(b Backend, local History, current Data) (History, error) {
	remote, etag, err := FetchRemoteHistory(b)
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
			if pushErr := PushRemoteHistory(b, merged, IfNoneMatchAny); pushErr != nil {
				return History{}, pushErr
			}
		}
		return merged, nil
	}

	if !HistoryContentEqual(merged, remote) {
		if pushErr := PushRemoteHistory(b, merged, etag); pushErr != nil {
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
// Pushes are conditional on the ETag captured at fetch time, so a
// concurrent writer can be detected (412) instead of silently clobbered.
// On ErrEtagMismatch the function refetches once and re-evaluates:
// typically the remote is now newer, so the result becomes a pull rather
// than another push.
func SyncWithRemote(b Backend, local Data) SyncResult {
	result, retry := syncOnce(b, local)
	if !retry {
		return result
	}
	result, _ = syncOnce(b, local)
	return result
}

// syncOnce runs a single sync attempt. The bool return is true when the
// caller should retry (because a concurrent writer changed the etag
// between our fetch and our PUT).
func syncOnce(b Backend, local Data) (SyncResult, bool) {
	remote, etag, err := FetchRemoteData(b)
	if err != nil {
		// If remote doesn't exist, push local with If-None-Match: *
		// so we don't clobber a file another client just created.
		if errors.Is(err, ErrRemoteNotFound) {
			pushErr := PushRemoteData(b, local, IfNoneMatchAny)
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
		pushErr := PushRemoteData(b, local, etag)
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

func SyncStateWithRemote(b Backend, local Data, history History) SyncStateResult {
	result := SyncWithRemote(b, local)
	if result.Action == "error" {
		return SyncStateResult{
			Data:    result.Data,
			History: history,
			Action:  result.Action,
			Message: result.Message,
		}
	}

	mergedHistory, err := SyncRemoteHistory(b, history, NormalizeData(result.Data))
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
